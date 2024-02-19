package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/inoxd/nodeimpl"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/localdb"
	"github.com/inoxlang/inox/internal/metricsperf"
	"github.com/inoxlang/inox/internal/projectserver"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

func ProjectServer(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
	//read & check arguments
	flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
	var configOrConfigFile string

	flags.StringVar(&configOrConfigFile, "config", "", "JSON configuration or JSON file")

	if showHelp(flags, mainSubCommandArgs, outW) { //only show help
		return
	}

	err := flags.Parse(mainSubCommandArgs)
	if err != nil {
		fmt.Fprintln(errW, "project-server:", err)
		return
	}

	var projectServerConfig projectserver.IndividualServerConfig

	configOrConfigFile = strings.TrimSpace(configOrConfigFile)
	if configOrConfigFile != "" {
		if configOrConfigFile[0] == '{' {
			err := json.Unmarshal([]byte(configOrConfigFile), &projectServerConfig)
			if err != nil {
				fmt.Fprintln(errW, "project-server: failed to unmarshal configuration argument:", err)
				return ERROR_STATUS_CODE
			}
		} else {
			content, err := os.ReadFile(configOrConfigFile)
			if err != nil {
				fmt.Fprintln(errW, "project-server: failed to read configuration file:", err)
				return ERROR_STATUS_CODE
			}
			err = json.Unmarshal(content, &projectServerConfig)
			if err != nil {
				fmt.Fprintln(errW, "project-server: failed to unmarshal configuration file:", err)
				return ERROR_STATUS_CODE
			}
		}
	}

	projectsDir := projectServerConfig.ProjectsDir
	if projectsDir == "" {
		projectsDir = filepath.Join(config.USER_HOME, "inox-projects") + "/"
	}

	var prodDir core.Path
	if projectServerConfig.ProdDir != "" {
		prodDir = core.DirPathFrom(projectServerConfig.ProdDir)
	}

	websocketAddr := ""

	if projectServerConfig.BindToAllInterfaces {
		websocketAddr = ":"
	} else {
		websocketAddr = "localhost:"
	}

	//append port
	if projectServerConfig.Port > 0 {
		websocketAddr += strconv.Itoa(projectServerConfig.Port)
	} else {
		websocketAddr += projectserver.DEFAULT_PROJECT_SERVER_PORT
	}

	out := os.Stdout

	//cleanup the temporary directories of dead inox processes.
	go func() {
		defer utils.Recover()

		logger := zerolog.New(out).With().Str(core.SOURCE_LOG_FIELD_NAME, "temp-dir-cleanup").Logger()
		fs_ns.DeleteDeadProcessTempDirs(logger, TEMP_DIR_CLEANUP_TIMEOUT)

		logger = zerolog.New(out).With().Str(core.SOURCE_LOG_FIELD_NAME, "temp-db-dir-cleanup").Logger()
		localdb.DeleteTempDatabaseDirsOfDeadProcesses(logger, TEMP_DB_DIR_CLEANUP_TIMEOUT)
	}()

	//create a temporary directory for the whole process
	_, _, removeTempDir := CreateTempDir()
	defer removeTempDir()

	if projectServerConfig.AllowBrowserAutomation {
		chrome_ns.AllowBrowserAutomation()

		//download a chrome browser if not present.
		//this is done synchronously because Landlock is invoked further in the code.
		func() {
			defer utils.Recover()

			logger := zerolog.New(out).With().Str(core.SOURCE_LOG_FIELD_NAME, "browser-installation").Logger()
			downloadCtx, cancel := context.WithTimeout(context.Background(), BROWSER_DOWNLOAD_TIMEOUT)
			defer cancel()

			if !projectServerConfig.IgnoreInstalledBrowser {
				path, ok := chrome_ns.LookPath()
				if ok {
					logger.Info().Msgf("chrome browser found at %q\n", path)
					chrome_ns.SetBrowserBinPath(path)
					return
				}
			} else {
				logger.Info().Msgf("any browser not installed by the project server will be ignored")
			}

			binpath, err := chrome_ns.DownloadBrowser(downloadCtx, logger)
			if err != nil {
				logger.Err(err).Msg("failed to download a browser")
				return
			}
			logger.Info().Msgf("set browser binary path to %s", binpath)
			chrome_ns.SetBrowserBinPath(binpath)
		}()
	}

	//create context & state
	perms := []core.Permission{
		//TODO: change path patterns
		core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
		core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.PathPattern("/...")},

		core.WebsocketPermission{Kind_: permkind.Provide},
		core.HttpPermission{Kind_: permkind.Provide, Entity: core.ANY_HTTPS_HOST_PATTERN},
		core.HttpPermission{Kind_: permkind.Provide, Entity: core.HostPattern("https://**:" + inoxconsts.DEV_PORT_0)},
		core.HttpPermission{Kind_: permkind.Provide, Entity: core.HostPattern("http://" + chrome_ns.BROWSER_PROXY_ADDR)},

		core.HttpPermission{Kind_: permkind.Read, AnyEntity: true},
		core.HttpPermission{Kind_: permkind.Write, AnyEntity: true},
		core.HttpPermission{Kind_: permkind.Delete, AnyEntity: true},

		core.LThreadPermission{Kind_: permkind.Create},
	}

	perms = append(perms, core.GetDefaultGlobalVarPermissions()...)

	filesystem := fs_ns.GetOsFilesystem()
	ctx := core.NewContext(core.ContextConfig{
		Permissions:             perms,
		Filesystem:              filesystem,
		InitialWorkingDirectory: core.DirPathFrom(utils.Must(os.Getwd())),
	})

	state := core.NewGlobalState(ctx)
	state.Out = out
	state.Logger = zerolog.New(out)
	state.OutputFieldsInitialized.Store(true)

	CancelOnSigintSigterm(state.Ctx, ROOT_CTX_TEARDOWN_TIMEOUT)

	//restrict filesystem access at the process level.
	inoxprocess.RestrictProcessAccess(ctx, inoxprocess.ProcessRestrictionConfig{
		AllowBrowserAccess: projectServerConfig.AllowBrowserAutomation,
		BrowserBinPath:     chrome_ns.BROWSER_BINPATH,
	})

	//configure server

	opts := projectserver.LSPServerConfiguration{
		Websocket: &projectserver.WebsocketServerConfiguration{
			Addr:              websocketAddr,
			MaxWebsocketPerIp: projectServerConfig.MaxWebSocketPerIp,
			BehindCloudProxy:  projectServerConfig.BehindCloudProxy,
		},
		UseContextLogger: true,
		ProjectMode:      true,
		ProjectsDir:      core.DirPathFrom(projectsDir),
		ProdDir:          prodDir,
		ExposeWebServers: projectServerConfig.ExposeWebServers,

		ProjectsDirFilesystem: ctx.GetFileSystem(),
		OnSession: func(rpcCtx *core.Context, s *jsonrpc.Session) error {
			//Create the core.Context for the LSP session.
			sessionCtx := core.NewContext(core.ContextConfig{
				Permissions:          rpcCtx.GetGrantedPermissions(),
				ForbiddenPermissions: rpcCtx.GetForbiddenPermissions(),
				Limits:               core.GetDefaultScriptLimits(),

				ParentContext: rpcCtx,
			})
			tempState := core.NewGlobalState(sessionCtx)
			tempState.Out = out
			tempState.Logger = zerolog.New(out)
			tempState.OutputFieldsInitialized.Store(true)
			s.SetContextOnce(sessionCtx)
			return nil
		},
	}

	if config.METRICS_PERF_BUCKET_NAME == "" {
		fmt.Fprintln(errW, "credentials of metrics-perf bucket are missing; no metrics will be collected.")
	} else {
		_, err = metricsperf.StartPeriodicPerfProfilesCollection(ctx, metricsperf.PerfDataCollectionConfig{
			ProfileSavePeriod: PERF_PROFILES_COLLECTION_SAVE_PERIOD,
			Bucket: s3_ns.OpenBucketWithCredentialsInput{
				Provider:   config.METRICS_PERF_BUCKET_PROVIDER,
				HttpsHost:  config.METRICS_PERF_BUCKET_ENDPOINT,
				AccessKey:  config.METRICS_PERF_BUCKET_ACCESS_KEY,
				SecretKey:  config.METRICS_PERF_BUCKET_SECRET_KEY.StringValue().GetOrBuildString(),
				BucketName: config.METRICS_PERF_BUCKET_NAME,
			},
		})

		if err != nil {
			fmt.Fprintln(errW, "failed to start collection of perfomance profiles:", err)
			return ERROR_STATUS_CODE
		}
	}

	if prodDir != "" && !projectServerConfig.BehindCloudProxy {
		// start the node agent in the same process (temporary solution)
		nodeAgent, err := nodeimpl.NewAgent(nodeimpl.AgentParameters{
			GoCtx:  ctx,
			Logger: zerolog.New(out).With().Str(core.SOURCE_LOG_FIELD_NAME, "node-agent").Logger(),
			Config: nodeimpl.AgentConfig{
				OsProdDir:                       prodDir,
				TemporaryOptionRunInSameProcess: true,
			},
		})

		if err != nil {
			fmt.Fprintln(errW, "failed to start node agent:", err)
			return ERROR_STATUS_CODE
		}

		node.SetAgent(nodeAgent)
	}

	//Start the browser proxy.

	if projectServerConfig.AllowBrowserAutomation {
		err = chrome_ns.StartSharedProxy(ctx)
		if err != nil {
			fmt.Fprintln(errW, "failed to start shared browser proxy:", err)
			return ERROR_STATUS_CODE
		}
	}

	//Start the project server and development servers.

	if err := projectserver.StartLSPServer(ctx, opts); err != nil {
		fmt.Fprintln(errW, "failed to start LSP server:", err)
		return ERROR_STATUS_CODE
	}
	return 0
}
