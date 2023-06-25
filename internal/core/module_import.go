package core

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"path/filepath"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	INOX_MIMETYPE          = "application/inox"
	DEFAULT_FETCH_TIMEOUT  = 10 * time.Second
	DEFAULT_IMPORT_TIMEOUT = 10 * time.Second
)

var (
	ErrInvalidModuleSourceURL                          = errors.New("invalid module source URL")
	ErrAbsoluteModuleSourcePathUsedInURLImportedModule = errors.New("absolute module source path used in module imported from URL")
)

var moduleCache = map[string]string{}
var moduleCacheLock sync.Mutex

// ImportWaitModule imports a module and waits for its routine to return its result.
func ImportWaitModule(config ImportConfig) (Value, error) {
	routine, err := ImportModule(config)
	if err != nil {
		return nil, err
	}
	//TODO: add timeout
	result, err := routine.WaitResult(config.ParentState.Ctx)
	if err != nil {
		return nil, fmt.Errorf("import: failed: %s", err.Error())
	}

	return result, nil
}

type ImportConfig struct {
	Src                Value
	ValidationString   Str     //hash of the imported module
	ArgObj             *Object //arguments for the evaluation of the imported module
	GrantedPermListing *Object
	ParentState        *GlobalState  //the state of the module doing the import
	Insecure           bool          //if true certificate verification is ignored when making HTTP requests
	Timeout            time.Duration //total timeout for combined fetching + evaluation of the imported module
}

func buildImportConfig(obj *Object, source Value, parentState *GlobalState) (ImportConfig, error) {
	config := ImportConfig{
		Src:         source,
		ParentState: parentState,
	}

	for k, v := range obj.EntryMap() {
		switch k {
		case "validation":
			config.ValidationString = v.(Str)
		case "arguments":
			config.ArgObj = v.(*Object)
		case "allow":
			config.GrantedPermListing = v.(*Object)
		default:
			return ImportConfig{}, fmt.Errorf("invalid import configuration, unknown section '%s'", k)
		}
	}

	return config, nil
}

// ImportModule imports a module and returned a spawned routine running the module.
func ImportModule(config ImportConfig) (*Routine, error) {

	var srcVal WrappedString
	var absScriptDir string

	//if src is a relative path and the importing module has been itself imported from an URL we make an URL with the right path.
	switch val := config.Src.(type) {
	case Path:
		if config.ParentState.Module != nil && config.ParentState.Module.HasURLSource() {
			if val.IsAbsolute() {
				return nil, ErrAbsoluteModuleSourcePathUsedInURLImportedModule
			}
			config.Src = URL(config.ParentState.Module.ResourceDir()).AppendRelativePath(val)
		}
	}

	switch val := config.Src.(type) {
	case URL:
		httpPerm := HttpPermission{permkind.Read, val}
		if err := config.ParentState.Ctx.CheckHasPermission(httpPerm); err != nil {
			return nil, fmt.Errorf("import: %s", err.Error())
		}
		srcVal = val
	case Path:
		fls := config.ParentState.Ctx.GetFileSystem()
		if val.IsRelative() && config.ParentState.Module != nil {
			val = Path(fls.Join(config.ParentState.Module.ResourceDir(), val.UnderlyingString()))
		}

		absScriptDir = filepath.Dir(string(val))
		fsPerm := FilesystemPermission{permkind.Read, val.ToAbs(fls)}
		if err := config.ParentState.Ctx.CheckHasPermission(fsPerm); err != nil {
			return nil, fmt.Errorf("import: %s", err.Error())
		}
		srcVal = val
	default:
		return nil, fmt.Errorf("import: invalid source, type is %T", val)
	}

	if !strings.HasSuffix(srcVal.UnderlyingString(), ".ix") {
		return nil, errors.New(symbolic.IMPORTED_MOD_PATH_MUST_END_WITH_IX)
	}

	grantedPerms, err := getPermissionsFromListing(config.GrantedPermListing, nil, nil, true)
	if err != nil {
		return nil, err
	}
	forbiddenPerms := config.ParentState.Ctx.forbiddenPermissions

	for _, perm := range grantedPerms {
		if err := config.ParentState.Ctx.CheckHasPermission(perm); err != nil {
			return nil, fmt.Errorf("import: cannot allow permission: %w", err)
		}
	}

	fetchStartTime := time.Now()

	mod, err := fetchParseImportedModule(config.ParentState.Ctx, srcVal, sourceFileDownloadConfig{
		validation: string(config.ValidationString),
		insecure:   config.Insecure,
		timeout:    config.Timeout,
	})

	var timeout time.Duration = 0
	if config.Timeout != 0 {
		timeout = config.Timeout - time.Since(fetchStartTime)
	}

	if err != nil {
		return nil, fmt.Errorf("import: cannot fetch module: %s", err.Error())
	}

	manifest, preinitState, _, err := mod.PreInit(PreinitArgs{
		GlobalConsts:          mod.MainChunk.Node.GlobalConstantDeclarations,
		PreinitStatement:      mod.MainChunk.Node.Preinit,
		AddDefaultPermissions: true,
	})

	if err != nil {
		return nil, fmt.Errorf("import: manifest: %s", err.Error())
	}

	if ok, missingPerms := manifest.ArePermsGranted(grantedPerms, forbiddenPerms); !ok {
		list := utils.MapSlice(missingPerms, func(p Permission) string {
			return p.String()
		})
		return nil, fmt.Errorf("import: some permissions in the imported module's manifest are not granted: %s", strings.Join(list, "\n"))
	}

	routineCtx := NewContext(ContextConfig{
		Permissions:          grantedPerms,
		ForbiddenPermissions: forbiddenPerms,
		ParentContext:        config.ParentState.Ctx,
	})

	// add base patterns
	var basePatterns map[string]Pattern
	var basePatternNamespaces map[string]*PatternNamespace
	if config.ParentState.GetBasePatternsForImportedModule != nil {
		basePatterns, basePatternNamespaces = config.ParentState.GetBasePatternsForImportedModule()

		for name, patt := range basePatterns {
			routineCtx.AddNamedPattern(name, patt)
		}
		for name, ns := range basePatternNamespaces {
			routineCtx.AddPatternNamespace(name, ns)
		}
	}

	// add base globals
	var globals GlobalVariables
	if config.ParentState.GetBaseGlobalsForImportedModule != nil {
		baseGlobals, err := config.ParentState.GetBaseGlobalsForImportedModule(routineCtx, manifest)
		if err != nil {
			return nil, err
		}
		globals = baseGlobals
	} else {
		globals = GlobalVariablesFromMap(map[string]Value{}, nil)
	}

	// pass patterns & host aliases of the preinit state to the context
	if preinitState != nil {
		for name, patt := range preinitState.Global.Ctx.GetNamedPatterns() {
			if _, ok := basePatterns[name]; ok {
				continue
			}
			routineCtx.AddNamedPattern(name, patt)
		}
		for name, ns := range preinitState.Global.Ctx.GetPatternNamespaces() {
			if _, ok := basePatternNamespaces[name]; ok {
				continue
			}
			routineCtx.AddPatternNamespace(name, ns)
		}
		for name, val := range preinitState.Global.Ctx.GetHostAliases() {
			routineCtx.AddHostAlias(name, val)
		}
	}

	if config.ArgObj != nil {
		args, err := manifest.Parameters.GetArguments(routineCtx, config.ArgObj)
		if err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		globals.Set(MOD_ARGS_VARNAME, args)
	} else {
		globals.Set(MOD_ARGS_VARNAME, Nil)
	}

	_ = absScriptDir
	routine, err := SpawnRoutine(RoutineSpawnArgs{
		SpawnerState: config.ParentState,
		Globals:      globals,
		Module:       mod,
		RoutineCtx:   routineCtx,
		//AbsScriptDir: absScriptDir,
		//bytecode: //TODO
		Timeout:                      timeout,
		IgnoreCreateRoutinePermCheck: true,
	})
	if err != nil {
		return nil, fmt.Errorf("import: %s", err.Error())
	}

	return routine, nil
}

type sourceFileDownloadConfig struct {
	validation string
	insecure   bool
	timeout    time.Duration
}

// fetchParseImportedModule first fetches a module by reading the filesystem or making an HTTP request, then it parses it.
func fetchParseImportedModule(ctx *Context, src WrappedString, config sourceFileDownloadConfig) (*Module, error) {

	timeout := config.timeout
	if timeout == 0 {
		timeout = DEFAULT_FETCH_TIMEOUT
	}

	//equal to  http.DefaultTransport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: config.insecure}
	client := http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	var b []byte
	var modString string
	var ok bool
	var absScriptDir string
	var isResourceURL bool
	fls := ctx.GetFileSystem()

	moduleCacheLock.Lock()
	unlock := true
	defer func() {
		if unlock {
			moduleCacheLock.Unlock()
		}
	}()

	if modString, ok = moduleCache[config.validation]; !ok {
		switch srcVal := src.(type) {
		case Path:
			absSrc, err := fls.Absolute(string(srcVal))
			if err != nil {
				return nil, err
			}
			absScriptDir = filepath.Dir(absSrc)
			file, err := ctx.fs.Open(string(srcVal))
			if err != nil {
				return nil, err
			}
			content, err := io.ReadAll(file)
			if err != nil {
				return nil, err
			}
			b = content
		case URL:
			isResourceURL = true

			pth := srcVal.Path()
			if pth.IsDirPath() || pth.IsRelative() {
				return nil, ErrInvalidModuleSourceURL
			}

			{
				lastSlashIndex := strings.LastIndex(string(pth), "/")
				absScriptDir = string(srcVal.Host()) + string(pth)[:lastSlashIndex+1]
			}

			req, err := http.NewRequest("GET", string(srcVal), nil)
			req.Header.Add("Accept", INOX_MIMETYPE)

			if err != nil {
				return nil, err
			}

			resp, err := client.Do(req)

			if err != nil {
				return nil, err
			}

			//TODO: sanitize .Status, Content-Type, etc before writing them to the terminal
			var bodyErr error
			b, bodyErr = io.ReadAll(resp.Body)

			if resp.StatusCode != 200 {
				return nil, &ModuleRetrievalError{message: fmt.Sprintf("failed to get %s: status %d: %s", srcVal, resp.StatusCode, resp.Status)}
			}

			// ctype := resp.Header.Get("Content-Type")
			// if ctype != INOX_MIMETYPE {
			// 	return nil, fmt.Errorf("failed to get %s: content-type is '%s'", importURL, ctype)
			// }

			if bodyErr != nil {
				return nil, &ModuleRetrievalError{message: fmt.Sprintf("failed to get %s: failed to read body: %s", srcVal, err.Error())}
			}
		}

		array := sha256.Sum256(b)
		hash := array[:]

		if config.validation != "" && !bytes.Equal(hash, []byte(config.validation)) {
			return nil, &ModuleRetrievalError{message: fmt.Sprintf("failed to get %s: validation failed", src.UnderlyingString())}
		}

		modString = string(b)
		moduleCache[string(hash)] = modString

		//TODO: limit cache size
	}

	unlock = false
	moduleCacheLock.Unlock()

	source := parse.SourceFile{
		NameString:    src.UnderlyingString(),
		Resource:      src.UnderlyingString(),
		ResourceDir:   absScriptDir,
		IsResourceURL: isResourceURL,
		CodeString:    modString,
	}

	chunk, err := parse.ParseChunkSource(source)
	if err != nil {
		return nil, err
	}

	return &Module{
		MainChunk:        chunk,
		ManifestTemplate: chunk.Node.Manifest,
	}, nil
}

type ModuleRetrievalError struct {
	message string
}

func (err ModuleRetrievalError) Error() string {
	return err.message
}
