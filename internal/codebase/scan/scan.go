package scan

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_MAX_SCANNED_INOX_FILE_SIZE = 1_000_000
)

type Configuration struct {
	TopDirectories       []string             //note that if TopDirectories == {"/"} '/.dev' will be excluded.
	MaxFileSize          int64                //defaults to DEFAULT_MAX_SCANNED_INOX_FILE_SIZE
	Fast                 bool                 //if true the scan will be faster but will use more CPU and memory.
	InoxFileHandlers     []InoxFileHandler    //File handlers are called for each inox file. They should not modify the chunk node.
	CSSFileHandlers      []CSSFileHandler     //File handlers are called for each inox file. They should not modify the node.
	ChunkCache           *parse.ChunkCache    //optional
	StylesheetParseCache *css.StylesheetCache //optional
	FileParsingTimeout   time.Duration        //maximum duration for parsing a single file. defaults to parse.DEFAULT_TIMEOUT
}

type InoxFileHandler func(path string, fileContent string, n *parse.Chunk) error
type CSSFileHandler func(path string, fileContent string, n css.Node) error

func ScanCodebase(ctx *core.Context, fls afs.Filesystem, config Configuration) error {

	maxFileSize := utils.DefaultIfZero(config.MaxFileSize, DEFAULT_MAX_SCANNED_INOX_FILE_SIZE)

	topDirs := utils.MapSlice(slices.Clone(config.TopDirectories), filepath.Clean)
	{
		// Remove duplicates
		sort.Strings(topDirs)
		for i := 0; i < len(topDirs); i++ {
			if topDirs[i] == "." {
				return fmt.Errorf("some top directories are invalid among %s", strings.Join(config.TopDirectories, ","))
			}
			if i > 0 && topDirs[i] == topDirs[i-1] {
				topDirs = slices.Delete(topDirs, i, i+1)
			}
		}
	}
	excludeRootDotDev := len(topDirs) == 1 && topDirs[0] == "/"

	if err := ctx.CheckHasPermission(core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}); err != nil {
		return err
	}

	//Track the cached chunks in order
	seenChunks := []*parse.Chunk{}
	seenStylesheets := []*css.Node{}
	chunkCache := config.ChunkCache
	stylesheetCache := config.StylesheetParseCache

	handleFile := func(path string, d fs.DirEntry, err error) error {
		if ctx.IsDoneSlowCheck() {
			return ctx.Err()
		}

		//Ignore /.dev
		if d.IsDir() && excludeRootDotDev && path == "/"+inoxconsts.DEV_DIR_NAME {
			return fs.SkipDir
		}

		//Ignore directories.
		if d.IsDir() {
			return nil
		}

		//Ignore large files.
		stat, err := fls.Stat(path)
		if err != nil {
			if os.IsNotExist(err) { //The file may have been deleted by the developer.
				return nil
			}
			return err
		}

		if stat.Size() > maxFileSize { //ignore file
			return nil
		}

		switch filepath.Ext(path) {
		case inoxconsts.INOXLANG_FILE_EXTENSION, ".css":
		default:
			return nil
		}

		//Open and read the file.

		f, err := fls.Open(path)
		if err != nil {
			if os.IsNotExist(err) { //The file may have been deleted by the developer.
				return nil
			}
			return err
		}

		var content []byte

		func() {
			defer f.Close()
			content, err = io.ReadAll(io.LimitReader(f, maxFileSize))
		}()

		if err != nil {
			return err
		}

		switch filepath.Ext(path) {
		//Inox file ----------------------------------------------------------------------------
		case inoxconsts.INOXLANG_FILE_EXTENSION:
			var (
				chunk    *parse.Chunk
				cacheHit bool
			)

			contentS := string(content)

			//Check the cache.
			if chunkCache != nil {
				chunk, cacheHit = chunkCache.GetResult(contentS)
			}

			if !cacheHit {

				//Parse the file.

				result, err := parse.ParseChunk(contentS, path, parse.ParserOptions{
					Timeout: config.FileParsingTimeout,
				})
				if result == nil { //critical error
					return nil
				}

				chunk = result

				//Update the cache.
				if chunkCache != nil {
					config.ChunkCache.Put(path, contentS, result, err)
				}
			}
			seenChunks = append(seenChunks, chunk)

			for _, handler := range config.InoxFileHandlers {
				err := handler(path, contentS, chunk)

				if err != nil {
					return fmt.Errorf("an iNox file handler returned an error for %s", path)
				}
			}
		//CSS file ----------------------------------------------------------------------------
		case ".css":
			var (
				stylesheet *css.Node
				cacheHit   bool
			)

			contentS := string(content)

			//Check the cache.
			if stylesheetCache != nil {
				stylesheet, cacheHit = stylesheetCache.GetResult(contentS)
			}

			if !cacheHit {

				//Parse the file.

				result, err := css.ParseString(ctx, contentS)
				if err != nil {
					return nil
				}

				stylesheet = &result

				//Update the cache.
				if stylesheetCache != nil {
					stylesheetCache.Put(path, contentS, stylesheet, err)
				}
			}
			seenStylesheets = append(seenStylesheets, stylesheet)

			for _, handler := range config.CSSFileHandlers {
				err := handler(path, contentS, *stylesheet)

				if err != nil {
					return fmt.Errorf("a CSS file handler returned an error for %s", path)
				}
			}
		}

		if !config.Fast {
			runtime.Gosched()
		}

		return nil
	}

	for _, topDir := range topDirs {
		err := core.WalkDirLow(fls, topDir, handleFile)

		if err != nil {
			return err
		}
	}

	//Remove the cache entries of old file versions.
	if chunkCache != nil {
		chunkCache.KeepEntriesByParsingResult(seenChunks...)
	}
	if stylesheetCache != nil {
		stylesheetCache.KeepEntriesByParsingResult(seenStylesheets...)
	}

	return nil
}
