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
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_MAX_SCANNED_INOX_FILE_SIZE = 1_000_000
)

type Configuration struct {
	TopDirectories []string //note that if TopDirectories == {"/"} '/.dev' will be excluded.
	MaxFileSize    int64    //defaults to DEFAULT_MAX_SCANNED_INOX_FILE_SIZE
	Fast           bool     //if true the scan will be faster but will use more CPU and memory.
	Phases         []Phase

	ChunkCache            *parse.ChunkCache    //optional
	StylesheetParseCache  *css.StylesheetCache //optional
	HyperscriptParseCache *hscode.ParseCache   //optional
	FileParsingTimeout    time.Duration        //maximum duration for parsing a single file. defaults to parse.DEFAULT_TIMEOUT
}

type Phase struct {
	Name                    string
	InoxFileHandlers        []InoxFileHandler //File handlers are called for each inox file. They should not modify the chunk node.
	CSSFileHandlers         []CSSFileHandler  //File handlers are called for each CSS file. They should not modify the node.
	HyperscriptFileHandlers []HyperscriptFileHandler
}

type InoxFileHandler func(path string, fileContent string, n *parse.ParsedChunkSource, phaseName string) error
type CSSFileHandler func(path string, fileContent string, n css.Node, phaseName string) error
type HyperscriptFileHandler func(path string, fileContent string, parsingResult *hscode.ParsingResult, parsingError *hscode.ParsingError, phaseName string) error

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

	if err := ctx.CheckHasPermission(core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")}); err != nil {
		return err
	}

	if len(config.Phases) == 0 {
		return fmt.Errorf("no phases")
	}

	//Track the encountered files in order to remove deleted ASTs from the cache.
	seenInoxFiles := []string{}
	seenCssFiles := []string{}
	seenHyperscriptFiles := []string{}
	chunkCache := config.ChunkCache
	stylesheetCache := config.StylesheetParseCache
	hyperscriptParseCache := config.HyperscriptParseCache

	currentPhase := config.Phases[0]

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
		case inoxconsts.INOXLANG_FILE_EXTENSION, "._hs", ".css":
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

			contentS := string(content)

			sourceFile := parse.SourceFile{
				NameString:             path,
				UserFriendlyNameString: path,
				Resource:               path,
				ResourceDir:            filepath.Dir(path),
				IsResourceURL:          false,
				CodeString:             contentS,
			}

			result, _ := parse.ParseChunkSource(sourceFile, parse.ParserOptions{
				Timeout:         config.FileParsingTimeout,
				ParsedFileCache: chunkCache,
			})

			if result == nil { //critical error
				return nil
			}

			seenInoxFiles = append(seenInoxFiles, path)

			for _, handler := range currentPhase.InoxFileHandlers {
				err := handler(path, contentS, result, currentPhase.Name)

				if err != nil {
					return fmt.Errorf("an iNox file handler returned an error for %s", path)
				}
			}
		//Hyperscript file ----------------------------------------------------------------------------
		case "._hs":
			var (
				parsingResult *hscode.ParsingResult
				parsingError  *hscode.ParsingError
				cacheHit      bool
			)

			contentS := string(content)

			//Check the cache.
			if hyperscriptParseCache != nil {
				parsingResult, cacheHit = hyperscriptParseCache.GetResult(contentS)
			}

			if !cacheHit {
				//Parse the file.

				parsingResult, parsingError, err = hsparse.ParseHyperScriptProgram(ctx, contentS)
				if err != nil {
					return nil
				}

				//Update the cache.
				if hyperscriptParseCache != nil {
					hyperscriptParseCache.Put(path, contentS, parsingResult, parsingError)
				}
			}
			seenHyperscriptFiles = append(seenHyperscriptFiles, path)

			for _, handler := range currentPhase.HyperscriptFileHandlers {
				err := handler(path, contentS, parsingResult, parsingError, currentPhase.Name)

				if err != nil {
					return fmt.Errorf("an Hyperscript file handler returned an error for %s", path)
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
			seenCssFiles = append(seenCssFiles, path)

			for _, handler := range currentPhase.CSSFileHandlers {
				err := handler(path, contentS, *stylesheet, currentPhase.Name)

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

	for _, phase := range config.Phases {
		currentPhase = phase
		for _, topDir := range topDirs {
			err := core.WalkDirLow(fls, topDir, handleFile)

			if err != nil {
				return err
			}
		}
	}

	//Remove the cache entries of old file versions.
	if chunkCache != nil {
		chunkCache.KeepEntriesByPath(seenInoxFiles...)
	}
	if stylesheetCache != nil {
		stylesheetCache.KeepEntriesByPath(seenCssFiles...)
	}
	if hyperscriptParseCache != nil {
		hyperscriptParseCache.KeepEntriesByPath(seenHyperscriptFiles...)
	}

	return nil
}
