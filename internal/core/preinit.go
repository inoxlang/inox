package core

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_PREINIT_FILE_SIZE      = int32(100_000)
	DEFAULT_MAX_READ_FILE_SIZE = int32(100_000_000)
)

var (
	USABLE_GLOBALS_IN_PREINIT       = []string{globalnames.READ_FN}
	ErrFileSizeExceedSpecifiedLimit = errors.New("file's size exceeds the specified limit")
)

type PreinitArgs struct {
	GlobalConsts     *parse.GlobalConstantDeclarations //only used if no running state
	PreinitStatement *parse.PreinitStatement           //only used if no running state

	RunningState *TreeWalkState //optional
	ParentState  *GlobalState   //optional
	Filesystem   afs.Filesystem

	//if RunningState is nil .PreinitFilesystem is used to create the temporary context.
	PreinitFilesystem afs.Filesystem

	DefaultLimits         []Limit
	AddDefaultPermissions bool
	HandleCustomType      CustomPermissionTypeHandler //optional
	IgnoreUnknownSections bool
	IgnoreConstDeclErrors bool

	//used if .RunningState is nil
	AdditionalGlobals map[string]Value

	Project Project //optional
}

// PreInit performs the pre-initialization of the module:
// 1)  the pre-init block is statically checked (if present).
// 2)  the manifest's object literal is statically checked.
// 3)  if .RunningState is not nil go to 10)
// 4)  else (.RunningState is nil) a temporary context & state are created.
// 5)  pre-evaluate the env section of the manifest.
// 6)  pre-evaluate the preinit-files section of the manifest.
// 7)  read & parse the preinit-files using the provided .PreinitFilesystem.
// 8)  evaluate & define the global constants (const ....).
// 9)  evaluate the preinit block.
// 10) evaluate the manifest's object literal.
// 11) create the manifest.
//
// If an error occurs at any step, the function returns.
func (m *Module) PreInit(preinitArgs PreinitArgs) (_ *Manifest, usedRunningState *TreeWalkState, _ []*StaticCheckError, preinitErr error) {
	defer func() {
		if preinitErr != nil && m.ManifestTemplate != nil {
			preinitErr = LocatedEvalError{
				error:    preinitErr,
				Message:  preinitErr.Error(),
				Location: parse.SourcePositionStack{m.MainChunk.GetSourcePosition(m.ManifestTemplate.Span)},
			}
		}
	}()

	initialWorkingDirectory := DEFAULT_IWD
	if _, ok := preinitArgs.Filesystem.(afs.OsFS); ok {
		wd, err := os.Getwd()
		if err != nil {
			preinitErr = err
			return
		}
		initialWorkingDirectory = DirPathFrom(wd)
	}

	if m.ManifestTemplate == nil {
		manifest := NewEmptyManifest()
		if preinitArgs.AddDefaultPermissions {
			manifest.RequiredPermissions = append(manifest.RequiredPermissions, GetDefaultGlobalVarPermissions()...)
		}
		manifest.InitialWorkingDirectory = initialWorkingDirectory
		return manifest, nil, nil, nil
	}

	manifestObjLiteral, ok := m.ManifestTemplate.Object.(*parse.ObjectLiteral)
	if !ok {
		return &Manifest{InitialWorkingDirectory: initialWorkingDirectory}, nil, nil, nil
	}

	if parse.HasErrorAtAnyDepth(manifestObjLiteral) ||
		(preinitArgs.PreinitStatement != nil && parse.HasErrorAtAnyDepth(preinitArgs.PreinitStatement)) {
		return nil, nil, nil, inoxmod.ErrParsingErrorInManifestOrPreinit
	}

	//check preinit block
	if preinitArgs.PreinitStatement != nil {
		var checkErrs []*StaticCheckError
		checkPreinitBlock(preinitBlockCheckParams{
			node:   preinitArgs.PreinitStatement,
			fls:    preinitArgs.PreinitFilesystem,
			module: m,

			onError: func(n parse.Node, msg string) {
				location := m.MainChunk.GetSourcePosition(n.Base().Span)
				checkErr := NewStaticCheckError(msg, parse.SourcePositionStack{location})
				checkErrs = append(checkErrs, checkErr)
			},
		})
		if len(checkErrs) != 0 {
			return nil, nil, checkErrs, fmt.Errorf("%s: error while checking preinit block: %w", m.Name(), combineStaticCheckErrors(checkErrs...))
		}
	}

	// check manifest
	{
		var checkErrs []*StaticCheckError
		checkManifestObject(manifestStaticCheckArguments{
			objLit:                manifestObjLiteral,
			ignoreUnknownSections: preinitArgs.IgnoreUnknownSections,
			moduleKind:            m.Kind,
			onError: func(n parse.Node, msg string) {
				location := m.MainChunk.GetSourcePosition(n.Base().Span)
				checkErr := NewStaticCheckError(msg, parse.SourcePositionStack{location})
				checkErrs = append(checkErrs, checkErr)
			},
			project: preinitArgs.Project,
		})
		if len(checkErrs) != 0 {
			return nil, nil, checkErrs, fmt.Errorf("%s: error while checking manifest's object literal: %w", m.Name(), combineStaticCheckErrors(checkErrs...))
		}
	}

	var state *TreeWalkState
	var envPattern *ObjectPattern
	preinitFiles := make(PreinitFiles, 0)

	//we create a temporary state to pre-evaluate some parts of the manifest
	if preinitArgs.RunningState == nil {

		ctxPerms := []Permission{GlobalVarPermission{permbase.Read, "*"}}

		//Allow calling some builtins.
		for _, name := range USABLE_GLOBALS_IN_PREINIT {
			ctxPerms = append(ctxPerms, GlobalVarPermission{permbase.Use, name})
		}

		//Allow using additional globals and user-defined global constants (direct call or method call).
		if preinitArgs.GlobalConsts != nil {
			for _, decl := range preinitArgs.GlobalConsts.Declarations {
				ident, ok := decl.Left.(*parse.IdentifierLiteral)
				if ok {
					ctxPerms = append(ctxPerms, GlobalVarPermission{permbase.Use, ident.Name})
				}
			}
		}

		for k := range preinitArgs.AdditionalGlobals {
			ctxPerms = append(ctxPerms, GlobalVarPermission{permbase.Use, k})
		}

		ctx := NewContext(ContextConfig{
			Permissions:               ctxPerms,
			Filesystem:                preinitArgs.PreinitFilesystem,
			DoNotSetFilesystemContext: true,
			DoNotSpawnDoneGoroutine:   true,
		})
		defer ctx.CancelGracefully()

		for k, v := range DEFAULT_NAMED_PATTERNS {
			ctx.AddNamedPattern(k, v)
		}

		for k, v := range DEFAULT_PATTERN_NAMESPACES {
			ctx.AddPatternNamespace(k, v)
		}

		global := NewGlobalState(ctx, map[string]Value{
			globalnames.INITIAL_WORKING_DIR_VARNAME:        initialWorkingDirectory,
			globalnames.INITIAL_WORKING_DIR_PREFIX_VARNAME: initialWorkingDirectory.ToPrefixPattern(),
		})
		global.OutputFieldsInitialized.Store(true)
		global.Module = m
		state = NewTreeWalkStateWithGlobal(global)

		//Pre-evaluate the env section of the manifest.
		envSection, ok := manifestObjLiteral.PropValue(inoxconsts.MANIFEST_ENV_SECTION_NAME)
		if ok {
			v, err := TreeWalkEval(envSection, state)
			if err != nil {
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%s: failed to pre-evaluate the %s section: %w", m.Name(), inoxconsts.MANIFEST_ENV_SECTION_NAME, err)
				}
			}
			envPattern = v.(*ObjectPattern)
		}

		//Declare additional globals. This is done before evaluating global constant declarations
		//because additional globals may be used in those declarations.
		for k, v := range preinitArgs.AdditionalGlobals {
			state.SetGlobal(k, v, GlobalConst)
		}

		//Evaluate & declare the global constants.
		if preinitArgs.GlobalConsts != nil {
			for _, decl := range preinitArgs.GlobalConsts.Declarations {
				//ignore declaration if incomplete
				if preinitArgs.IgnoreConstDeclErrors && decl.Left == nil || decl.Right == nil || utils.Implements[*parse.MissingExpression](decl.Right) {
					continue
				}

				constVal, err := TreeWalkEval(decl.Right, state)
				if err != nil {
					if !preinitArgs.IgnoreConstDeclErrors {
						return nil, nil, nil, fmt.Errorf(
							"%s: failed to evaluate manifest object: error while evaluating constant declarations: %w", m.Name(), err)
					}
				} else {
					state.SetGlobal(decl.Ident().Name, constVal, GlobalConst)
				}
			}
		}

		//evalute preinit block
		if preinitArgs.PreinitStatement != nil {
			_, err := TreeWalkEval(preinitArgs.PreinitStatement.Block, state)
			if err != nil {
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%s: failed to evaluate preinit block: %w", m.Name(), err)
				}
			}
		}

		// pre evaluate the preinit-files section of the manifest
		preinitFilesSection, ok := manifestObjLiteral.PropValue(inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME)
		if ok {
			v, err := TreeWalkEval(preinitFilesSection, state)
			if err != nil {
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%s: failed to pre-evaluate the %s section: %w", m.Name(), inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME, err)
				}
			}

			obj := v.(*Object)

			err = obj.ForEachEntry(func(k string, v Serializable) error {
				desc := v.(*Object)
				propNames := desc.PropertyNames(ctx)

				if !utils.SliceContains(propNames, inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME) {
					return fmt.Errorf("missing .%s property in description of preinit file %s", inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME, k)
				}

				if !utils.SliceContains(propNames, inoxconsts.MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME) {
					return fmt.Errorf("missing .%s property in description of preinit file %s", inoxconsts.MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME, k)
				}

				path, ok := desc.Prop(ctx, inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME).(Path)
				if !ok {
					return fmt.Errorf("property .%s in description of preinit file %s is not a path", inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME, k)
				}
				pattern, ok := desc.Prop(ctx, inoxconsts.MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME).(Pattern)
				if !ok {
					return fmt.Errorf("property .%s in description of preinit file %s is not a pattern", inoxconsts.MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME, k)
				}

				if !path.IsAbsolute() {
					return fmt.Errorf("property .%s in description of preinit file %s should be an absolute path", inoxconsts.MANIFEST_PREINIT_FILE__PATH_PROP_NAME, k)
				}

				switch patt := pattern.(type) {
				case StringPattern:
				case *SecretPattern:
				case *TypePattern:
					if patt != STR_PATTERN {
						return fmt.Errorf("invalid pattern type %T for preinit file '%s'", patt, k)
					}
				default:
					return fmt.Errorf("invalid pattern type %T for preinit file '%s'", patt, k)
				}

				preinitFiles = append(preinitFiles, &PreinitFile{
					Name:    k,
					Path:    path,
					Pattern: pattern,
					RequiredPermission: FilesystemPermission{
						Kind_:  permbase.Read,
						Entity: path,
					},
				})

				return nil
			})

			if err != nil {
				return nil, nil, nil, fmt.Errorf("%s: failed to pre-evaluate the %s section: %w", m.Name(), inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME, err)
			}

			//read & parse preinit files
			atLeastOneReadParseError := false
			for _, file := range preinitFiles {
				content, err := ReadFileInFS(preinitArgs.PreinitFilesystem, string(file.Path), MAX_PREINIT_FILE_SIZE)
				file.Content = content
				file.ReadParseError = err

				if err != nil {
					atLeastOneReadParseError = true
					continue
				}

				switch patt := file.Pattern.(type) {
				case StringPattern:
					file.Parsed, file.ReadParseError = patt.Parse(ctx, string(content))
				case *SecretPattern:
					file.Parsed, file.ReadParseError = patt.NewSecret(ctx, string(content))
				case *TypePattern:
					if patt != STR_PATTERN {
						panic(ErrUnreachable)
					}
					file.Parsed = String(content)
				default:
					panic(ErrUnreachable)
				}

				if file.ReadParseError != nil {
					atLeastOneReadParseError = true
				}
			}

			if atLeastOneReadParseError {
				//not very explicative on purpose.
				return nil, nil, nil, fmt.Errorf("%s: at least one error when reading & parsing preinit files", m.Name())
			}
		}

	} else {
		if preinitArgs.GlobalConsts != nil {
			return nil, nil, nil, fmt.Errorf(".GlobalConstants argument should not have been passed")
		}

		if preinitArgs.PreinitStatement != nil {
			return nil, nil, nil, fmt.Errorf(".Preinit argument should not have been passed")
		}

		state = preinitArgs.RunningState
	}

	// evaluate object literal
	v, err := TreeWalkEval(m.ManifestTemplate.Object, state)
	if err != nil {
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: failed to evaluate manifest object: %w", m.Name(), err)
		}
	}

	manifestObj := v.(*Object)

	manifest, err := m.createManifest(state.Global.Ctx, manifestObj, manifestObjectConfig{
		parentState:           preinitArgs.ParentState,
		defaultLimits:         preinitArgs.DefaultLimits,
		handleCustomType:      preinitArgs.HandleCustomType,
		addDefaultPermissions: preinitArgs.AddDefaultPermissions,
		envPattern:            envPattern,
		preinitFileConfigs:    preinitFiles,
		//addDefaultPermissions: true,
		ignoreUnkownSections:    preinitArgs.IgnoreUnknownSections,
		initialWorkingDirectory: initialWorkingDirectory,
	})

	return manifest, state, nil, err
}

// ReadFileInFS reads up to maxSize bytes from a file in the given filesystem.
// if maxSize is <=0 the max size is set to 100MB.
func ReadFileInFS(fls billy.Basic, name string, maxSize int32) ([]byte, error) {
	f, err := fls.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	if maxSize <= 0 {
		maxSize = DEFAULT_MAX_READ_FILE_SIZE
	}

	var size32 int32
	if info, err := afs.FileStat(f, fls); err == nil {
		size64 := info.Size()
		if size64 > int64(maxSize) || size64 >= math.MaxInt32 {
			return nil, ErrFileSizeExceedSpecifiedLimit
		}

		size32 = int32(size64)
	}

	size32++ // one byte for final read at EOF
	// If a file claims a small size, read at least 512 bytes.
	// In particular, files in Linux's /proc claim size 0 but
	// then do not work right if read in small pieces,
	// so an initial read of 1 byte would not work correctly.

	if size32 < 512 {
		size32 = 512
	}

	data := make([]byte, 0, size32)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}

		n, err := f.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]

		if len(data) > int(maxSize) {
			return nil, ErrFileSizeExceedSpecifiedLimit
		}

		if err != nil {
			if err == io.EOF {
				err = nil
			}

			return data, err
		}
	}
}
