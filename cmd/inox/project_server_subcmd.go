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
	"time"

	inox "github.com/inoxlang/inox"

	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/slog"
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/deno"
	denobinary "github.com/inoxlang/inox/internal/deno/binary"
	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/htmx"
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

const (
	LSP_SESSION_INIT_TIMEOUT = 2 * time.Second //The websocket is closed if the session is not initialized after this duration.

	DENO_BINARY_LOCATION = "/tmp/service-deno"
)

// ProjectServer starts the project server and some adjacent services. On success ProjectServer never returns.
func ProjectServer(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
	//Read CLI arguments.

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

	//Get the configuration of the project server (JSON).

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

	//Append port
	if projectServerConfig.Port > 0 {
		websocketAddr += strconv.Itoa(projectServerConfig.Port)
	} else {
		websocketAddr += inoxconsts.DEFAULT_PROJECT_SERVER_PORT
	}

	out := os.Stdout

	//Cleanup the temporary directories of dead inox processes.
	go func() {
		defer utils.Recover()

		logger := zerolog.New(out).With().Str(slog.SOURCE_FIELD_NAME, "temp-dir-cleanup").Logger()
		fs_ns.DeleteDeadProcessTempDirs(logger, TEMP_DIR_CLEANUP_TIMEOUT)

		logger = zerolog.New(out).With().Str(slog.SOURCE_FIELD_NAME, "temp-db-dir-cleanup").Logger()
		localdb.DeleteTempDatabaseDirsOfDeadProcesses(logger, TEMP_DB_DIR_CLEANUP_TIMEOUT)
	}()

	//Create a temporary directory for the whole process.
	_, _, removeTempDir := CreateTempDir()
	defer removeTempDir()

	if projectServerConfig.AllowBrowserAutomation {
		chrome_ns.AllowBrowserAutomation()

		//Download a chrome browser if not present. This is done synchronously because Landlock is invoked further in the code.
		downloadChromeBrowser(out, projectServerConfig)
	}

	//Initializations.

	utils.PanicIfErr(tailwind.InitSubset())
	htmx.Load()
	core.InoxCodebaseFS = inox.CodebaseFS

	//Create the root context.

	perms := determineProjectServerPermissions(projectServerConfig)

	filesystem := fs_ns.GetOsFilesystem()
	ctx := core.NewContext(core.ContextConfig{
		Permissions: perms,
		//The project server has access to the OS FS, but Inox applications don't.
		Filesystem:              filesystem,
		InitialWorkingDirectory: core.DirPathFrom(utils.Must(os.Getwd())),
	})

	state := core.NewGlobalState(ctx)
	state.Out = out
	state.Logger = zerolog.New(out)
	state.OutputFieldsInitialized.Store(true)

	CancelOnSigintSigterm(state.Ctx, ROOT_CTX_TEARDOWN_TIMEOUT)

	ctx.OnDone(func(timeoutCtx context.Context, teardownStatus core.GracefulTeardownStatus) error {
		//Stops the process 1s after the root context is done.
		//If the root context is cancelled because of an OS signal the code after the time.Sleep(...) call
		//will never get executed because the process should already have exited at this point (see CancelOnSigintSigterm).
		go func() {
			time.Sleep(time.Second)
			if teardownStatus == core.NeverStartedGracefulTeardown {
				os.Exit(2)
			} else {
				os.Exit(0) //Success
			}
		}()
		return nil
	})

	//Configure server

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
				//Set a filesystem that is not the OS FS. This FS will not be used.
				Filesystem: fs_ns.NewMemFilesystem(1_000),
			})
			tempState := core.NewGlobalState(sessionCtx)
			tempState.Out = out
			tempState.Logger = zerolog.New(out)
			tempState.OutputFieldsInitialized.Store(true)
			s.SetContextOnce(sessionCtx)

			//Close the websocket if the session is not initialized after LSP_SESSION_INIT_TIMEOUT.
			go func() {
				defer utils.Recover()

				time.Sleep(LSP_SESSION_INIT_TIMEOUT)

				if !projectserver.IsLspSessionInitialized(s) {
					s.Close()
				}
			}()
			return nil
		},
	}

	//Metrics and performance profiles collection.

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
		// Start the node agent in the same process (temporary solution).
		nodeAgent, err := nodeimpl.NewAgent(nodeimpl.AgentParameters{
			GoCtx:  ctx,
			Logger: zerolog.New(out).With().Str(slog.SOURCE_FIELD_NAME, "node-agent").Logger(),
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

	//Start adjacent services from another goroutine to speed things up.

	go func() {
		err := denobinary.Install(DENO_BINARY_LOCATION)

		if err == nil {
			//Restrict filesystem access at the process level.
			//This is done after the installation because Landlock needs to see the Deno binary.
			//Otherwise executing the binary will not be allowed.
			inoxprocess.RestrictProcessAccess(ctx, inoxprocess.ProcessRestrictionConfig{
				AllowBrowserAccess: projectServerConfig.AllowBrowserAutomation,
				BrowserBinPath:     chrome_ns.BROWSER_BINPATH,
			})

			err = startAdjacentServices(ctx, projectServerConfig)
		}

		if err != nil {
			fmt.Fprintln(errW, err)
			fmt.Fprintln(errW, "cancel the root context because some adjacent services failed to start")
			ctx.CancelGracefully()
		}
	}()

	//Start the project server and development servers.

	if err := projectserver.StartLSPServer(ctx, opts); err != nil {
		fmt.Fprintln(errW, "failed to start LSP server:", err)
		return ERROR_STATUS_CODE
	}
	return 0
}

func determineProjectServerPermissions(projectServerConfig projectserver.IndividualServerConfig) []core.Permission {

	const DEV_LOCALHOST_0 = core.Host("https://localhost:" + inoxconsts.DEV_PORT_0)
	const DEV_LOCALHOST_1 = core.Host("https://localhost:" + inoxconsts.DEV_PORT_1)
	const DEV_LOCALHOST_2 = core.Host("https://localhost:" + inoxconsts.DEV_PORT_2)

	perms := []core.Permission{
		//Filesystem permissions
		//TODO: change path patterns
		core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
		core.FilesystemPermission{Kind_: permbase.Write, Entity: core.PathPattern("/...")},
		core.FilesystemPermission{Kind_: permbase.Delete, Entity: core.PathPattern("/...")},

		//Websocket provide permission

		core.WebsocketPermission{Kind_: permbase.Provide},

		//HTTP Provide permissions

		core.HttpPermission{Kind_: permbase.Provide, Entity: core.ANY_HTTPS_HOST_PATTERN},
		core.HttpPermission{Kind_: permbase.Provide, Entity: core.HostPattern("https://**:" + inoxconsts.DEV_PORT_0)},
		core.HttpPermission{Kind_: permbase.Provide, Entity: core.HostPattern("https://**:" + inoxconsts.DEV_PORT_1)},
		core.HttpPermission{Kind_: permbase.Provide, Entity: core.HostPattern("https://**:" + inoxconsts.DEV_PORT_2)},
		core.HttpPermission{Kind_: permbase.Provide, Entity: core.HostPattern("http://" + chrome_ns.BROWSER_PROXY_ADDR)},

		//Default HTTP read|write|delete permissions

		core.HttpPermission{Kind_: permbase.Read, Entity: DEV_LOCALHOST_0},
		core.HttpPermission{Kind_: permbase.Write, Entity: DEV_LOCALHOST_0},
		core.HttpPermission{Kind_: permbase.Delete, Entity: DEV_LOCALHOST_0},

		core.HttpPermission{Kind_: permbase.Read, Entity: DEV_LOCALHOST_1},
		core.HttpPermission{Kind_: permbase.Write, Entity: DEV_LOCALHOST_1},
		core.HttpPermission{Kind_: permbase.Delete, Entity: DEV_LOCALHOST_1},

		core.HttpPermission{Kind_: permbase.Read, Entity: DEV_LOCALHOST_2},
		core.HttpPermission{Kind_: permbase.Write, Entity: DEV_LOCALHOST_2},
		core.HttpPermission{Kind_: permbase.Delete, Entity: DEV_LOCALHOST_2},

		//Lighweight thread permissions

		core.LThreadPermission{Kind_: permbase.Create},

		//Command permissions

		core.CommandPermission{CommandName: core.String(DENO_BINARY_LOCATION)}, //We need the permission because of landlock.
	}

	perms = append(perms, core.GetDefaultGlobalVarPermissions()...)

	//Add read+write+delete permissions for domains in the allow list.
	for _, domain := range projectServerConfig.DomainAllowList {
		httpsHost := core.Host("https://" + domain)
		httpHost := core.Host("http://" + domain)

		perms = append(perms,
			core.HttpPermission{
				Kind_:  permbase.Read,
				Entity: httpsHost,
			},
			core.HttpPermission{
				Kind_:  permbase.Read,
				Entity: httpHost,
			},
			core.HttpPermission{
				Kind_:  permbase.Write,
				Entity: httpsHost,
			},
			core.HttpPermission{
				Kind_:  permbase.Write,
				Entity: httpHost,
			},
			core.HttpPermission{
				Kind_:  permbase.Delete,
				Entity: httpsHost,
			},
			core.HttpPermission{
				Kind_:  permbase.Delete,
				Entity: httpHost,
			},
		)
	}

	//If no domains are specified in the allow list we add the permissions to make any HTTP request.
	if len(projectServerConfig.DomainAllowList) == 0 {
		perms = append(perms,
			core.HttpPermission{Kind_: permbase.Read, AnyEntity: true},
			core.HttpPermission{Kind_: permbase.Write, AnyEntity: true},
			core.HttpPermission{Kind_: permbase.Delete, AnyEntity: true})
	}

	return perms
}

func downloadChromeBrowser(out io.Writer, projectServerConfig projectserver.IndividualServerConfig) {
	defer utils.Recover()

	logger := zerolog.New(out).With().Str(slog.SOURCE_FIELD_NAME, "browser-installation").Logger()
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
}

func startAdjacentServices(ctx *core.Context, projectServerConfig projectserver.IndividualServerConfig) error {
	//Start the browser proxy.

	if projectServerConfig.AllowBrowserAutomation {
		err := chrome_ns.StartSharedProxy(ctx)
		if err != nil {
			return fmt.Errorf("failed to start shared browser proxy: %w", err)
		}
	}

	//Start some Deno services for internal use.

	{
		controlServerCtx := ctx.BoundChildWithOptions(core.BoundChildContextOptions{
			Limits: []core.Limit{
				utils.Must(core.GetLimit(ctx, fs_ns.FS_TOTAL_NEW_FILE_LIMIT_NAME, core.Int(10_000))),
				utils.Must(core.GetLimit(ctx, fs_ns.FS_NEW_FILE_RATE_LIMIT_NAME, core.Frequency(10*core.FREQ_LIMIT_SCALE))),
				utils.Must(core.GetLimit(ctx, fs_ns.FS_WRITE_LIMIT_NAME, core.ByteRate(10_000_000))),
				utils.Must(core.GetLimit(ctx, fs_ns.FS_READ_LIMIT_NAME, core.ByteRate(10_000_000))),
			},
		})
		controlServer, err := deno.NewControlServer(controlServerCtx, deno.ControlServerConfig{
			Port: inoxconsts.DEFAULT_DENO_CONTROL_SERVER_PORT_INT_FOR_PROJECT_SERVER,
		})

		if err != nil {
			return fmt.Errorf("failed to create control server for Deno processes: %w", err)
		}

		earlyErrChan := make(chan error)
		go func() {
			defer utils.Recover()
			earlyErrChan <- controlServer.Start()
		}()

		select {
		case err = <-earlyErrChan:
			if err != nil {
				return fmt.Errorf("failed to start control server for Deno processes: %w", err)
			}
		case <-time.After(100 * time.Millisecond):
		}
	}

	return nil
}
