package projectserver

import (
	"context"
	"io/fs"
	"strings"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/codecompletion"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func handleCompletion(ctx context.Context, req *defines.CompletionParams) (result *[]defines.CompletionItem, err error) {
	rpcSession := jsonrpc.GetSession(ctx)

	//--------------------------------------------------------
	session := getCreateLockedProjectSession(rpcSession)
	projectMode := session.inProjectMode
	memberAuthToken := session.memberAuthToken
	session.lock.Unlock()
	//--------------------------------------------------------

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		return nil, err
	}

	if memberAuthToken == "" {
		return nil, ErrMemberNotAuthenticated
	}

	line, column := getLineColumn(req.Position)

	completions := getCompletions(fpath, line, column, rpcSession, memberAuthToken)
	completionIndex := 0

	lspCompletions := utils.MapSlice(completions, func(completion codecompletion.Completion) defines.CompletionItem {
		defer func() {
			completionIndex++
		}()

		item := defines.CompletionItem{
			Label: completion.Value,
			Kind:  &completion.Kind,
			SortText: func() *string {
				index := completionIndex
				if index > 99 {
					index = 99
				}
				s := string(rune(index/10) + 'a')
				s += string(rune(index%10) + 'a')
				return &s
			}(),
		}

		if completion.LabelDetail != "" {
			detail := "  " + completion.LabelDetail
			item.LabelDetails = &defines.CompletionItemLabelDetails{
				Detail: &detail,
			}
		}

		if completion.MarkdownDocumentation != "" {
			item.Documentation = defines.MarkupContent{
				Kind:  defines.MarkupKindMarkdown,
				Value: completion.MarkdownDocumentation,
			}
		}

		if completion.ReplacedRange.Span != (parse.NodeSpan{}) {
			lspRange := rangeToLspRange(completion.ReplacedRange)

			// if completion.ReplacedRange.Span.Len() == 0 {
			// 	item.TextEdit = defines.InsertReplaceEdit{
			// 		Insert:  lspRange,
			// 		NewText: completion.Value,
			// 	}
			// } else {
			item.TextEdit = defines.TextEdit{
				Range:   lspRange,
				NewText: completion.Value,
			}
			//}

		}

		return item
	})
	return &lspCompletions, nil
}

// getCompletions gets the completions for a specific position in an Inox code file.
func getCompletions(fpath string, line, column int32, rpcSession *jsonrpc.Session, memberAuthToken string) []codecompletion.Completion {
	//----------------------------------------------------
	session := getCreateLockedProjectSession(rpcSession)

	fls := session.filesystem
	if fls == nil {
		session.lock.Unlock()
		return nil
	}

	lastCodebaseAnalysis := session.lastCodebaseAnalysis
	project := session.project
	lspFilesystem := session.filesystem
	chunkCache := session.inoxChunkCache
	session.lock.Unlock()
	//----------------------------------------------------

	handlingCtx := rpcSession.Context().BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	prepResult, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:         fpath,
		requiresState: true,

		rpcSession:      rpcSession,
		lspFilesystem:   lspFilesystem,
		project:         project,
		memberAuthToken: memberAuthToken,
		inoxChunkCache:  chunkCache,
	})

	if !ok {
		return nil
	}

	state := prepResult.state
	chunk := prepResult.chunk
	cachedOrHitCache := prepResult.cachedOrGotCache

	if !cachedOrHitCache && state != nil {
		//teardown in separate goroutine to return quickly
		defer func() {
			go func() {
				defer utils.Recover()
				state.Ctx.CancelGracefully()
			}()
		}()
	}

	if state == nil {
		return nil
	}

	pos := chunk.GetLineColumnPosition(line, column)
	staticResourcePaths := getStaticResourcePaths(fls, "/static")

	return codecompletion.FindCompletions(codecompletion.SearchArgs{
		State:       core.NewTreeWalkStateWithGlobal(state),
		Chunk:       chunk,
		CursorIndex: int(pos),
		Mode:        codecompletion.LspCompletions,

		InputData: codecompletion.InputData{
			StaticFileURLPaths: staticResourcePaths,
			CodebaseAnalysis:   lastCodebaseAnalysis,
		},
	})
}

func getStaticResourcePaths(fls afs.Filesystem, absStaticDir string) (paths []string) {
	//remove trailing slash
	if absStaticDir != "/" && absStaticDir[len(absStaticDir)-1] == '/' {
		absStaticDir = absStaticDir[:len(absStaticDir)-1]
	}

	core.WalkDirLow(fls, absStaticDir, func(path string, d fs.DirEntry, err error) error {
		paths = append(paths, strings.TrimPrefix(path, absStaticDir))
		return nil
	})

	return
}
