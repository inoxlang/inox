package internal

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/inoxlang/inox/internal/config"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"golang.org/x/exp/maps"

	"github.com/inoxlang/inox/internal/default_state"
	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/containers"
	"github.com/inoxlang/inox/internal/globals/env_ns"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/help_ns"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/inoxlsp_ns"
	"github.com/inoxlang/inox/internal/globals/strmanip_ns"

	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/globals/inoxsh_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/globals/s3_ns"

	_ "github.com/inoxlang/inox/internal/local_db"
	_ "github.com/inoxlang/inox/internal/obs_db"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

var (
	DEFAULT_SCRIPT_LIMITS = []core.Limit{
		{Name: fs_ns.FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},
		{Name: fs_ns.FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},

		{Name: fs_ns.FS_NEW_FILE_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 100},
		{Name: fs_ns.FS_TOTAL_NEW_FILE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 10_000},

		{Name: net_ns.HTTP_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 100},
		{Name: net_ns.WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 10},
		{Name: net_ns.TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 10},

		{Name: s3_ns.OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 50},

		{Name: core.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 5},
	}

	_ = []core.GoValue{
		(*html_ns.HTMLNode)(nil), (*core.GoFunction)(nil), (*http_ns.HttpServer)(nil), (*net_ns.TcpConn)(nil),
		(*net_ns.WebsocketConnection)(nil), (*http_ns.HttpRequest)(nil), (*http_ns.HttpResponseWriter)(nil),
		(*fs_ns.File)(nil),
	}
)

func init() {
	//set initial working directory on unix, on WASM it's done by the main package
	targetSpecificInit()
	registerHelp()

	inoxsh_ns.SetNewDefaultGlobalState(func(ctx *core.Context, envPattern *core.ObjectPattern, out io.Writer) *core.GlobalState {
		return utils.Must(NewDefaultGlobalState(ctx, default_state.DefaultGlobalStateConfig{
			EnvPattern: envPattern,
			Out:        out,
		}))
	})

	default_state.SetNewDefaultGlobalStateFn(NewDefaultGlobalState)
	default_state.SetNewDefaultContext(NewDefaultContext)
	default_state.SetDefaultScriptLimits(DEFAULT_SCRIPT_LIMITS)
}

// NewDefaultGlobalState creates a new GlobalState with the default globals.
func NewDefaultGlobalState(ctx *core.Context, conf default_state.DefaultGlobalStateConfig) (*core.GlobalState, error) {
	logOut := conf.LogOut
	var logger zerolog.Logger
	if logOut == nil { //if there is not writer for logs we log to conf.Out
		logOut = conf.Out

		consoleLogger := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = logOut
			w.NoColor = !config.SHOULD_COLORIZE
			w.TimeFormat = "15:04:05"
			w.FieldsExclude = []string{"src"}
		})
		logger = zerolog.New(consoleLogger)
	} else {
		logger = zerolog.New(logOut)
	}

	logger = logger.With().Timestamp().Logger().Level(zerolog.InfoLevel)

	//create env namespace

	envNamespace, err := env_ns.NewEnvNamespace(ctx, conf.EnvPattern, conf.AllowMissingEnvVars)
	if err != nil {
		return nil, err
	}

	//create value for the preinit-data global
	var preinitFilesKeys []string
	var preinitDataValues []core.Serializable
	for _, preinitFile := range conf.PreinitFiles {
		preinitFilesKeys = append(preinitFilesKeys, preinitFile.Name)
		preinitDataValues = append(preinitDataValues, preinitFile.Parsed)
	}

	preinitData :=
		core.NewRecordFromKeyValLists([]string{"files"}, []core.Serializable{core.NewRecordFromKeyValLists(preinitFilesKeys, preinitDataValues)})

	//
	constants := map[string]core.Value{
		// constants
		core.INITIAL_WORKING_DIR_VARNAME:        core.INITIAL_WORKING_DIR_PATH,
		core.INITIAL_WORKING_DIR_PREFIX_VARNAME: core.INITIAL_WORKING_DIR_PATH_PATTERN,

		// namespaces
		"fs":       fs_ns.NewFsNamespace(),
		"http":     http_ns.NewHttpNamespace(),
		"tcp":      net_ns.NewTcpNamespace(),
		"dns":      net_ns.NewDNSnamespace(),
		"ws":       net_ns.NewWebsocketNamespace(),
		"s3":       s3_ns.NewS3namespace(),
		"chrome":   chrome_ns.NewChromeNamespace(),
		"env":      envNamespace,
		"html":     html_ns.NewHTMLNamespace(),
		"inox":     inox_ns.NewInoxNamespace(),
		"inoxsh":   inoxsh_ns.NewInoxshNamespace(),
		"inoxlsp":  inoxlsp_ns.NewInoxLspNamespace(),
		"strmanip": strmanip_ns.NewStrManipNnamespace(),
		"rsa":      newRSANamespace(),

		"ls": core.WrapGoFunction(fs_ns.ListFiles),

		// transaction
		"get_current_tx": core.ValOf(_get_current_tx),
		"start_tx":       core.ValOf(core.StartNewTransaction),

		"Error": core.ValOf(_Error),

		// resource
		"read": core.ValOf(_readResource),
		//"get":    core.ValOf(_getResource),
		"create": core.ValOf(_createResource),
		"update": core.ValOf(_updateResource),
		"delete": core.ValOf(_deleteResource),

		"serve": core.ValOf(_serve),

		// events
		"Event":       core.ValOf(_Event),
		"EventSource": core.ValOf(core.NewEventSource),

		// watch
		"watch_received_messages": core.ValOf(core.WatchReceivedMessages),
		"ValueHistory":            core.WrapGoFunction(core.NewValueHistory),
		"dynif":                   core.WrapGoFunction(core.NewDynamicIf),
		"dyncall":                 core.WrapGoFunction(core.NewDynamicCall),
		"get_system_graph":        core.WrapGoFunction(_get_system_graph),

		// send & receive values
		"sendval": core.ValOf(core.SendVal),

		// crypto
		"insecure":       newInsecure(),
		"sha256":         core.ValOf(_sha256),
		"sha384":         core.ValOf(_sha384),
		"sha512":         core.ValOf(_sha512),
		"hash_password":  core.ValOf(_hashPassword),
		"check_password": core.ValOf(_checkPassword),
		"rand":           core.ValOf(_rand),

		//encodings
		"b64":  core.ValOf(encodeBase64),
		"db64": core.ValOf(decodeBase64),

		"hex":   core.ValOf(encodeHex),
		"unhex": core.ValOf(decodeHex),

		// conversion
		"tostr":      core.ValOf(_tostr),
		"torune":     core.ValOf(_torune),
		"tobyte":     core.ValOf(_tobyte),
		"tofloat":    core.ValOf(_tofloat),
		"toint":      core.ValOf(_toint),
		"torstream":  core.ValOf(_torstream),
		"tojson":     core.ValOf(core.ToJSON),
		"topjson":    core.ValOf(core.ToPrettyJSON),
		"repr":       core.ValOf(_repr),
		"parse_repr": core.ValOf(_parse_repr),
		"parse":      core.ValOf(_parse),
		"split":      core.ValOf(_split),

		// time
		"ago":   core.ValOf(_ago),
		"now":   core.ValOf(_now),
		"sleep": core.ValOf(core.Sleep),

		// printing
		"logvals":       core.ValOf(_logvals),
		"log":           core.ValOf(_log),
		"print":         core.ValOf(_print),
		"printvals":     core.ValOf(_printvals),
		"fprint":        core.ValOf(_fprint),
		"stringify_ast": core.ValOf(_stringify_ast),
		"fmt":           core.ValOf(core.Fmt),

		// bytes & string
		"mkbytes":       core.ValOf(_mkbytes),
		"Runes":         core.ValOf(_Runes),
		"Bytes":         core.ValOf(_Bytes),
		"is_rune_space": core.ValOf(_is_rune_space),
		"Reader":        core.ValOf(_Reader),
		"RingBuffer":    core.ValOf(core.NewRingBuffer),

		// functional
		"idt":     core.WrapGoFunction(_idt),
		"map":     core.WrapGoFunction(core.Map),
		"filter":  core.WrapGoFunction(core.Filter),
		"some":    core.WrapGoFunction(core.Some),
		"all":     core.WrapGoFunction(core.All),
		"none":    core.WrapGoFunction(core.None),
		"replace": core.WrapGoFunction(_replace),
		"find":    core.WrapGoFunction(_find),
		"sort":    core.WrapGoFunction(core.Sort),

		// concurrency & execution
		"LThreadGroup": core.ValOf(core.NewLThreadGroup),
		"run":          core.ValOf(_run),
		"ex":           core.ValOf(_execute),
		"cancel_exec":  core.ValOf(_cancel_exec),

		// integer
		"is_even": core.ValOf(_is_even),
		"is_odd":  core.ValOf(_is_odd),

		// protocol
		"set_client_for_url":  core.ValOf(setClientForURL),
		"set_client_for_host": core.ValOf(setClientForHost),

		// other functions
		"add_ctx_data": core.ValOf(_add_ctx_data),
		"ctx_data":     core.ValOf(_ctx_data),
		"propnames":    core.WrapGoFunction(_propnames),

		"Array": core.ValOf(core.NewArray),
		"List":  core.ValOf(_List),

		"typeof":    core.ValOf(_typeof),
		"url_of":    core.ValOf(_url_of),
		"len":       core.ValOf(_len),
		"len_range": core.ValOf(_len_range),

		"sum_options": core.ValOf(core.SumOptions),
		"mime":        core.ValOf(http_ns.Mime_),

		"Color":                           core.WrapGoFunction(_Color),
		core.FILEMODE_PRIMORDIAL_FUNCNAME: core.WrapGoFunction(core.FileModeFrom),

		"help": core.ValOf(help_ns.Help),
	}

	for k, v := range containers.NewContainersNamespace() {
		constants[k] = v
	}

	if conf.AbsoluteModulePath != "" {
		constants[default_state.MODULE_DIRPATH_GLOBAL_NAME] = core.DirPathFrom(filepath.Dir(conf.AbsoluteModulePath))
		constants[default_state.MODULE_FILEPATH_GLOBAL_NAME] = core.PathFrom(conf.AbsoluteModulePath)
	}

	baseGlobals := maps.Clone(constants)
	constants[default_state.PREINIT_DATA_GLOBAL_NAME] = preinitData

	symbolicBaseGlobals := map[string]symbolic.SymbolicValue{}
	{
		encountered := map[uintptr]symbolic.SymbolicValue{}
		for k, v := range baseGlobals {
			symbolicValue, err := v.ToSymbolicValue(ctx, encountered)
			if err != nil {
				return nil, fmt.Errorf("failed to convert base global '%s' to symbolic: %w", k, err)
			}
			symbolicBaseGlobals[k] = symbolicValue
		}
	}

	state := core.NewGlobalState(ctx, constants)
	state.Out = conf.Out
	state.Logger = logger
	state.GetBaseGlobalsForImportedModule = func(ctx *core.Context, manifest *core.Manifest) (core.GlobalVariables, error) {
		importedModuleGlobals := maps.Clone(baseGlobals)
		env, err := env_ns.NewEnvNamespace(ctx, nil, conf.AllowMissingEnvVars)
		if err != nil {
			return core.GlobalVariables{}, err
		}

		importedModuleGlobals["env"] = env
		baseGlobalKeys := maps.Keys(importedModuleGlobals)
		return core.GlobalVariablesFromMap(importedModuleGlobals, baseGlobalKeys), nil
	}
	state.GetBasePatternsForImportedModule = func() (map[string]core.Pattern, map[string]*core.PatternNamespace) {
		return maps.Clone(core.DEFAULT_NAMED_PATTERNS), maps.Clone(core.DEFAULT_PATTERN_NAMESPACES)
	}
	state.SymbolicBaseGlobalsForImportedModule = symbolicBaseGlobals

	return state, nil
}

// NewDefaultState creates a new Context with the default patterns.
func NewDefaultContext(config default_state.DefaultContextConfig) (*core.Context, error) {

	ctxConfig := core.ContextConfig{
		Permissions:          config.Permissions,
		ForbiddenPermissions: config.ForbiddenPermissions,
		Limits:               config.Limits,
		HostResolutions:      config.HostResolutions,
		ParentContext:        config.ParentContext,
		Filesystem:           config.Filesystem,
		OwnedDatabases:       config.OwnedDatabases,
	}

	if ctxConfig.Filesystem == nil && ctxConfig.ParentContext == nil {
		ctxConfig.Filesystem = fs_ns.GetOsFilesystem()
	}

	if ctxConfig.ParentContext != nil {
		if err, _ := ctxConfig.Check(); err != nil {
			return nil, err
		}
	}

	ctx := core.NewContext(ctxConfig)

	for k, v := range core.DEFAULT_NAMED_PATTERNS {
		ctx.AddNamedPattern(k, v)
	}

	for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
		ctx.AddPatternNamespace(k, v)
	}

	return ctx, nil
}
