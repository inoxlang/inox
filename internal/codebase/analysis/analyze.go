package analysis

import (
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/codebase/scan"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/parse"
)

type Configuration struct {
	TopDirectories []string
	MaxFileSize    int64 //defaults to scan.DEFAULT_MAX_SCANNED_INOX_FILE_SIZE
	InoxChunkCache *parse.ChunkCache
}

func AnalyzeCodebase(ctx *core.Context, fls afs.Filesystem, config Configuration) (*Result, error) {

	result := newEmptyResult()

	err := scan.ScanCodebase(ctx, fls, scan.Configuration{
		TopDirectories:     config.TopDirectories,
		ChunkCache:         config.InoxChunkCache,
		FileParsingTimeout: 50 * time.Millisecond,
		MaxFileSize:        config.MaxFileSize,

		FileHandlers: []scan.FileHandler{
			func(path, fileContent string, n *parse.Chunk) error {
				analyzeInoxFile(path, n, result)
				return nil
			},
		},
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
