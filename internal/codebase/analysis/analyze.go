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
	"github.com/inoxlang/inox/internal/utils"
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
		result:             NewEmptyResult(),
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

		Phases: []scan.Phase{
			{
				Name: "0",
				InoxFileHandlers: []scan.InoxFileHandler{
					a.prepareIfDatabaseProvidingModule,
				},
				HyperscriptFileHandlers: []scan.HyperscriptFileHandler{
					a.analyzeHyperscriptFile,
				},
			},
			{
				Name: "1",
				InoxFileHandlers: []scan.InoxFileHandler{
					a.preAnalyzeInoxFile,
				},
				CSSFileHandlers: []scan.CSSFileHandler{
					func(path, fileContent string, n css.Node, _ string) error {
						a.addCssVariables(n)
						return nil
					},
				},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	result = a.result

	defer func() {
		// Remove .state from info on local modules and cancel contexts.

		var contexts []*core.Context

		for path, modInfo := range result.LocalModules {
			contexts = append(contexts, modInfo.state.Ctx)
			modInfo.state = nil
			result.LocalModules[path] = modInfo
		}

		go func() {
			defer utils.Recover()

			for _, ctx := range contexts {
				ctx.CancelGracefully()
			}
		}()
	}()

	//------------------- True analysis  -------------------

	mainProgramPath := layout.MAIN_PROGRAM_PATH

	//Determine the server API and handler modules.

	appModuleEntry, ok := a.result.LocalModules[mainProgramPath]
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

	for _, sameNameBehaviors := range result.HyperscriptBehaviors {
		for _, behavior := range sameNameBehaviors {
			err := a.analyzeHyperscriptBehavior(behavior)
			if err != nil {
				return nil, err
			}
		}
	}

	for _, sameNameComponents := range result.HyperscriptComponents {
		for _, component := range sameNameComponents {
			err := a.analyzeHyperscriptComponent(component)
			if err != nil {
				return nil, err
			}
		}
	}

	return
}
