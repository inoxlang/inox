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
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_MAX_SCANNED_INOX_FILE_SIZE = 1_000_000
)

type Configuration struct {
	TopDirectories     []string          //note that if TopDirectories == {"/"} '/.dev' will be excluded.
	MaxFileSize        int64             //defaults to DEFAULT_MAX_SCANNED_INOX_FILE_SIZE
	Fast               bool              //if true the scan will be faster but will use more CPU and memory.
	FileHandlers       []FileHandler     //File handlers are called for each file. They should not modify the chunk node.
	ChunkCache         *parse.ChunkCache //optional
	FileParsingTimeout time.Duration     //maximum duration for parsing a single file. defaults to parse.DEFAULT_TIMEOUT
}

type FileHandler func(path string, fileContent string, n *parse.Chunk) error

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
	chunkCache := config.ChunkCache

	for _, topDir := range topDirs {

		err := core.WalkDirLow(fls, topDir, func(path string, d fs.DirEntry, err error) error {

			if ctx.IsDoneSlowCheck() {
				return ctx.Err()
			}

			//Ignore /.dev
			if d.IsDir() && excludeRootDotDev && path == "/"+inoxconsts.DEV_DIR_NAME {
				return fs.SkipDir
			}

			//Ignore non-Inox files.
			//TODO: scan some other file types. The parser should change accordingly.
			if d.IsDir() || filepath.Ext(path) != inoxconsts.INOXLANG_FILE_EXTENSION {
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

			var (
				chunk    *parse.Chunk
				cacheHit bool
			)

			contentS := string(content)

			//Check the cache.
			if chunkCache != nil {
				chunk, cacheHit = chunkCache.Get(contentS)
			}

			if !cacheHit {

				//Parse the file.

				result, _ := parse.ParseChunk(contentS, path, parse.ParserOptions{
					Timeout: config.FileParsingTimeout,
				})
				if result == nil { //critical error
					return nil
				}

				chunk = result

				//Update the cache.
				if chunkCache != nil {
					config.ChunkCache.Put(contentS, result)
				}
			}
			seenChunks = append(seenChunks, chunk)

			for _, handler := range config.FileHandlers {
				err := handler(path, contentS, chunk)

				if err != nil {
					return fmt.Errorf("a file handler returned an error for %s", path)
				}
			}

			if !config.Fast {
				runtime.Gosched()
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	//Remove the cache entries of old file versions.
	if config.ChunkCache != nil {
		chunkCache.KeepEntriesByValue(seenChunks...)
	}

	return nil
}
