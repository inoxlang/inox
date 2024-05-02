package analysis

import (
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/codebase/scan"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/css"
	httpspec "github.com/inoxlang/inox/internal/globals/http_ns/spec"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/project/layout"
)

const FILE_PARSING_TIMEOUT = 50 * time.Millisecond

type Configuration struct {
	TopDirectories []string

	MaxFileSize     int64 //defaults to scan.DEFAULT_MAX_SCANNED_INOX_FILE_SIZE
	Project         *project.Project
	MemberAuthToken string

	//Cache

	ModuleCache        *core.PreparationCache
	InoxChunkCache     *parse.ChunkCache
	CssStylesheetCache *css.StylesheetCache
}

type analyzer struct {
	result             *Result
	fls                afs.Filesystem
	fileParsingTimeout time.Duration
	ctx                *core.Context

	Configuration
}

// AnalyzeCodebase analyzes several types of files in codebase: Inox and CSS files.
// The filesystem is obtained from the context.
func AnalyzeCodebase(ctx *core.Context, config Configuration) (*Result, error) {

	analyzer := &analyzer{
		Configuration:      config,
		ctx:                ctx,
		result:             newEmptyResult(),
		fls:                ctx.GetFileSystem(),
		fileParsingTimeout: FILE_PARSING_TIMEOUT,
	}

	return analyzer.analyzeCodebase(ctx)
}

func (a *analyzer) analyzeCodebase(ctx *core.Context) (result *Result, finalErr error) {
	config := a.Configuration

	// ------------------- Pre-analysis phase -------------------
	// We gather information about files individually.

	err := scan.ScanCodebase(ctx, a.fls, scan.Configuration{
		TopDirectories:       config.TopDirectories,
		ChunkCache:           config.InoxChunkCache,
		StylesheetParseCache: config.CssStylesheetCache,
		FileParsingTimeout:   a.fileParsingTimeout,
		MaxFileSize:          config.MaxFileSize,

		InoxFileHandlers: []scan.InoxFileHandler{
			a.preAnalyzeInoxFile,
		},
		CSSFileHandlers: []scan.CSSFileHandler{
			func(path, fileContent string, n css.Node) error {
				a.addCssVariables(n)
				return nil
			},
		},
	})

	if err != nil {
		return nil, err
	}

	result = a.result

	//------------------- True analysis  -------------------

	mainProgramPath := layout.MAIN_PROGRAM_PATH

	//Determine the server API and handler modules.

	appModuleEntry, ok := a.result.InoxModules[mainProgramPath]
	if ok || appModuleEntry.SymbolicData != nil {
		staticDir, dynamicDir := a.tryGetStaticAndDynamicDirs(appModuleEntry)

		result.ServerStaticDir = staticDir

		if dynamicDir != "" {
			api, err := httpspec.GetFSRoutingServerAPI(a.ctx, httpspec.ServerApiResolutionConfig{
				IgnoreModulesWithErrors: true,
				DynamicDir:              dynamicDir,
				InoxChunkCache:          a.InoxChunkCache,
				FallbackMainProgramPath: mainProgramPath,
				FallbackProject:         a.Project,
				MemberAuthToken:         a.MemberAuthToken,
			})

			if err != nil {
				return nil, err
			}
			result.ServerAPI = api
			result.ServerDynamicDir = dynamicDir
		}
	}

	//Analyze Hyperscript code.

	return
}
