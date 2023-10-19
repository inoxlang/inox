package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"

	"path/filepath"
	"strings"
	"time"

	afs "github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/inoxconsts"
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

// ImportWaitModule imports a module and waits for its lthread to return its result.
// ImportWaitModule also adds the test suite results to the parent state.
func ImportWaitModule(config ImportConfig) (Value, error) {
	lthread, err := ImportModule(config)
	if err != nil {
		return nil, err
	}
	//TODO: add timeout
	result, err := lthread.WaitResult(config.ParentState.Ctx)
	if err != nil {
		return nil, fmt.Errorf("import: failed: %s", err.Error())
	}
	parentState := config.ParentState

	//add test suite results to the parent state.
	//we only try to lock to avoid blocking if already locked.
	if parentState.IsTestingEnabled && lthread.state.TestResultsLock.TryLock() {
		func() {
			defer lthread.state.TestResultsLock.Unlock()

			parentState.TestResultsLock.Lock()
			defer parentState.TestResultsLock.Unlock()

			parentState.TestSuiteResults = append(parentState.TestSuiteResults, lthread.state.TestSuiteResults...)
		}()
	}

	return result, nil
}

type ImportConfig struct {
	Src                ResourceName
	ValidationString   Str     //hash of the imported module
	ArgObj             *Object //arguments for the evaluation of the imported module
	GrantedPermListing *Object
	ParentState        *GlobalState  //the state of the module doing the import
	Insecure           bool          //if true certificate verification is ignored when making HTTP requests
	Timeout            time.Duration //total timeout for combined fetching + evaluation of the imported module
}

func buildImportConfig(obj *Object, importSource ResourceName, parentState *GlobalState) (ImportConfig, error) {
	src, err := getSourceFromImportSource(importSource, parentState.Module, parentState.Ctx)
	if err != nil {
		return ImportConfig{}, err
	}

	config := ImportConfig{
		Src:         src,
		ParentState: parentState,
	}

	for k, v := range obj.EntryMap(nil) {
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

// ImportModule imports a module and returned a spawned lthread running the module.
func ImportModule(config ImportConfig) (*LThread, error) {
	parentState := config.ParentState
	timeout := config.Timeout
	if timeout == 0 {
		timeout = DEFAULT_IMPORT_TIMEOUT
	}
	deadline := time.Now().Add(timeout)

	grantedPerms, err := getPermissionsFromListing(parentState.Ctx, config.GrantedPermListing, nil, nil, true)
	if err != nil {
		return nil, err
	}
	forbiddenPerms := parentState.Ctx.forbiddenPermissions

	for _, perm := range grantedPerms {
		if err := parentState.Ctx.CheckHasPermission(perm); err != nil {
			return nil, fmt.Errorf("import: cannot allow permission: %w", err)
		}
	}

	importedMod, ok := parentState.Module.DirectlyImportedModules[config.Src.ResourceName()]
	if !ok {
		panic(ErrUnreachable)
	}

	manifest, preinitState, _, err := importedMod.PreInit(PreinitArgs{
		ParentState:           parentState,
		GlobalConsts:          importedMod.MainChunk.Node.GlobalConstantDeclarations,
		PreinitStatement:      importedMod.MainChunk.Node.Preinit,
		AddDefaultPermissions: true,

		//TODO: should Project be set ?
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
	basePatterns, basePatternNamespaces = config.ParentState.GetBasePatternsForImportedModule()

	for name, patt := range basePatterns {
		routineCtx.AddNamedPattern(name, patt)
	}
	for name, ns := range basePatternNamespaces {
		routineCtx.AddPatternNamespace(name, ns)
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
		args, err := manifest.Parameters.GetArgumentsFromObject(routineCtx, config.ArgObj)
		if err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		globals.Set(MOD_ARGS_VARNAME, args)
	} else {
		globals.Set(MOD_ARGS_VARNAME, Nil)
	}

	lthread, err := SpawnLThread(LthreadSpawnArgs{
		SpawnerState: config.ParentState,
		Globals:      globals,
		Module:       importedMod,
		Manifest:     manifest,
		LthreadCtx:   routineCtx,
		//Bytecode: //TODO
		//AbsScriptDir: absScriptDir,
		Timeout:                      deadline.Sub(time.Now()),
		IgnoreCreateLThreadPermCheck: true,

		IsTestingEnabled: parentState.IsTestingEnabled && parentState.IsImportTestingEnabled,
		TestFilters:      parentState.TestFilters,
	})
	if err != nil {
		return nil, fmt.Errorf("import: %s", err.Error())
	}

	config.ParentState.SetDescendantState(config.Src, lthread.state)

	return lthread, nil
}

type importedModulesFetchConfig struct {
	recoverFromNonExistingFiles, ignoreBadlyConfiguredImports bool
	timeout                                                   time.Duration
	insecure                                                  bool
	subModuleParsing                                          ModuleParsingConfig
}

func fetchParseImportedModules(mod *Module, ctx *Context, fls afs.Filesystem, config importedModulesFetchConfig) (unrecoverableError error) {
	importStmts := parse.FindNodes(mod.MainChunk.Node, (*parse.ImportStatement)(nil), nil)

	stmtSources := map[WrappedString]*parse.ImportStatement{}
	validationStrings := map[WrappedString]string{}

	for _, importStmt := range importStmts {
		var src WrappedString

		switch importStmt.Source.(type) {
		case *parse.URLLiteral, *parse.AbsolutePathLiteral, *parse.RelativePathLiteral:
			value, err := evalSimpleValueLiteral(importStmt.Source.(parse.SimpleValueLiteral), nil)
			if err != nil {
				continue
			} else {
				src = value.(WrappedString)
			}
		}

		objLiteral, ok := importStmt.Configuration.(*parse.ObjectLiteral)
		if !ok {
			if !config.ignoreBadlyConfiguredImports {
				return errors.New("invalid module import: configuration should be an object")
			}
			continue
		}

		var validationString string = ""
		validationNode, ok := objLiteral.PropValue("validation")
		if ok {
			validationStringLit, ok := validationNode.(*parse.QuotedStringLiteral)
			if ok {
				validationString = validationStringLit.Value
			} else {
				if !config.ignoreBadlyConfiguredImports {
					return errors.New("invalid module import: <configuration>.validation should be a string")
				}
			}
		}

		stmtSources[src] = importStmt
		validationStrings[src] = validationString
	}

	var (
		wg                          = new(sync.WaitGroup)
		lock                        sync.Mutex
		importedModules             = map[string]*Module{}
		importedModulesByImportStmt = map[*parse.ImportStatement]*Module{}
		errors                      []error
		unrecoverableErors          []error
	)

	wg.Add(len(stmtSources))

	for src := range stmtSources {

		go func(src WrappedString, validationString string) {
			defer wg.Done()

			var importedMod *Module
			var err error

			defer func() {
				if e, ok := recover().(error); ok {
					lock.Lock()
					unrecoverableErors = append(unrecoverableErors, e)
					lock.Unlock()
				} else if e == nil {
					lock.Lock()

					if importedMod != nil {
						importedModules[importedMod.Name()] = importedMod
						importedModulesByImportStmt[stmtSources[src]] = importedMod
						if err != nil {
							errors = append(errors, err)
						}
					} else {
						unrecoverableErors = append(unrecoverableErors, err)
					}
					lock.Unlock()
				}
			}()

			importedMod, err = fetchParseImportedModule(ctx, src, sourceFileDownloadConfig{
				parentModule:     mod,
				validation:       validationString,
				insecure:         config.insecure,
				timeout:          config.timeout,
				subModuleParsing: config.subModuleParsing,
			})

		}(src, validationStrings[src])

	}

	wg.Wait()

	if len(unrecoverableErors) > 0 {
		return utils.CombineErrors(unrecoverableErors...)
	}

	mod.DirectlyImportedModules = importedModules
	mod.DirectlyImportedModulesByStatement = importedModulesByImportStmt

	for _, importedMod := range importedModules {
		mod.OriginalErrors = append(mod.OriginalErrors, importedMod.OriginalErrors...)
		mod.ParsingErrors = append(mod.ParsingErrors, importedMod.ParsingErrors...)
		mod.ParsingErrorPositions = append(mod.ParsingErrorPositions, importedMod.ParsingErrorPositions...)
	}

	return nil
}

func getSourceFromImportSource(importSrc Value, parentModule *Module, ctx *Context) (ResourceName, error) {
	//if src is a relative path and the importing module has been itself imported from an URL we make an URL with the right path.
	switch val := importSrc.(type) {
	case Path:
		if parentModule != nil && parentModule.HasURLSource() {
			if val.IsAbsolute() {
				return nil, ErrAbsoluteModuleSourcePathUsedInURLImportedModule
			}
			return URL(parentModule.ResourceDir()).AppendRelativePath(val), nil
		} else {
			fls := ctx.GetFileSystem()
			if val.IsRelative() {
				if parentModule != nil {
					val = Path(fls.Join(parentModule.ResourceDir(), val.UnderlyingString()))
				} else {
					return nil, fmt.Errorf("import: impossible to resolve relative import path as parent state has no module")
				}
			}

			fsPerm := FilesystemPermission{permkind.Read, utils.Must(val.ToAbs(fls))}
			if err := ctx.CheckHasPermission(fsPerm); err != nil {
				return nil, fmt.Errorf("import: %s", err.Error())
			}
			return val, nil
		}
	case URL:
		httpPerm := HttpPermission{permkind.Read, val}
		if err := ctx.CheckHasPermission(httpPerm); err != nil {
			return nil, fmt.Errorf("import: %s", err.Error())
		}
		return val, nil
	default:
		return nil, fmt.Errorf("import: invalid source, type is %T", val)
	}
}

type sourceFileDownloadConfig struct {
	parentModule *Module
	validation   string
	insecure     bool
	timeout      time.Duration

	subModuleParsing ModuleParsingConfig
}

// fetchParseImportedModule first fetches a module by reading the filesystem or making an HTTP request, then it parses it.
func fetchParseImportedModule(ctx *Context, importSrc WrappedString, config sourceFileDownloadConfig) (*Module, error) {
	parentModule := config.parentModule

	src, err := getSourceFromImportSource(importSrc, parentModule, ctx)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(src.UnderlyingString(), inoxconsts.INOXLANG_FILE_EXTENSION) {
		return nil, errors.New(symbolic.IMPORTED_MOD_PATH_MUST_END_WITH_IX)
	}

	timeout := config.timeout
	if timeout == 0 {
		timeout = DEFAULT_FETCH_TIMEOUT
	}

	deadline := time.Now().Add(timeout)

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 5 * time.Second,
			Deadline:  deadline.Add(-time.Second),
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1,
		IdleConnTimeout:       1 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
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

	if modString, ok = moduleCache[config.validation]; !ok || config.validation == "" {
		switch srcVal := src.(type) {
		case Path:
			absSrc, err := fls.Absolute(string(srcVal))
			if err != nil {
				return nil, err
			}
			absScriptDir = filepath.Dir(absSrc)
			file, err := ctx.fs.OpenFile(string(srcVal), os.O_RDONLY, 0)
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

			reqCtx, cancel := context.WithDeadline(ctx, deadline)
			defer cancel()
			req = req.WithContext(reqCtx)

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

	return ParseModuleFromSource(source, src, config.subModuleParsing)
}

type ModuleRetrievalError struct {
	message string
}

func (err ModuleRetrievalError) Error() string {
	return err.message
}
