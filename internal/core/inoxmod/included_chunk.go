package inoxmod

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

// An IncludedChunk represents an Inox chunk that is included in another chunk,
// it does not hold any state and should NOT be modified.
type IncludedChunk struct {
	*parse.ParsedChunkSource
	IncludedChunkForest []*IncludedChunk
	OriginalErrors      []*parse.ParsingError
	Errors              []Error
}

type LocalSecondaryChunkParsingConfig struct {
	Context                  Context
	SingleFileParsingTimeout time.Duration
	ChunkCache               *parse.ChunkCache
	ChunkFilepath            string

	Module                              *Module
	TopLevelImportPosition              parse.SourcePositionRange
	ImportPosition                      parse.SourcePositionRange
	RecoverFromNonExistingIncludedFiles bool
}

func ParseIncludedChunk(config LocalSecondaryChunkParsingConfig) (_ *IncludedChunk, absolutePath string, _ error) {
	fpath := config.ChunkFilepath
	ctx := config.Context
	fls := ctx.GetFileSystem()
	mod := config.Module

	if strings.Contains(fpath, "..") {
		return nil, "", errors.New(INCLUDED_FILE_PATH_SHOULD_NOT_CONTAIN_X)
	}

	absPath, err := fls.Absolute(fpath)
	if err != nil {
		return nil, "", err
	}

	if alreadyIncludedChunk, ok := mod.IncludedChunkMap[absPath]; ok {
		return alreadyIncludedChunk, absPath, fmt.Errorf("%w: %s", ErrFileAlreadyIncluded, absPath)
	}

	//read the file

	{
		readPerm := CreateReadFilePermission(absPath)
		if err := config.Context.CheckHasPermission(readPerm); err != nil {
			return nil, "", fmt.Errorf("failed to parse included chunk %s: %w", config.ChunkFilepath, err)
		}
	}

	src := parse.SourceFile{
		NameString:             absPath,
		UserFriendlyNameString: fpath, //fpath is probably equal to absPath since config.ChunkFilepath is absolute (?).
		Resource:               absPath,
		ResourceDir:            filepath.Dir(absPath),
		IsResourceURL:          false,
	}

	var existenceError error

	file, err := ctx.GetFileSystem().OpenFile(fpath, os.O_RDONLY, 0)

	var info fs.FileInfo
	if err == nil {
		info, err = afs.FileStat(file, fls)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get information for file to include %s: %w", fpath, err)
		}
	}

	if os.IsNotExist(err) {
		if !config.RecoverFromNonExistingIncludedFiles {
			return nil, "", err
		}

		existenceError = fmt.Errorf("%w: %s", ErrFileToIncludeDoesNotExist, fpath)
	} else if err == nil && info.IsDir() {
		if !config.RecoverFromNonExistingIncludedFiles {
			return nil, "", err
		}

		existenceError = fmt.Errorf("%w: %s", ErrFileToIncludeIsAFolder, fpath)
	} else {
		if err != nil {
			return nil, "", fmt.Errorf("failed to open included file %s: %s", fpath, err)
		}

		b, err := io.ReadAll(file)

		if err != nil {
			return nil, "", fmt.Errorf("failed to read included file %s: %s", fpath, err)
		}

		src.CodeString = utils.BytesAsString(b)
	}

	//parse

	chunk, err := parse.ParseChunkSource(src, parse.ParserOptions{
		ParentContext:   config.Context,
		Timeout:         config.SingleFileParsingTimeout,
		ParsedFileCache: config.ChunkCache,
	})

	if err != nil && chunk == nil { //critical error
		return nil, "", fmt.Errorf("failed to parse included file %s: %w", fpath, err)
	}

	isModule := chunk != nil && chunk.Node.Manifest != nil

	includedChunk := &IncludedChunk{
		ParsedChunkSource: chunk,
	}

	if isModule {
		// Add error and return.
		includedChunk.Errors = append(includedChunk.Errors, Error{
			BaseError:      fmt.Errorf("included files should not contain a manifest: %s", fpath),
			AdditionalInfo: fpath,
			Position:       config.ImportPosition,
		},
		)
		return includedChunk, "", ErrNotAnIncludableFile
	}

	// add parsing errors to the included chunk
	if existenceError != nil {
		includedChunk.Errors = []Error{
			{
				BaseError:      existenceError,
				AdditionalInfo: fpath,
				Position:       config.ImportPosition,
			},
		}
	} else if err != nil {
		errorAggregation, ok := err.(*parse.ParsingErrorAggregation)
		if !ok {
			panic(ErrUnreachable)
		}
		includedChunk.OriginalErrors = append(mod.FileLevelParsingErrors, errorAggregation.Errors...)
		includedChunk.Errors = make([]Error, len(errorAggregation.Errors))

		for i, err := range errorAggregation.Errors {
			pos := errorAggregation.ErrorPositions[i]
			includedChunk.Errors[i] = Error{
				BaseError: err,
				Position:  pos,
			}
		}
	}

	if existenceError == nil && chunk.Node.IncludableChunkDesc == nil {
		// Add an error if the includable-file

		includedChunk.Errors = append(includedChunk.Errors, Error{
			BaseError:      fmt.Errorf("included files should start with the %s keyword: %s", parse.INCLUDABLE_CHUNK_KEYWORD_STR, fpath),
			AdditionalInfo: fpath,
			Position:       config.ImportPosition,
		})
	}

	inclusionStmts := parse.FindNodes(chunk.Node, (*parse.InclusionImportStatement)(nil), nil)

	for _, stmt := range inclusionStmts {
		//ignore import if the source has an error
		if config.RecoverFromNonExistingIncludedFiles && (stmt.Source == nil || stmt.Source.Base().Err != nil) {
			continue
		}

		path, isAbsolute := stmt.PathSource()
		chunkFilepath := path

		if !isAbsolute {
			chunkFilepath = fls.Join(src.ResourceDir, path)
		}

		stmtPos := chunk.GetSourcePosition(stmt.Span)

		childChunk, absoluteChunkPath, err := ParseIncludedChunk(LocalSecondaryChunkParsingConfig{
			Context:                  config.Context,
			ChunkFilepath:            chunkFilepath,
			SingleFileParsingTimeout: config.SingleFileParsingTimeout,
			ChunkCache:               config.ChunkCache,

			Module:                              mod,
			ImportPosition:                      stmtPos,
			TopLevelImportPosition:              config.TopLevelImportPosition,
			RecoverFromNonExistingIncludedFiles: config.RecoverFromNonExistingIncludedFiles,
		})

		if err != nil && childChunk == nil {
			return nil, "", err
		}

		if errors.Is(err, ErrFileAlreadyIncluded) {
			//mod.InclusionStatementMap[stmt] = includedChunk

			//Add the error at the import in the module.

			err := Error{
				BaseError: err,
				Position:  config.TopLevelImportPosition,
			}

			mod.Errors = append(mod.Errors, err)

			if slices.Contains(includedChunk.IncludedChunkForest, childChunk) {
				//TODO: also add the error at the import in the included file (importer) if the inclusion is duplicated
				//in its subtree but in a different way.

				relocatedError := err
				relocatedError.Position = stmtPos
				includedChunk.Errors = append(includedChunk.Errors, relocatedError)
			}

			continue
		}

		includedChunk.OriginalErrors = append(mod.FileLevelParsingErrors, childChunk.OriginalErrors...)
		includedChunk.Errors = append(includedChunk.Errors, childChunk.Errors...)

		if !errors.Is(err, ErrNotAnIncludableFile) {
			mod.InclusionStatementMap[stmt] = childChunk
			mod.IncludedChunkMap[absoluteChunkPath] = childChunk
			includedChunk.IncludedChunkForest = append(includedChunk.IncludedChunkForest, childChunk)
			mod.FlattenedIncludedChunkList = append(mod.FlattenedIncludedChunkList, childChunk)
		}
	}

	return includedChunk, absPath, nil
}
