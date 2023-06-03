package internal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	fsutil "github.com/go-git/go-billy/v5/util"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/utils"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/compl"
	help_ns "github.com/inoxlang/inox/internal/globals/help_ns"
	parse "github.com/inoxlang/inox/internal/parse"

	_ "net/http/pprof"
	"net/url"
)

var (
	ErrFileURIExpected     = errors.New("a file: URI was expected")
	ErrRemoteFsURIExpected = errors.New("a remotefs: URI was expected")
)

func registerHandlers(server *lsp.Server, remoteFs bool) {

	var shuttingDownSessionsLock sync.Mutex
	shuttingDownSessions := make(map[*jsonrpc.Session]struct{})

	server.OnInitialize(func(ctx context.Context, req *defines.InitializeParams) (result *defines.InitializeResult, err *defines.InitializeError) {
		logs.Println("initialized")
		s := &defines.InitializeResult{}

		s.Capabilities.HoverProvider = true
		s.Capabilities.WorkspaceSymbolProvider = true
		s.Capabilities.DefinitionProvider = true

		// makes the client send the whole document during synchronization
		s.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindFull

		s.Capabilities.CompletionProvider = &defines.CompletionOptions{}
		return s, nil
	})

	server.OnShutdown(func(ctx context.Context, req *defines.NoParams) (err error) {
		session := jsonrpc.GetSession(ctx)

		shuttingDownSessionsLock.Lock()
		defer shuttingDownSessionsLock.Unlock()

		shuttingDownSessions[session] = struct{}{}
		return nil
	})

	server.OnExit(func(ctx context.Context, req *defines.NoParams) (err error) {
		session := jsonrpc.GetSession(ctx)

		shuttingDownSessionsLock.Lock()
		defer shuttingDownSessionsLock.Unlock()

		if _, ok := shuttingDownSessions[session]; ok {
			session.Close()
		} else {
			return errors.New("the client should make shutdown request before sending an exit notification")
		}

		return nil
	})

	server.OnHover(func(ctx context.Context, req *defines.HoverParams) (result *defines.Hover, err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, remoteFs)
		if err != nil {
			return nil, err
		}
		line, column := getLineColumn(req.Position)
		session := jsonrpc.GetSession(ctx)
		sessionCtx := session.Context()

		state, mod, _, _ := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: sessionCtx,
			ParentContext:             nil,
			Out:                       os.Stdout,
			IgnoreNonCriticalIssues:   true,
			AllowMissingEnvVars:       true,
			PreinitFilesystem:         sessionCtx.GetFileSystem(),
			ScriptContextFileSystem:   sessionCtx.GetFileSystem(),
		})

		if state == nil || state.SymbolicData == nil {
			logs.Println("no data")
			return &defines.Hover{}, nil
		}

		span := mod.MainChunk.GetLineColumnSingeCharSpan(line, column)
		foundNode, ok := mod.MainChunk.GetNodeAtSpan(span)

		if !ok || foundNode == nil {
			logs.Println("no data")
			return &defines.Hover{}, nil
		}

		mostSpecificVal, ok := state.SymbolicData.GetMostSpecificNodeValue(foundNode)
		var lessSpecificVal symbolic.SymbolicValue
		if !ok {
			logs.Println("no data")
			return &defines.Hover{}, nil
		}

		buff := &bytes.Buffer{}
		w := bufio.NewWriterSize(buff, 1000)
		var stringified string
		{
			utils.PanicIfErr(symbolic.PrettyPrint(mostSpecificVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))
			var ok bool
			lessSpecificVal, ok = state.SymbolicData.GetLessSpecificNodeValue(foundNode)
			if ok {
				w.Write(utils.StringAsBytes("\n\n# less specific\n"))
				utils.PanicIfErr(symbolic.PrettyPrint(lessSpecificVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))
			}

			w.Flush()
			stringified = strings.ReplaceAll(buff.String(), "\n\r", "\n")
			logs.Println(stringified)
		}

		//help
		var helpMessage string
		{
			val := mostSpecificVal
			for {
				switch val := val.(type) {
				case *symbolic.GoFunction:
					text, ok := help_ns.HelpForSymbolicGoFunc(val, help_ns.HelpMessageConfig{Format: help_ns.MarkdownFormat})
					if ok {
						helpMessage = "\n-----\n" + strings.ReplaceAll(text, "\n\r", "\n")
					}
				}
				if helpMessage == "" && val == mostSpecificVal && lessSpecificVal != nil {
					val = lessSpecificVal
					continue
				}
				break
			}

		}

		return &defines.Hover{
			Contents: defines.MarkupContent{
				Kind:  defines.MarkupKindMarkdown,
				Value: "```inox\n" + stringified + "\n```" + helpMessage,
			},
		}, nil
	})

	server.OnCompletion(func(ctx context.Context, req *defines.CompletionParams) (result *[]defines.CompletionItem, err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, remoteFs)
		if err != nil {
			return nil, err
		}

		line, column := getLineColumn(req.Position)
		session := jsonrpc.GetSession(ctx)

		completions := getCompletions(fpath, session.Context(), line, column, session)
		completionIndex := 0

		lspCompletions := utils.MapSlice(completions, func(completion compl.Completion) defines.CompletionItem {
			defer func() {
				completionIndex++
			}()

			var labelDetails *defines.CompletionItemLabelDetails
			if completion.Detail != "" {
				detail := "  " + completion.Detail
				labelDetails = &defines.CompletionItemLabelDetails{
					Detail: &detail,
				}
			}

			return defines.CompletionItem{
				Label: completion.Value,
				Kind:  &completion.Kind,
				TextEdit: defines.TextEdit{
					Range: rangeToLspRange(completion.ReplacedRange),
				},
				SortText: func() *string {
					index := completionIndex
					if index > 99 {
						index = 99
					}
					s := string(rune(index/10) + 'a')
					s += string(rune(index%10) + 'a')
					return &s
				}(),
				LabelDetails: labelDetails,
			}
		})
		return &lspCompletions, nil
	})

	server.OnDidOpenTextDocument(func(ctx context.Context, req *defines.DidOpenTextDocumentParams) (err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, remoteFs)
		if err != nil {
			return err
		}

		fullDocumentText := req.TextDocument.Text
		session := jsonrpc.GetSession(ctx)
		fls := session.Context().GetFileSystem().(*Filesystem)

		fsErr := fsutil.WriteFile(fls.docsFS(), fpath, []byte(fullDocumentText), 0700)
		if fsErr != nil {
			logs.Println("failed to update state of document", fpath+":", fsErr)
		}

		return notifyDiagnostics(session, req.TextDocument.Uri, remoteFs)
	})

	server.OnDidChangeTextDocument(func(ctx context.Context, req *defines.DidChangeTextDocumentParams) (err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, remoteFs)
		if err != nil {
			return err
		}

		if len(req.ContentChanges) > 1 {
			return errors.New("single change supported")
		}
		session := jsonrpc.GetSession(ctx)
		fls := session.Context().GetFileSystem().(*Filesystem)

		fullDocumentText := req.ContentChanges[0].Text.(string)
		fsErr := fsutil.WriteFile(fls.docsFS(), fpath, []byte(fullDocumentText), 0700)
		if fsErr != nil {
			logs.Println("failed to update state of document", fpath+":", fsErr)
		}

		return notifyDiagnostics(session, req.TextDocument.Uri, remoteFs)
	})

	server.OnDefinition(func(ctx context.Context, req *defines.DefinitionParams) (result *[]defines.LocationLink, err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, remoteFs)
		if err != nil {
			return nil, err
		}
		line, column := getLineColumn(req.Position)
		session := jsonrpc.GetSession(ctx)
		sessionCtx := session.Context()

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: sessionCtx,
			ParentContext:             nil,
			Out:                       os.Stdout,
			IgnoreNonCriticalIssues:   true,
			AllowMissingEnvVars:       true,
			ScriptContextFileSystem:   sessionCtx.GetFileSystem(),
			PreinitFilesystem:         sessionCtx.GetFileSystem(),
		})

		if state == nil || state.SymbolicData == nil {
			logs.Println("failed to prepare script", err)
			return nil, nil
		}

		//TODO: support definition when included chunk is being edited
		chunk := mod.MainChunk

		span := chunk.GetLineColumnSingeCharSpan(line, column)
		foundNode, ancestors, ok := chunk.GetNodeAndChainAtSpan(span)

		if !ok || foundNode == nil {
			logs.Println("no data: node not found")
			return nil, nil
		}

		var position parse.SourcePositionRange

		switch n := foundNode.(type) {
		case *parse.Variable, *parse.GlobalVariable, *parse.IdentifierLiteral:
			position, ok = state.SymbolicData.GetVariableDefinitionPosition(foundNode, ancestors)

		case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceIdentifierLiteral:
			position, ok = state.SymbolicData.GetNamedPatternOrPatternNamespacePositionDefinition(foundNode, ancestors)
		case *parse.RelativePathLiteral:
			parent := ancestors[len(ancestors)-1]
			switch parent.(type) {
			case *parse.InclusionImportStatement:
				file, isFile := chunk.Source.(parse.SourceFile)
				if !isFile || file.IsResourceURL || file.ResourceDir == "" {
					break
				}

				path := filepath.Join(file.ResourceDir, n.Value)
				position = parse.SourcePositionRange{
					SourceName:  path,
					StartLine:   1,
					StartColumn: 1,
					Span:        parse.NodeSpan{Start: 0, End: 1},
				}
				ok = true
			}
		}

		if !ok {
			logs.Println("no data")
			return nil, nil
		}

		links := []defines.LocationLink{
			{
				TargetUri:            defines.DocumentUri("file://" + position.SourceName),
				TargetRange:          rangeToLspRange(position),
				TargetSelectionRange: rangeToLspRange(position),
			},
		}
		return &links, nil
	})

	if remoteFs {
		server.OnCustom(jsonrpc.MethodInfo{
			Name: "fs/fileStat",
			NewRequest: func() interface{} {
				return &FsFileStatParams{}
			},
			Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				session := jsonrpc.GetSession(ctx)
				fls := session.Context().GetFileSystem()
				params := req.(*FsFileStatParams)

				fpath, err := getPath(params.FileURI, remoteFs)
				if err != nil {
					return nil, err
				}

				stat, err := fls.Stat(fpath)
				if err != nil {
					if os.IsNotExist(err) {
						return FsFileNotFound, nil
					}
					return nil, fmt.Errorf("failed to get stat for file %s: %w", fpath, err)
				}

				ctime, mtime, err := fs_ns.GetCreationAndModifTime(stat)
				if err != nil {
					return nil, fmt.Errorf("failed to get the creation/modification time for file %s", fpath)
				}

				return &FsFileStat{
					CreationTime:     ctime.UnixMilli(),
					ModificationTime: mtime.UnixMilli(),
					Size:             stat.Size(),
					FileType:         FileTypeFromInfo(stat),
				}, nil
			},
		})

		server.OnCustom(jsonrpc.MethodInfo{
			Name: "fs/readFile",
			NewRequest: func() interface{} {
				return &FsReadFileParams{}
			},
			Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				session := jsonrpc.GetSession(ctx)
				fls := session.Context().GetFileSystem()
				params := req.(*FsReadFileParams)

				fpath, err := getPath(params.FileURI, remoteFs)
				if err != nil {
					if os.IsNotExist(err) {
						return FsFileNotFound, nil
					}
					return nil, err
				}

				content, err := fsutil.ReadFile(fls, fpath)
				if err != nil {
					return nil, fmt.Errorf("failed to read file %s: %w", fpath, err)
				}

				return FsFileContentBase64{Content: base64.StdEncoding.EncodeToString(content)}, nil
			},
		})

		server.OnCustom(jsonrpc.MethodInfo{
			Name: "fs/writeFile",
			NewRequest: func() interface{} {
				return &FsWriteFileParams{}
			},
			Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				session := jsonrpc.GetSession(ctx)
				fls := session.Context().GetFileSystem()
				params := req.(*FsWriteFileParams)

				fpath, err := getPath(params.FileURI, remoteFs)
				if err != nil {
					return nil, err
				}

				content, err := base64.StdEncoding.DecodeString(string(params.ContentBase64))
				if err != nil {
					return nil, fmt.Errorf("failed to decode received content for file %s: %w", fpath, err)
				}

				if params.Create {
					f, err := fls.OpenFile(fpath, os.O_CREATE|os.O_WRONLY, fs_ns.DEFAULT_FILE_FMODE)

					defer func() {
						if f != nil {
							f.Close()
						}
					}()

					if err != nil && !os.IsNotExist(err) {
						return nil, fmt.Errorf("failed to create file %s: %w", fpath, err)
					}

					alreadyExists := err == nil

					if alreadyExists {
						if !params.Overwrite {
							return nil, fmt.Errorf("failed to create file %s: already exists and overwrite option is false", fpath)
						}

						if err := f.Truncate(int64(len(content))); err != nil {
							return nil, fmt.Errorf("failed to truncate file before write %s: %w", fpath, err)
						}
					}

					_, err = f.Write(content)

					if err != nil {
						return nil, fmt.Errorf("failed to create file %s: failed to write: %w", fpath, err)
					}
				} else {
					f, err := fls.OpenFile(fpath, os.O_WRONLY, 0)

					defer func() {
						if f != nil {
							f.Close()
						}
					}()

					if os.IsNotExist(err) {
						return FsFileNotFound, nil
					} else if err != nil {
						return nil, fmt.Errorf("failed to write file %s: failed to open: %w", fpath, err)
					}

					if err := f.Truncate(int64(len(content))); err != nil {
						return nil, fmt.Errorf("failed to truncate file before write: %s: %w", fpath, err)
					}

					_, err = f.Write(content)

					if err != nil {
						return nil, fmt.Errorf("failed to create file %s: failed to write: %w", fpath, err)
					}
				}

				return nil, nil
			},
		})

		server.OnCustom(jsonrpc.MethodInfo{
			Name: "fs/renameFile",
			NewRequest: func() interface{} {
				return &FsRenameFileParams{}
			},
			Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				session := jsonrpc.GetSession(ctx)
				fls := session.Context().GetFileSystem()
				params := req.(*FsRenameFileParams)

				path, err := getPath(params.FileURI, remoteFs)
				if err != nil {
					return nil, err
				}

				newPath, err := getPath(params.NewFileURI, remoteFs)
				if err != nil {
					return nil, err
				}

				_, err = fls.Stat(path)
				if os.IsNotExist(err) {
					return FsFileNotFound, nil
				}

				newPathStat, err := fls.Stat(newPath)

				if os.IsNotExist(err) {
					//there is no file at the desination path so we can rename it.
					return nil, fls.Rename(path, newPath)
				} else { //exists
					if params.Overwrite {
						if err == nil && newPathStat.IsDir() {
							if err := fls.Remove(newPath); err != nil {
								return nil, fmt.Errorf("failed to rename %s to %s: deletion of found dir failed: %w", path, newPath, err)
							}
						}

						//TODO: return is-dir error if there is a directory.
						return nil, fls.Rename(path, newPath)
					}
					return nil, fmt.Errorf("failed to rename %s to %s: file or dir found at new path and overwrite option is false ", path, newPath)
				}
			},
		})

		server.OnCustom(jsonrpc.MethodInfo{
			Name: "fs/deleteFile",
			NewRequest: func() interface{} {
				return &FsDeleteFileParams{}
			},
			Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				session := jsonrpc.GetSession(ctx)
				fls := session.Context().GetFileSystem()
				params := req.(*FsDeleteFileParams)

				path, err := getPath(params.FileURI, remoteFs)
				if err != nil {
					return nil, err
				}

				err = fls.Remove(path)

				if os.IsNotExist(err) {
					return FsFileNotFound, nil
				} else if err != nil { //exists
					return nil, fmt.Errorf("failed to delete %s: %w", path, err)
				}

				return nil, nil
			},
		})

		server.OnCustom(jsonrpc.MethodInfo{
			Name: "fs/readDir",
			NewRequest: func() interface{} {
				return &FsReadirParams{}
			},
			Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				session := jsonrpc.GetSession(ctx)
				fls := session.Context().GetFileSystem()
				params := req.(*FsReadirParams)

				dpath, err := getPath(params.DirURI, remoteFs)
				if err != nil {
					return nil, err
				}

				entries, err := fls.ReadDir(dpath)
				if err != nil {
					if os.IsNotExist(err) {
						return FsFileNotFound, nil
					}
					return nil, fmt.Errorf("failed to read dir %s", dpath)
				}

				var fsDirEntries FsDirEntries
				for _, e := range entries {
					fsDirEntries = append(fsDirEntries, FsDirEntry{
						Name:     e.Name(),
						FileType: FileTypeFromInfo(e),
					})
				}

				return fsDirEntries, nil
			},
		})

		server.OnCustom(jsonrpc.MethodInfo{
			Name: "fs/createDir",
			NewRequest: func() interface{} {
				return &FsCreateDirParams{}
			},
			Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				session := jsonrpc.GetSession(ctx)
				fls := session.Context().GetFileSystem()
				params := req.(*FsCreateDirParams)

				path, err := getPath(params.DirURI, remoteFs)
				if err != nil {
					return nil, err
				}

				err = fls.MkdirAll(path, fs_ns.DEFAULT_DIR_FMODE)
				if err != nil {
					return nil, err
				}

				return nil, nil
			},
		})
	}

}

func getFilePath(uri defines.DocumentUri, remoteFs bool) (string, error) {
	u, err := url.Parse(string(uri))
	if err != nil {
		return "", fmt.Errorf("invalid URI: %s: %w", uri, err)
	}
	if remoteFs && u.Scheme != "remotefs" {
		return "", fmt.Errorf("%w, URI is: %s", ErrRemoteFsURIExpected, string(uri))
	}
	if !remoteFs && u.Scheme != "file" {
		return "", fmt.Errorf("%w, URI is: %s", ErrFileURIExpected, string(uri))
	}
	return u.Path, nil
}

func getPath(uri defines.URI, remoteFs bool) (string, error) {
	u, err := url.Parse(string(uri))
	if err != nil {
		return "", fmt.Errorf("invalid URI: %s: %w", uri, err)
	}
	if remoteFs && u.Scheme != "remotefs" {
		return "", fmt.Errorf("%w, actual is: %s", ErrRemoteFsURIExpected, string(uri))
	}
	if !remoteFs && u.Scheme != "file" {
		return "", fmt.Errorf("%w, actual is: %s", ErrFileURIExpected, string(uri))
	}
	return u.Path, nil
}

func getCompletions(fpath string, compilationCtx *core.Context, line, column int32, session *jsonrpc.Session) []compl.Completion {
	fls := session.Context().GetFileSystem()

	state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
		Fpath:                     fpath,
		ParsingCompilationContext: compilationCtx,
		ParentContext:             nil,
		Out:                       io.Discard,
		IgnoreNonCriticalIssues:   true,
		AllowMissingEnvVars:       true,
		ScriptContextFileSystem:   fls,
		PreinitFilesystem:         fls,
	})

	if mod == nil { //unrecoverable parsing error
		logs.Println("unrecoverable parsing error", err.Error())
		session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
		return nil
	}

	if state == nil {
		logs.Println("error", err.Error())
		session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
		return nil
	}

	chunk := mod.MainChunk
	pos := chunk.GetLineColumnPosition(line, column)

	return compl.FindCompletions(compl.CompletionSearchArgs{
		State:       core.NewTreeWalkStateWithGlobal(state),
		Chunk:       chunk,
		CursorIndex: int(pos),
		Mode:        compl.LspCompletions,
	})
}

func rangeToLspRange(r parse.SourcePositionRange) defines.Range {
	return defines.Range{
		Start: defines.Position{
			Line:      uint(r.StartLine) - 1,
			Character: uint(r.StartColumn - 1),
		},
		End: defines.Position{
			Line:      uint(r.StartLine) - 1,
			Character: uint(r.StartColumn - 1 + r.Span.End - r.Span.Start),
		},
	}
}

func firstCharLspRange() defines.Range {
	return rangeToLspRange(parse.SourcePositionRange{
		StartLine:   1,
		StartColumn: 1,
		Span:        parse.NodeSpan{Start: 0, End: 1},
	})
}

func getLineColumn(pos defines.Position) (int32, int32) {
	line := int32(pos.Line + 1)
	column := int32(pos.Character + 1)
	return line, column
}
