package core

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"slices"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/text"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"golang.org/x/exp/maps"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	//the initial working dir is the working dir at the start of the program execution.
	INITIAL_WORKING_DIR_PATH         Path
	INITIAL_WORKING_DIR_PATH_PATTERN PathPattern

	MODULE_KIND_TO_ALLOWED_SECTION_NAMES = map[ModuleKind][]string{
		UnspecifiedModuleKind: inoxconsts.MANIFEST_SECTION_NAMES,
		ApplicationModule:     inoxconsts.MANIFEST_SECTION_NAMES,
		SpecModule:            {inoxconsts.MANIFEST_KIND_SECTION_NAME, inoxconsts.MANIFEST_PERMS_SECTION_NAME, inoxconsts.MANIFEST_LIMITS_SECTION_NAME},
		LifetimeJobModule:     {inoxconsts.MANIFEST_PERMS_SECTION_NAME, inoxconsts.MANIFEST_LIMITS_SECTION_NAME},
		TestSuiteModule:       {inoxconsts.MANIFEST_PERMS_SECTION_NAME, inoxconsts.MANIFEST_LIMITS_SECTION_NAME},
		TestCaseModule:        {inoxconsts.MANIFEST_PERMS_SECTION_NAME, inoxconsts.MANIFEST_LIMITS_SECTION_NAME},
	}

	ErrURLNotCorrespondingToDefinedDB = errors.New("URL does not correspond to a defined database")
)

func SetInitialWorkingDir(getWd func() (string, error)) {
	wd, err := getWd()
	if err != nil {
		panic(err)
	}

	INITIAL_WORKING_DIR_PATH = Path(wd)
	if !INITIAL_WORKING_DIR_PATH.IsDirPath() {
		INITIAL_WORKING_DIR_PATH += "/"
	}

	INITIAL_WORKING_DIR_PATH_PATTERN = PathPattern(INITIAL_WORKING_DIR_PATH + "...")
}

// A Manifest contains most of the user-defined metadata about a Module.
type Manifest struct {
	explicitModuleKind ModuleKind
	//note: permissions required for reading the preinit files are in .PreinitFiles.
	RequiredPermissions []Permission
	Limits              []Limit

	HostDefinitions map[Host]Value
	EnvPattern      *ObjectPattern
	Parameters      ModuleParameters
	PreinitFiles    PreinitFiles
	Databases       DatabaseConfigs
	AutoInvocation  *AutoInvocationConfig //can be nil

	InitialWorkingDirectory Path
}

func NewEmptyManifest() *Manifest {
	return &Manifest{
		Parameters: ModuleParameters{
			paramsPattern: EMPTY_MODULE_ARGS_PATTERN,
		},
		InitialWorkingDirectory: DEFAULT_IWD,
	}
}

type ModuleParameters struct {
	positional             []ModuleParameter
	others                 []ModuleParameter
	hasParamsRequiredOnCLI bool //true if at least one positional parameter or one required non-positional parameter
	hasOptions             bool //true if at least one optional non-positional parameter

	paramsPattern *ModuleParamsPattern
}

func (p ModuleParameters) NoParameters() bool {
	return len(p.positional) == 0 && len(p.others) == 0
}

type PreinitFiles []*PreinitFile

type PreinitFile struct {
	Name               string //declared name, this is NOT the basename.
	Path               Path   //absolute
	Pattern            Pattern
	RequiredPermission FilesystemPermission
	Content            []byte
	Parsed             Serializable
	ReadParseError     error
}

type DatabaseConfigs []DatabaseConfig

// A DatabaseConfig represent the configuration of a database accessed by a module.
// The configurations of databases owned by a module are defined in the databases section of the manifest.
// When a module B needs a database owned (defined) by another module A, a DatabaseConfig
// with a set .Provided field is added to the manifest of B.
type DatabaseConfig struct {
	Name                 string       //declared name, this is NOT the basename.
	Resource             SchemeHolder //URL or Host
	ResolutionData       Serializable //ResourceName or Nil
	ExpectedSchemaUpdate bool
	ExpectedSchema       *ObjectPattern //can be nil, not related to .ExpectedSchemaUpdate
	Owned                bool

	Provided *DatabaseIL //optional (can be provided by another module instance)
}

func (c DatabaseConfig) IsPermissionForThisDB(perm DatabasePermission) bool {
	return (DatabasePermission{
		Kind_:  perm.Kind_,
		Entity: c.Resource,
	}).Includes(perm)
}

func (p *ModuleParameters) PositionalParameters() []ModuleParameter {
	return slices.Clone(p.positional)
}

func (p *ModuleParameters) NonPositionalParameters() []ModuleParameter {
	return slices.Clone(p.others)
}

func (p *ModuleParameters) GetArgumentsFromObject(ctx *Context, argObj *Object) (*ModuleArgs, error) {
	var positionalArgs []Serializable
	resultEntries := map[string]Value{}

	restParam := false

	argObj.ForEachElement(ctx, func(index int, v Serializable) error {
		positionalArgs = append(positionalArgs, v)
		return nil
	})

	for paramIndex, param := range p.positional {
		if param.rest {
			list := NewWrappedValueList(positionalArgs[paramIndex:]...)
			if !param.pattern.Test(ctx, list) {
				return nil, fmt.Errorf("invalid value for rest positional argument %s", param.name)
			}
			resultEntries[string(param.name)] = list
			restParam = true
		} else {
			if paramIndex >= len(positionalArgs) {
				return nil, fmt.Errorf("missing value for positional argument %s", param.name)
			}
			arg := positionalArgs[paramIndex]
			if !param.pattern.Test(ctx, arg) {
				return nil, fmt.Errorf("invalid value for positional argument %s", param.name)
			}
			resultEntries[string(param.name)] = arg
		}
	}

	if !restParam && len(positionalArgs) > len(p.positional) {
		return nil, errors.New(fmtTooManyPositionalArgs(len(positionalArgs), len(p.positional)))
	}

	err := argObj.ForEachEntry(func(k string, arg Serializable) error {
		if k == inoxconsts.IMPLICIT_PROP_NAME { //positional arguments are already processed
			return nil
		}

		for _, param := range p.others {
			if string(param.name) == k {
				if !param.pattern.Test(ctx, arg) {
					return fmt.Errorf("invalid value for non positional argument %s", param.name)
				}
				resultEntries[k] = arg
				return nil
			}
		}

		return errors.New(fmtUnknownArgument(k))
	})

	if err != nil {
		return nil, err
	}

	return p.getArguments(ctx, resultEntries)
}

func (p *ModuleParameters) GetArgumentsFromStruct(ctx *Context, argStruct *ModuleArgs) (*ModuleArgs, error) {
	resultEntries := map[string]Value{}

	propertyNames := argStruct.PropertyNames(ctx)

	for _, param := range p.positional {
		paramName := param.Name()

		if !slices.Contains(propertyNames, paramName) {
			return nil, fmt.Errorf("missing value for argument %s", param.name)
		}

		arg := argStruct.Prop(ctx, paramName)

		if !param.pattern.Test(ctx, arg) {
			return nil, fmt.Errorf("invalid value for argument %s", param.name)
		}

		resultEntries[paramName] = arg
	}

	for _, param := range p.others {
		paramName := param.Name()
		if !slices.Contains(propertyNames, paramName) {
			if defaultVal, ok := param.DefaultValue(ctx); ok {
				resultEntries[paramName] = defaultVal
				continue
			} else {
				return nil, fmt.Errorf("missing value for argument %s", param.name)
			}
		}

		arg := argStruct.Prop(ctx, paramName)

		if !param.pattern.Test(ctx, arg) {
			return nil, fmt.Errorf("invalid value for argument %s", param.name)
		}
		resultEntries[paramName] = arg
	}

	err := argStruct.ForEachField(func(fieldName string, fieldValue Value) error {
		_, ok := resultEntries[fieldName]
		if !ok {
			return errors.New(fmtUnknownArgument(fieldName))
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return p.getArguments(ctx, resultEntries)
}

func (p *ModuleParameters) getArguments(ctx *Context, entries map[string]Value) (*ModuleArgs, error) {
	for _, param := range p.others {
		if _, ok := entries[string(param.name)]; !ok {
			return nil, fmt.Errorf("missing value for non positional argument %s", param.name)
		}
	}

	structValues := make([]Value, len(p.paramsPattern.keys))
	for name, value := range entries {
		index, ok := p.paramsPattern.indexOfField(name)
		if !ok {
			panic(ErrUnreachable)
		}
		structValues[index] = value
	}

	return &ModuleArgs{
		pattern: p.paramsPattern,
		values:  structValues,
	}, nil
}

func (p *ModuleParameters) GetArgumentsFromCliArgs(ctx *Context, cliArgs []string) (*ModuleArgs, error) {
	var positionalArgs []string
	entries := map[string]Serializable{}

	// non positional arguments
outer:
	for _, cliArg := range cliArgs {

		for _, param := range p.others {
			paramValue, handled, err := param.GetArgumentFromCliArg(ctx, cliArg)
			if !handled {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("invalid value for argument %s (%s): %w", param.name, param.CliArgNames(), err)
			}

			entries[string(param.name)] = paramValue
			continue outer
		}

		if len(cliArg) > 0 && cliArg[0] == '-' {
			opt := cliArg[1:]
			if len(opt) > 0 && opt[0] == '-' {
				opt = opt[1:]
			}

			name, _, _ := strings.Cut(opt, "=")
			if name != "" {
				return nil, errors.New(fmtUnknownArgument(name))
			}
		}

		positionalArgs = append(positionalArgs, cliArg)
	}

	//default values
	for _, param := range p.others {
		_, ok := entries[string(param.name)]
		if !ok {
			defaultVal, ok := param.DefaultValue(ctx)
			if !ok {

				return nil, fmt.Errorf("missing value for argument %s (%s)", param.name, param.CliArgNames())
			}
			entries[string(param.name)] = defaultVal.(Serializable)
		}
	}

	restParam := false

	for i, param := range p.positional {
		if param.rest {
			paramValue, err := param.GetRestArgumentFromCliArgs(ctx, positionalArgs[i:])
			if err != nil {
				return nil, fmt.Errorf("invalid value for rest argument %s: %w", param.name, err)
			}
			entries[string(param.name)] = paramValue.(Serializable)
		} else {
			if i >= len(positionalArgs) {
				return nil, ErrNotEnoughCliArgs
			}
			paramValue, _, err := param.GetArgumentFromCliArg(ctx, positionalArgs[i])
			if err != nil {
				return nil, fmt.Errorf("invalid value for argument %s: %w", param.name, err)
			}
			entries[string(param.name)] = paramValue
		}
	}

	if !restParam && len(positionalArgs) > len(p.positional) {
		return nil, errors.New(fmtTooManyPositionalArgs(len(positionalArgs), len(p.positional)))
	}

	structValues := make([]Value, len(p.paramsPattern.keys))
	for name, value := range entries {
		index, ok := p.paramsPattern.indexOfField(name)
		if !ok {
			panic(ErrUnreachable)
		}
		structValues[index] = value
	}

	return &ModuleArgs{
		pattern: p.paramsPattern,
		values:  structValues,
	}, nil
}

func (p *ModuleParameters) GetSymbolicArguments(ctx *Context) *symbolic.ModuleArgs {
	resultEntries := map[string]symbolic.Value{}
	encountered := map[uintptr]symbolic.Value{}

	for _, param := range p.others {
		symbolicPatt := utils.Must(param.pattern.ToSymbolicValue(nil, encountered)).(symbolic.Pattern)
		resultEntries[string(param.name)] = symbolicPatt.SymbolicValue()
	}

	for _, param := range p.positional {
		symbolicPatt := utils.Must(param.pattern.ToSymbolicValue(nil, encountered)).(symbolic.Pattern)
		resultEntries[string(param.name)] = symbolicPatt.SymbolicValue()
	}

	symbolicStructType := utils.Must(p.paramsPattern.ToSymbolicValue(ctx, encountered)).(*symbolic.ModuleParamsPattern)
	return symbolic.NewModuleArgs(symbolicStructType, resultEntries)
}

type ModuleParameter struct {
	name                   Identifier
	singleLetterCliArgName rune
	cliArgName             string
	rest                   bool
	positional             bool
	pattern                Pattern
	description            string
	defaultVal             Serializable
}

func (p ModuleParameter) DefaultValue(ctx *Context) (Value, bool) {
	if p.defaultVal != nil {
		return utils.Must(RepresentationBasedClone(ctx, p.defaultVal)), true
	}
	if p.pattern == BOOL_PATTERN {
		return False, true
	}
	return nil, false
}

func (p ModuleParameter) RequiredOnCLI(ctx *Context) bool {
	_, hasDefault := p.DefaultValue(ctx)
	return !hasDefault
}

func (p ModuleParameter) StringifiedPattern() string {
	symb := utils.Must(p.pattern.ToSymbolicValue(nil, map[uintptr]symbolic.Value{}))
	return symbolic.Stringify(symb.(symbolic.Pattern).SymbolicValue())
}

func (p ModuleParameter) StringifiedPatternNoPercent() string {
	return strings.ReplaceAll(p.StringifiedPattern(), "%", "")
}

func (p ModuleParameter) Name() string {
	return string(p.name)
}

func (p ModuleParameter) Pattern() Pattern {
	return p.pattern
}

func (p ModuleParameter) CliArgNames() string {
	buf := bytes.NewBuffer(nil)
	if p.singleLetterCliArgName != 0 {
		buf.WriteByte('-')
		buf.WriteString(string(p.singleLetterCliArgName))

		if p.cliArgName != "" {
			buf.WriteString("|--")
			buf.WriteString(p.cliArgName)
		}
	} else {
		buf.WriteString("--")
		buf.WriteString(p.cliArgName)
	}
	return buf.String()
}

func (p ModuleParameter) GetArgumentFromCliArg(ctx *Context, s string) (v Serializable, handled bool, err error) {
	if len(s) == 0 {
		return nil, false, nil
	}
	if s[0] == '-' {
		if p.positional {
			return nil, false, nil
		}

		cliArg := s[1:]
		if len(s) >= 2 && s[1] == '-' {
			cliArg = s[2:]
		}

		name, valueString, hasValue := strings.Cut(cliArg, "=")

		switch name {
		case string(p.singleLetterCliArgName), string(p.cliArgName):
			if !hasValue {
				if p.pattern == BOOL_PATTERN {
					return True, true, nil
				}
				return nil, true, errors.New("missing value, after the name add '=' followed by a value")
			}

			stringPatt, ok := p.pattern.StringPattern()
			if !ok {
				return nil, true, errors.New("parameter's pattern has no corresponding string pattern")
			}
			argValue, err := stringPatt.Parse(ctx, valueString)
			if err != nil {
				return nil, true, err
			}
			return argValue, true, nil
		default:
			return nil, false, nil
		}
	} else {
		if !p.positional {
			return nil, false, nil
		}

		stringPatt, ok := p.pattern.StringPattern()
		if !ok {
			return nil, true, errors.New("parameter's pattern has no corresponding string pattern")
		}
		argValue, err := stringPatt.Parse(ctx, s)
		if err != nil {
			return nil, true, err
		}
		return argValue, true, nil
	}
}

func (p ModuleParameter) GetRestArgumentFromCliArgs(ctx *Context, args []string) (v Value, err error) {
	if !p.rest {
		return nil, errors.New("not a rest parameter")
	}

	patt, ok := p.pattern.(*ListPattern)
	if !ok {
		return nil, errors.New("only list patterns are supported for rest parameter")
	}

	elemPattern := patt.generalElementPattern
	stringPatt, ok := elemPattern.StringPattern()
	if !ok {
		return nil, errors.New("parameter's pattern has no corresponding string pattern")
	}

	elements := make([]Serializable, len(args))

	for i, arg := range args {
		argValue, err := stringPatt.Parse(ctx, arg)
		if err != nil {
			return nil, err //TODO: improve error
		}
		elements[i] = argValue
	}

	return NewWrappedValueListFrom(elements), nil
}

func (m *Manifest) RequiresPermission(perm Permission) bool {
	for _, requiredPerm := range m.RequiredPermissions {
		if requiredPerm.Includes(perm) {
			return true
		}
	}
	return false
}

func (m *Manifest) ArePermsGranted(grantedPerms []Permission, forbiddenPermissions []Permission) (b bool, missingPermissions []Permission) {
	for _, forbiddenPerm := range forbiddenPermissions {
		if m.RequiresPermission(forbiddenPerm) {
			missingPermissions = append(missingPermissions, forbiddenPerm)
		}
	}
	for _, requiredPerm := range m.RequiredPermissions {
		ok := false
		for _, grantedPerm := range grantedPerms {
			if grantedPerm.Includes(requiredPerm) {
				ok = true
				break
			}
		}
		if !ok {
			missingPermissions = append(missingPermissions, requiredPerm)
		}
	}

	return len(missingPermissions) == 0, missingPermissions
}

func (m *Manifest) Usage(ctx *Context) string {
	buf := bytes.NewBuffer(nil)

	if len(m.Parameters.positional) == 0 && len(m.Parameters.others) == 0 {
		return "no arguments expected"
	}

	for _, param := range m.Parameters.positional {
		buf.WriteByte('<')
		buf.WriteString(string(param.name))
		if param.rest {
			buf.WriteString("...")
		}

		if param.pattern != BOOL_PATTERN {
			buf.WriteByte(' ')
			buf.WriteString(param.StringifiedPatternNoPercent())
		}
		buf.WriteByte('>')
	}

	for _, param := range m.Parameters.others {
		if !param.RequiredOnCLI(ctx) {
			buf.WriteString(" [")
		} else {
			buf.WriteByte(' ')
		}

		buf.WriteString(param.CliArgNames())

		if param.pattern != BOOL_PATTERN {
			buf.WriteByte('=')
			buf.WriteString(param.StringifiedPatternNoPercent())
		}

		if !param.RequiredOnCLI(ctx) {
			buf.WriteByte(']')
		}
	}

	leftPadding := "  "
	tripleLeftPadding := "      "

	if m.Parameters.hasParamsRequiredOnCLI { //rest parameters count as required
		buf.WriteString("\n\nrequired:\n")

		for _, param := range m.Parameters.positional {
			if param.description == "" {
				buf.WriteString(
					fmt.Sprintf("\n%s%s: %s\n", leftPadding, param.name, param.StringifiedPatternNoPercent()))
			} else {
				buf.WriteString(fmt.Sprintf("\n%s%s: %s\n%s%s\n", leftPadding, param.name, param.StringifiedPattern(), tripleLeftPadding, param.description))
			}
		}

		for _, param := range m.Parameters.others {
			if !param.RequiredOnCLI(ctx) {
				continue
			}
			buf.WriteString(
				fmt.Sprintf("\n%s%s (%s): %s\n%s%s\n", leftPadding, param.name, param.CliArgNames(), param.StringifiedPatternNoPercent(), tripleLeftPadding, param.description))
		}
	}

	if m.Parameters.hasOptions {
		buf.WriteString("\noptions:\n")

		for _, param := range m.Parameters.others {
			if param.RequiredOnCLI(ctx) {
				continue
			}
			buf.WriteString(
				fmt.Sprintf("\n%s%s (%s): %s\n%s%s\n", leftPadding, param.name, param.CliArgNames(), param.StringifiedPatternNoPercent(), tripleLeftPadding, param.description))
		}
	}

	return buf.String()
}

func (m *Manifest) OwnedDatabases() []DatabaseConfig {
	return utils.FilterSlice(m.Databases, func(db DatabaseConfig) bool {
		return db.Owned
	})
}

// EvaluatePermissionListingObjectNode evaluates the object literal listing permissions in a permission drop statement.
func EvaluatePermissionListingObjectNode(n *parse.ObjectLiteral, config PreinitArgs) (*Object, error) {

	var state *TreeWalkState

	{
		var checkErr []error
		staticcheck.CheckPermissionListingObject(n, func(n parse.Node, msg string) {
			checkErr = append(checkErr, errors.New(msg))
		})
		if len(checkErr) != 0 {
			return nil, utils.CombineErrors(checkErr...)
		}
	}

	//we create a temporary state to evaluate some parts of the permissions
	if config.RunningState == nil {
		ctx := NewContext(ContextConfig{Permissions: []Permission{GlobalVarPermission{permbase.Read, "*"}}})
		state = NewTreeWalkState(ctx)

		if config.GlobalConsts != nil {
			for _, decl := range config.GlobalConsts.Declarations {
				state.SetGlobal(decl.Ident().Name, utils.Must(TreeWalkEval(decl.Right, state)), GlobalConst)
			}
		}

	} else {
		state = config.RunningState
	}

	v, err := TreeWalkEval(n, state)
	if err == nil {
		return v.(*Object), nil
	}

	return nil, err
}

type CustomPermissionTypeHandler func(kind PermissionKind, name string, value Value) (perms []Permission, handled bool, err error)

type manifestObjectConfig struct {
	defaultLimits           []Limit
	addDefaultPermissions   bool
	handleCustomType        CustomPermissionTypeHandler //optional
	envPattern              *ObjectPattern              //pre-evaluated
	preinitFileConfigs      PreinitFiles                //pre-evaluated
	ignoreUnkownSections    bool
	parentState             *GlobalState //optional
	initialWorkingDirectory Path
}

// createManifest gets permissions and limits by evaluating an object literal.
// Custom permissions are handled by config.HandleCustomType
func (m *Module) createManifest(ctx *Context, object *Object, config manifestObjectConfig) (*Manifest, error) {
	var (
		perms        []Permission
		envPattern   *ObjectPattern
		moduleParams = ModuleParameters{
			paramsPattern: EMPTY_MODULE_ARGS_PATTERN,
		}
		dbConfigs      DatabaseConfigs
		autoInvocation *AutoInvocationConfig
	)
	permListing := NewObject()
	limits := make(map[string]Limit, 0)
	hostDefinitions := make(map[Host]Value, 0)
	specifiedGlobalPermKinds := map[PermissionKind]bool{}
	actualModuleKind := m.Kind
	manifestModuleKind := UnspecifiedModuleKind

	err := object.ForEachEntry(func(k string, v Serializable) error {
		switch k {
		case inoxconsts.MANIFEST_KIND_SECTION_NAME:
			kindName, ok := v.(StringLike)
			if !ok {
				return fmt.Errorf("invalid manifest, the " + k + " section should have a value of type string")
			}

			var err error
			parsedKind, err := inoxmod.ParseModuleKind(kindName.GetOrBuildString())
			if err != nil {
				return err
			}
			if actualModuleKind != UnspecifiedModuleKind && actualModuleKind != parsedKind {
				return errors.New("unexpected state: module kind not equal to the kind determined during parsing")
			}
			if actualModuleKind.IsEmbedded() {
				return errors.New(text.INVALID_KIND_SECTION_EMBEDDED_MOD_KINDS_NOT_ALLOWED)
			}
			manifestModuleKind = parsedKind
		case inoxconsts.MANIFEST_LIMITS_SECTION_NAME:
			l, err := getLimits(v)
			if err != nil {
				return err
			}
			maps.Copy(limits, l)
		case inoxconsts.MANIFEST_HOST_DEFINITIONS_SECTION_NAME:
			definitions, err := getHostDefinitions(v)
			if err != nil {
				return err
			}
			hostDefinitions = definitions
		case inoxconsts.MANIFEST_PERMS_SECTION_NAME:
			listing, ok := v.(*Object)
			if !ok {
				return fmt.Errorf("invalid manifest, the " + inoxconsts.MANIFEST_PERMS_SECTION_NAME + " section should have a value of type object")
			}
			permListing = listing
		case inoxconsts.MANIFEST_ENV_SECTION_NAME:
			envPattern = config.envPattern
			if envPattern == nil {
				return fmt.Errorf("missing pre-evaluated environment pattern")
			}
		case inoxconsts.MANIFEST_PARAMS_SECTION_NAME:
			params, err := getModuleParameters(ctx, v)
			if err != nil {
				return err
			}
			moduleParams = params
		case inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME:
			configs := config.preinitFileConfigs
			if configs == nil {
				return fmt.Errorf("missing pre-evaluated description of %s", inoxconsts.MANIFEST_PREINIT_FILES_SECTION_NAME)
			}
		case inoxconsts.MANIFEST_DATABASES_SECTION_NAME:
			configs, err := getDatabaseConfigurations(v, config.parentState)
			if err != nil {
				return err
			}
			dbConfigs = configs
		case inoxconsts.MANIFEST_INVOCATION_SECTION_NAME:
			description, ok := v.(*Object)
			if !ok {
				return fmt.Errorf("invalid manifest, the '%s' section should have a value of type object", inoxconsts.MANIFEST_INVOCATION_SECTION_NAME)
			}
			autoInvocation = &AutoInvocationConfig{}

			description.ForEachEntry(func(k string, v Serializable) error {
				switch k {
				case inoxconsts.MANIFEST_INVOCATION__ASYNC_PROP_NAME:
					autoInvocation.Async = bool(v.(Bool))
				case inoxconsts.MANIFEST_INVOCATION__ON_ADDED_ELEM_PROP_NAME:
					autoInvocation.OnAddedElement = v.(URL)
				}
				return nil
			})

		default:
			if config.ignoreUnkownSections {
				break
			}
			return fmt.Errorf("invalid manifest, unknown section '%s'", k)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	//add default limits
	for _, limit := range config.defaultLimits {
		if _, ok := limits[limit.Name]; !ok {
			limits[limit.Name] = limit
		}
	}
	//add minimal limits.
	//this piece of code is here to make sure that almost all limits are present.
	limRegistry.forEachRegisteredLimit(func(name string, kind LimitKind, minimum int64) error {

		switch name {
		//ignored because these limits have a .DecrementFn.
		case EXECUTION_TOTAL_LIMIT_NAME, EXECUTION_CPU_TIME_LIMIT_NAME:
			return nil
		}

		if _, ok := limits[name]; !ok {
			limits[name] = Limit{
				Name:  name,
				Kind:  kind,
				Value: minimum,
			}
		}
		return nil
	})

	var ownerDBPermissions []Permission
	//add permissions for accessing owned databases
	for _, db := range dbConfigs {
		if db.Owned {
			ownerDBPermissions = append(ownerDBPermissions,
				DatabasePermission{
					Kind_:  permbase.Read,
					Entity: db.Resource,
				},
				DatabasePermission{
					Kind_:  permbase.Write,
					Entity: db.Resource,
				})
		}
	}

	//finalize the permission list
	perms, err = getPermissionsFromListing(ctx, permListing, specifiedGlobalPermKinds, config.handleCustomType, config.addDefaultPermissions)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	perms = append(ownerDBPermissions, perms...)

	//make sure the invocation events are valid
	if autoInvocation != nil {
		if autoInvocation.OnAddedElement != "" {
			dbFound := false

		search_watched_db:
			for _, db := range dbConfigs {
				switch r := db.Resource.(type) {
				case URL:
					if strings.HasPrefix(autoInvocation.OnAddedElement.UnderlyingString(), r.UnderlyingString()) {
						dbFound = true
						break search_watched_db
					}
				case Host:
					if strings.HasPrefix(autoInvocation.OnAddedElement.UnderlyingString(), r.UnderlyingString()) {
						dbFound = true
						break search_watched_db
					}
				default:
					return nil, fmt.Errorf(
						"invalid manifest: failed to check invocation events: resource of database %q is not supported", db.Name)
				}
			}

			if !dbFound {
				return nil, fmt.Errorf(
					"invalid manifest: errors in invocation events: %w: %q", ErrURLNotCorrespondingToDefinedDB, autoInvocation.OnAddedElement)
			}
		}
	}

	return &Manifest{
		explicitModuleKind:      manifestModuleKind,
		RequiredPermissions:     perms,
		Limits:                  maps.Values(limits),
		HostDefinitions:         hostDefinitions,
		EnvPattern:              envPattern,
		Parameters:              moduleParams,
		PreinitFiles:            config.preinitFileConfigs,
		Databases:               dbConfigs,
		AutoInvocation:          autoInvocation,
		InitialWorkingDirectory: config.initialWorkingDirectory,
	}, nil
}

func evaluateEnvSection(n *parse.ObjectPatternLiteral, state *TreeWalkState, m *Module) (*ObjectPattern, error) {
	v, err := TreeWalkEval(n, state)
	if err != nil {
		if err != nil {
			return nil, fmt.Errorf("%s: failed to evaluate manifest object: %w", m.Name(), err)
		}
	}

	patt, ok := v.(*ObjectPattern)
	if !ok {
		return nil, fmt.Errorf("invalid manifest, the " + inoxconsts.MANIFEST_ENV_SECTION_NAME + " section should have a value of type object pattern")
	}
	err = patt.ForEachEntry(func(entry ObjectPatternEntry) error {
		switch entry.Pattern.(type) {
		case StringPattern, *SecretPattern:
			return nil
		case *TypePattern:
			if entry.Pattern == STR_PATTERN {
				return nil
			}
		default:
		}
		return fmt.Errorf("invalid "+inoxconsts.MANIFEST_ENV_SECTION_NAME+" section in manifest: invalid pattern type %T for environment variable '%s'",
			entry.Pattern, entry.Name)
	})
	if err != nil {
		return nil, err
	}

	return patt, nil
}

func getPermissionsFromListing(
	ctx *Context, permDescriptions *Object, specifiedGlobalPermKinds map[PermissionKind]bool,
	handleCustomType CustomPermissionTypeHandler, addDefaultPermissions bool,
) ([]Permission, error) {
	perms := make([]Permission, 0)

	if specifiedGlobalPermKinds == nil {
		specifiedGlobalPermKinds = make(map[PermissionKind]bool)
	}

	if permDescriptions != nil {
		err := permDescriptions.ForEachEntry(func(propName string, propValue Serializable) error {
			permKind, ok := permbase.PermissionKindFromString(propName)

			if ok {
				p, err := getSingleKindPermissions(permKind, propValue, specifiedGlobalPermKinds, handleCustomType)
				if err != nil {
					return err
				}
				perms = append(perms, p...)
			} else {
				return fmt.Errorf("invalid permission kind: %s", propName)
			}
			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	if addDefaultPermissions {
		// for some permission kinds if no permissions are specified for globals we add a default lax permission
		for _, kind := range []PermissionKind{permbase.Read, permbase.Use, permbase.Create} {
			if !specifiedGlobalPermKinds[kind] {
				perms = append(perms, GlobalVarPermission{kind, "*"})
			}
		}
	}

	return perms, nil
}

func estimatePermissionsFromListingNode(permDescriptions *parse.ObjectLiteral) ([]Permission, error) {
	perms := make([]Permission, 0)

	handleCustomType := func(kind PermissionKind, name string, value Value) (perms []Permission, handled bool, err error) {
		handled = false
		return
	}

	for _, propNode := range permDescriptions.Properties {
		if propNode.HasNoKey() {
			continue
		}
		propName := propNode.Name()
		permKind, ok := permbase.PermissionKindFromString(propName)
		if !ok {
			continue
		}

		propValue, ok := estimatePartialPermissionNodeValue(propNode.Value)
		if ok {
			p, err := getSingleKindPermissions(permKind, propValue, map[permbase.PermissionKind]bool{}, handleCustomType)
			if err != nil {
				return nil, err
			}
			perms = append(perms, p...)
		}
	}

	return perms, nil
}

func estimatePartialPermissionNodeValue(n parse.Node) (Serializable, bool) {
	switch node := n.(type) {
	case *parse.ObjectLiteral:
		values := ValMap{}
		i := 0
		for _, propNode := range node.Properties {
			var key string
			if propNode.HasNoKey() {
				key = strconv.Itoa(i)
			} else {
				key = propNode.Name()
			}
			val, ok := estimatePartialPermissionNodeValue(propNode.Value)
			if ok {
				values[key] = val
			}
		}
		return NewObjectFromMapNoInit(values), true
	case *parse.ListLiteral:
		var elements []Serializable
		for _, elemNode := range node.Elements {
			elem, ok := estimatePartialPermissionNodeValue(elemNode)
			if ok {
				elements = append(elements, elem)
			}
		}
		return NewWrappedValueList(elements...), true
	case *parse.IdentifierLiteral:
		return nil, false
	case parse.SimpleValueLiteral:
		result, err := EvalSimpleValueLiteral(node, nil)
		if err != nil {
			return nil, false
		}
		return result, true
	}
	return nil, false
}

func GetDefaultGlobalVarPermissions() (perms []Permission) {
	for _, kind := range []PermissionKind{permbase.Read, permbase.Use, permbase.Create} {
		perms = append(perms, GlobalVarPermission{kind, "*"})
	}
	return
}

func getLimits(desc Value) (map[string]Limit, error) {
	limits := make(map[string]Limit, 0)
	ctx := NewContext(ContextConfig{
		DoNotSpawnDoneGoroutine: true,
	})
	defer ctx.CancelGracefully()

	limitObj, isObj := desc.(*Object)
	if !isObj {
		return nil, fmt.Errorf("invalid manifest, description of limits should be an object")
	}

	//add limits

	err := limitObj.ForEachEntry(func(limitName string, limitPropValue Serializable) error {
		var limit Limit
		limit, err := GetLimit(ctx, limitName, limitPropValue)

		if err != nil {
			return err
		}

		if _, ok := limits[limit.Name]; ok {
			panic(ErrUnreachable)
		}

		limits[limit.Name] = limit
		return nil
	})

	if err != nil {
		return nil, err
	}

	return limits, nil
}

func getHostDefinitions(desc Value) (map[Host]Value, error) {

	resolutions := make(map[Host]Value)

	dict, ok := desc.(*Dictionary)
	if !ok {
		return nil, fmt.Errorf("invalid manifest, description of %s should be an object", inoxconsts.MANIFEST_HOST_DEFINITIONS_SECTION_NAME)
	}

	for k, v := range dict.entries {
		host, ok := dict.keys[k].(Host)
		if !ok {
			return nil, fmt.Errorf("invalid manifest, keys of of %s should be hosts", inoxconsts.MANIFEST_HOST_DEFINITIONS_SECTION_NAME)
		}
		// resource, ok := v.(ResourceName)
		// if !ok {
		// 	return nil, fmt.Errorf("invalid manifest, values of of %s should be resource names", inoxconsts.MANIFEST_HOST_RESOLUTION_SECTION_NAME)
		// }

		resolutions[host] = v
	}

	return resolutions, nil
}

func getSingleKindPermissions(
	permKind PermissionKind, desc Value, specifiedGlobalPermKinds map[PermissionKind]bool,
	handleCustomType CustomPermissionTypeHandler,
) ([]Permission, error) {

	var perms []Permission

	var list []Serializable
	switch v := desc.(type) {
	case *List:
		list = v.GetOrBuildElements(nil)
	case *Object:

		err := v.ForEachElement(nil, func(index int, v Serializable) error {
			list = append(list, v)
			return nil
		})

		if err != nil {
			return nil, err
		}

		err = v.ForEachEntry(func(propKey string, propVal Serializable) error {
			if propKey == inoxconsts.IMPLICIT_PROP_NAME {
				return nil
			}

			p, err := getSingleKindNamedPermPermissions(permKind, propKey, propVal, specifiedGlobalPermKinds, handleCustomType)
			if err != nil {
				return fmt.Errorf("invalid manifest: %w", err)
			}
			perms = append(perms, p...)
			return nil
		})
		if err != nil {
			return nil, err
		}

	default:
		list = []Serializable{v.(Serializable)}
	}

	for _, e := range list {
		perm, err := getPermissionFromSingleKindPermissionItem(e, permKind)
		if err != nil {
			return nil, fmt.Errorf("invalid manifest: %w", err)
		}
		perms = append(perms, perm)
	}

	return perms, nil
}

func getSingleKindNamedPermPermissions(
	permKind PermissionKind,
	propKey string,
	propVal Value,
	specifiedGlobalPermKinds map[PermissionKind]bool,
	handleCustomType CustomPermissionTypeHandler,
) ([]Permission, error) {
	typeName := propKey

	var p []Permission
	var err error

	switch typeName {
	case "dns":
		p, err = getDnsPermissions(permKind, propVal)
	case "tcp":
		p, err = getRawTcpPermissions(permKind, propVal)
	case "globals":
		p, err = getGlobalVarPerms(permKind, propVal, specifiedGlobalPermKinds)
	case "env":
		p, err = getEnvVarPermissions(permKind, propVal)
	case "threads":
		switch propVal.(type) {
		case *Object:
			p = []Permission{LThreadPermission{permKind}}
		default:
			return nil, errors.New("invalid permission, 'threads' should be followed by an object literal")
		}
	case "system-graph":
		switch propVal.(type) {
		case *Object:
			p = []Permission{SystemGraphAccessPermission{permKind}}
		default:
			return nil, errors.New("invalid permission, 'system-graph' should be followed by an object literal")
		}
	case "commands":
		if permKind != permbase.Use {
			return nil, errors.New("permission 'commands' should be required in the 'use' section of permission")
		}

		newPerms, err := getCommandPermissions(propVal)
		if err != nil {
			return nil, err
		}
		p = newPerms
	case "values":
		if permKind != permbase.See {
			if permKind != permbase.Read {
				return nil, fmt.Errorf("invalid permissions: 'values' is only defined for 'see'")
			}
		}

		newPerms, err := getVisibilityPerms(propVal)
		if err != nil {
			return nil, err
		}
		p = newPerms
	case "custom":
		if handleCustomType != nil {
			customPerms, handled, err := handleCustomType(permKind, typeName, propVal)
			if handled {
				if err != nil {
					return nil, fmt.Errorf(text.FmtCannotInferPermission(permKind.String(), typeName)+": %w", err)
				}
				p = append(p, customPerms...)
				break
			}
		}
		fallthrough
	default:
		return nil, errors.New(text.FmtCannotInferPermission(permKind.String(), typeName))
	}

	if err != nil {
		return nil, err
	}

	return p, nil
}

func getPermissionFromSingleKindPermissionItem(e Value, permKind PermissionKind) (Permission, error) {
	switch v := e.(type) {
	case URL:
		switch string(v.Scheme()) {
		case "wss", "ws":
			return WebsocketPermission{
				Kind_:    permKind,
				Endpoint: v,
			}, nil
		case "http", "https":
			return HttpPermission{
				Kind_:  permKind,
				Entity: v,
			}, nil
		case inoxconsts.LDB_SCHEME_NAME, inoxconsts.ODB_SCHEME_NAME:
			return DatabasePermission{
				Kind_:  permKind,
				Entity: v,
			}, nil
		default:
			return nil, fmt.Errorf("URL has a valid but unsupported scheme '%s'", v.Scheme())
		}

	case URLPattern:
		switch string(v.Scheme()) {
		case "http", "https":
			return HttpPermission{
				Kind_:  permKind,
				Entity: v,
			}, nil
		case inoxconsts.LDB_SCHEME_NAME, inoxconsts.ODB_SCHEME_NAME:
			return DatabasePermission{
				Kind_:  permKind,
				Entity: v,
			}, nil
		}
	case Host:
		switch string(v.Scheme()) {
		case "wss", "ws":
			return WebsocketPermission{
				Kind_:    permKind,
				Endpoint: v,
			}, nil
		case "http", "https":
			return HttpPermission{
				Kind_:  permKind,
				Entity: v,
			}, nil
		case inoxconsts.LDB_SCHEME_NAME, inoxconsts.ODB_SCHEME_NAME:
			return DatabasePermission{
				Kind_:  permKind,
				Entity: v,
			}, nil
		default:
			return nil, fmt.Errorf("Host %s has a valid scheme '%s' that makes no sense here", v, v.Scheme())
		}
	case HostPattern:
		switch v.Scheme() {
		case "http", "https":
			return HttpPermission{
				Kind_:  permKind,
				Entity: v,
			}, nil
		default:
			return nil, fmt.Errorf("HostPattern %s has a valid scheme '%s' that makes no sense here", v, v.Scheme())
		}
	case Path:
		if !v.IsAbsolute() {
			return nil, errors.New(text.FmtOnlyAbsPathsAreAcceptedInPerms(v.UnderlyingString()))
		}
		return FilesystemPermission{
			Kind_:  permKind,
			Entity: v,
		}, nil

	case PathPattern:
		if !v.IsAbsolute() {
			return nil, errors.New(text.FmtOnlyAbsPathPatternsAreAcceptedInPerms(v.UnderlyingString()))
		}
		return FilesystemPermission{
			Kind_:  permKind,
			Entity: v,
		}, nil
	default:
	}
	return nil, fmt.Errorf("cannot infer permission, value is a(n) %T", e)
}

func getModuleParameters(ctx *Context, v Value) (ModuleParameters, error) {
	description, ok := v.(*Object)
	if !ok {
		return ModuleParameters{}, fmt.Errorf("invalid manifest, the '%s' section should have a value of type object", inoxconsts.MANIFEST_PARAMS_SECTION_NAME)
	}

	var params ModuleParameters
	restParamFound := false

	//positional parameters.
	err := description.ForEachElement(ctx, func(index int, v Serializable) error {
		var param ModuleParameter

		obj, ok := v.(*Object)
		if !ok {
			return errors.New("each positional parameter should be described with an object")
		}

		obj.ForEachEntry(func(propName string, propVal Serializable) error {
			switch propName {
			case inoxconsts.MANIFEST_NON_POSITIONAL_PARAM__NAME_PROPNAME: //name
				param.name = propVal.(Identifier)
				param.positional = true
			case inoxconsts.MANIFEST_POSITIONAL_PARAM__REST_PROPNAME: //rest
				rest := bool(propVal.(Bool))
				if rest && restParamFound {
					return errors.New("at most one positional parameter should be a rest parameter")
				}
				param.rest = rest
				restParamFound = rest
			case inoxconsts.MANIFEST_PARAM__PATTERN_PROPNAME: //pattern
				patt := propVal.(Pattern)
				param.pattern = patt
			case inoxconsts.MANIFEST_PARAM__DESCRIPTION_PROPNAME:
				param.description = string(propVal.(String))
			}
			return nil
		})
		if param.pattern == nil {
			return errors.New("missing .pattern in description of positional parameter")
		}

		params.positional = append(params.positional, param)
		return nil
	})

	if err != nil {
		return ModuleParameters{}, fmt.Errorf("invalid manifest: '%s' section: %w", inoxconsts.MANIFEST_PARAMS_SECTION_NAME, err)
	}

	//non-positional parameters.
	err = description.ForEachEntry(func(k string, v Serializable) error {
		if k == inoxconsts.IMPLICIT_PROP_NAME {
			return nil
		}

		var param ModuleParameter
		param.name = Identifier(k)

		switch val := v.(type) {
		case *OptionPattern:
			if len(val.name) == 1 {
				param.singleLetterCliArgName = rune(val.name[0])
			} else {
				param.cliArgName = val.name
			}
			param.pattern = val.value
		case Pattern:
			param.cliArgName = string(param.name)
			param.pattern = val
		case *Object:
			paramDesc := val
			param.cliArgName = k

			paramDesc.ForEachEntry(func(propName string, propVal Serializable) error {
				if propName == inoxconsts.IMPLICIT_PROP_NAME {
					return nil
				}
				switch propName {
				case inoxconsts.MANIFEST_PARAM__PATTERN_PROPNAME:
					patt := propVal.(Pattern)
					param.pattern = patt
				case inoxconsts.MANIFEST_NON_POSITIONAL_PARAM__DEFAULT_PROPNAME:
					param.defaultVal = propVal
				case inoxconsts.MANIFEST_NON_POSITIONAL_PARAM__CHAR_NAME_PROPNAME:
					param.singleLetterCliArgName = rune(propVal.(Rune))
				case inoxconsts.MANIFEST_PARAM__DESCRIPTION_PROPNAME:
					param.description = string(propVal.(String))
				}
				return nil
			})
		default:
			return errors.New("each non positional parameter should be described with a pattern or an object")
		}

		if param.pattern == nil {
			return errors.New("missing .pattern in description of non positional parameter")
		}
		if param.RequiredOnCLI(ctx) {
			params.hasParamsRequiredOnCLI = true
		} else {
			params.hasOptions = true
		}

		params.others = append(params.others, param)
		return nil
	})

	if len(params.positional) > 0 {
		params.hasParamsRequiredOnCLI = true
	}

	if err != nil {
		return ModuleParameters{}, fmt.Errorf("invalid manifest: '%s' section: %w", inoxconsts.MANIFEST_PARAMS_SECTION_NAME, err)
	}

	var paramNames []string
	var paramPatterns []Pattern

	for _, param := range params.positional {
		paramNames = append(paramNames, param.name.UnderlyingString())
		paramPatterns = append(paramPatterns, param.Pattern())
	}

	for _, param := range params.others {
		paramNames = append(paramNames, param.name.UnderlyingString())
		paramPatterns = append(paramPatterns, param.Pattern())
	}

	params.paramsPattern = NewModuleParamsPattern(paramNames, paramPatterns)
	return params, nil
}

func getDatabaseConfigurations(v Value, parentState *GlobalState) (DatabaseConfigs, error) {
	var configs DatabaseConfigs

	if path, ok := v.(Path); ok {
		var provider *GlobalState

		if parentState != nil {
			if parentState.Module != nil && parentState.Module.MainChunk.Source.Name() == path.UnderlyingString() {
				provider = parentState
			} else if parentState.MainState != nil {
				parentState.MainState.descendantStatesLock.Lock()
				provider = parentState.MainState.descendantStates[path]
				parentState.MainState.descendantStatesLock.Unlock()
			}
		}

		if provider == nil {
			return nil, fmt.Errorf("failed to get the module providing one or more databases: state of %s not found", path.UnderlyingString())
		}

		for _, dbConfig := range provider.Manifest.Databases {
			dbConfig.Provided = provider.Databases[dbConfig.Name]
			dbConfig.Owned = false
			configs = append(configs, dbConfig)
		}

		return configs, nil
	}

	description, ok := v.(*Object)
	if !ok {
		return nil, fmt.Errorf("invalid manifest, the '%s' section should have a value of type object or path", inoxconsts.MANIFEST_DATABASES_SECTION_NAME)
	}

	err := description.ForEachEntry(func(dbName string, desc Serializable) error {
		dbDesc, ok := desc.(*Object)
		if !ok {
			return errors.New("each database should be described with an object")
		}

		config := DatabaseConfig{Name: dbName, Owned: true}

		err := dbDesc.ForEachEntry(func(propName string, propVal Serializable) error {
			switch propName {
			case inoxconsts.MANIFEST_DATABASE__RESOURCE_PROP_NAME:
				switch val := propVal.(type) {
				case Host:
					config.Resource = val
				case URL:
					config.Resource = val
				default:
					return fmt.Errorf("invalid value found for the .%s of a database description", inoxconsts.MANIFEST_DATABASE__RESOURCE_PROP_NAME)
				}
			case inoxconsts.MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME:
				switch val := propVal.(type) {
				case Path:
					config.ResolutionData = val
				case Host:
					config.ResolutionData = val
				case NilT:
					config.ResolutionData = Nil
				default:
					return fmt.Errorf("invalid value found for the .%s of a database description", inoxconsts.MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME)
				}
			case inoxconsts.MANIFEST_DATABASE__EXPECTED_SCHEMA_UPDATE_PROP_NAME:
				switch val := propVal.(type) {
				case Bool:
					config.ExpectedSchemaUpdate = bool(val)
				default:
					return fmt.Errorf("invalid value found for the .%s of a database description", inoxconsts.MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME)
				}
			case inoxconsts.MANIFEST_DATABASE__ASSERT_SCHEMA_UPDATE_PROP_NAME:
				switch val := propVal.(type) {
				case *ObjectPattern:
					config.ExpectedSchema = val
					err := checkDatabaseSchema(val)
					if err != nil {
						return err
					}
				default:
					return fmt.Errorf("invalid value found for the .%s of a database description", inoxconsts.MANIFEST_DATABASE__ASSERT_SCHEMA_UPDATE_PROP_NAME)
				}
			}
			return nil
		})
		configs = append(configs, config)

		return err
	})

	if err != nil {
		return nil, fmt.Errorf("invalid manifest: '%s' section: %w", inoxconsts.MANIFEST_DATABASES_SECTION_NAME, err)
	}

	return configs, nil
}

// getDnsPermissions gets a list DNSPermission from an AST node
func getDnsPermissions(permKind PermissionKind, desc Value) ([]Permission, error) {
	var perms []Permission

	if permKind != permbase.Read {
		return nil, fmt.Errorf("invalid manifest, 'dns' is only defined for 'read'")
	}
	dnsReqNodes := make([]Serializable, 0)

	switch v := desc.(type) {
	case *List:
		dnsReqNodes = append(dnsReqNodes, v.GetOrBuildElements(nil)...)
	default:
		dnsReqNodes = append(dnsReqNodes, v.(Serializable))
	}

	for _, dnsReqNode := range dnsReqNodes {
		var domain GoString

		switch simpleVal := dnsReqNode.(type) {
		case Host:
			if simpleVal.HasScheme() {
				return nil, fmt.Errorf("invalid manifest, 'dns' host should have no scheme")
			}
			domain = simpleVal
		case HostPattern:
			if simpleVal.HasScheme() {
				return nil, fmt.Errorf("invalid manifest, 'dns' host pattern should have no scheme")
			}
			domain = simpleVal
		default:
			return nil, fmt.Errorf("invalid manifest, 'dns' should be followed by a (or a list of) host or host pattern literals, not %T", simpleVal)
		}

		perms = append(perms, DNSPermission{
			Kind_:  permKind,
			Domain: domain,
		})
	}

	return perms, nil
}

// getRawTcpPermissions gets a list RawTcpPermission from an AST node
func getRawTcpPermissions(permKind PermissionKind, desc Value) ([]Permission, error) {
	var perms []Permission
	tcpRequirementNodes := make([]Serializable, 0)

	switch v := desc.(type) {
	case *List:
		tcpRequirementNodes = append(tcpRequirementNodes, v.GetOrBuildElements(nil)...)
	default:
		tcpRequirementNodes = append(tcpRequirementNodes, v.(Serializable))
	}

	for _, dnsReqNode := range tcpRequirementNodes {
		var domain GoString

		switch simpleVal := dnsReqNode.(type) {
		case Host:
			if simpleVal.HasScheme() {
				return nil, errors.New("invalid manifest, 'tcp' host literals should have no scheme")
			}
			domain = simpleVal
		case HostPattern:
			if simpleVal.HasScheme() {
				return nil, errors.New("invalid manifest, 'tcp' host pattern literals should have no scheme")
			}
			domain = simpleVal
		default:
			return nil, fmt.Errorf("invalid manifest, 'tcp' should be followed by a (or a list of) host or host pattern literals, not %T", simpleVal)
		}

		perms = append(perms, RawTcpPermission{
			Kind_:  permKind,
			Domain: domain,
		})
	}

	return perms, nil
}

func getGlobalVarPerms(permKind PermissionKind, desc Value, specifiedGlobalPermKinds map[PermissionKind]bool) ([]Permission, error) {

	var perms []Permission
	globalReqNodes := make([]Serializable, 0)

	switch v := desc.(type) {
	case *List:
		globalReqNodes = append(globalReqNodes, v.GetOrBuildElements(nil)...)
	default:
		globalReqNodes = append(globalReqNodes, v.(Serializable))
	}

	for _, gn := range globalReqNodes {
		nameOrAny, ok := gn.(String)
		if !ok { //TODO: + check with regex
			return nil, errors.New("invalid manifest, 'globals' should be followed by a (or a list of) variable name(s) or a star *")
		}

		specifiedGlobalPermKinds[permKind] = true

		perms = append(perms, GlobalVarPermission{
			Kind_: permKind,
			Name:  string(nameOrAny),
		})
	}

	return perms, nil
}

// getEnvVarPermissions gets a list EnvVarPermission from an AST node
func getEnvVarPermissions(permKind PermissionKind, desc Value) ([]Permission, error) {
	var perms []Permission
	envReqNodes := make([]Serializable, 0)

	switch v := desc.(type) {
	case *List:
		envReqNodes = append(envReqNodes, v.GetOrBuildElements(nil)...)
	default:
		envReqNodes = append(envReqNodes, v.(Serializable))
	}

	for _, n := range envReqNodes {
		nameOrAny, ok := n.(String)
		if !ok { //TODO: + check with regex
			log.Panicln("invalid manifest, 'globals' should be followed by a (or a list of) variable name(s) or a start *")
		}

		perms = append(perms, EnvVarPermission{
			Kind_: permKind,
			Name:  string(nameOrAny),
		})
	}

	return perms, nil
}

// getCommandPermissions gets a list of CommandPermission from an AST node
func getCommandPermissions(n Value) ([]Permission, error) {

	var perms []Permission

	topObject, ok := n.(*Object)
	if !ok {
		return nil, errors.New(text.CMD_NAME_ERROR)
	}

	err := topObject.ForEachEntry(func(name string, propValue Serializable) error {
		if name == inoxconsts.IMPLICIT_PROP_NAME {
			return nil
		}

		var cmdNameKey = name
		var cmdName GoString
		if strings.HasPrefix(cmdNameKey, "./") || strings.HasPrefix(cmdNameKey, "/") || strings.HasPrefix(cmdNameKey, "%") {

			const PATH_ERR = text.INVALID_COMMANDS_PREFIX + "command starting with / or ./ should be valid paths"

			chunk, err := parse.ParseChunk(cmdNameKey, "")
			if err != nil || len(chunk.Statements) != 1 {
				return errors.New(PATH_ERR)
			}
			switch pth := chunk.Statements[0].(type) {
			case *parse.AbsolutePathLiteral:
				cmdName = Path(pth.Value)
			case *parse.RelativePathLiteral:
				cmdName = Path(pth.Value)
			case *parse.AbsolutePathPatternLiteral:
				cmdName = PathPattern(pth.Value)
			case *parse.RelativePathPatternLiteral:
				cmdName = PathPattern(pth.Value)
			default:
				return errors.New(PATH_ERR)
			}

			if patt, ok := cmdName.(PathPattern); ok && !patt.IsPrefixPattern() {
				return errors.New("only prefix path patterns are allowed")
			}
		} else {
			cmdName = String(cmdNameKey)
		}

		cmdDesc, ok := propValue.(*Object)
		if !ok {
			return errors.New(text.CMD_NAME_ERROR)
		}

		if len(cmdDesc.keys) == 0 {
			cmdPerm := CommandPermission{
				CommandName: cmdName,
			}
			perms = append(perms, cmdPerm)
			return nil
		}

		err := getSubcommandPermissions(cmdName, cmdDesc, &perms)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return perms, nil
}

func getSubcommandPermissions(cmdName GoString, cmdDesc *Object, perms *[]Permission) error {
	return cmdDesc.ForEachEntry(func(subcmdName string, cmdDescPropVal Serializable) error {
		if subcmdName == inoxconsts.IMPLICIT_PROP_NAME {
			return errors.New(text.INVALID_COMMANDS_PREFIX + "elements (values without a key) are not allowed")
		}

		subCmdDesc, ok := cmdDescPropVal.(*Object)
		if !ok {
			return errors.New(text.CMD_NAME_ERROR)
		}

		if len(subCmdDesc.keys) == 0 {
			subcommandPerm := CommandPermission{
				CommandName:         cmdName,
				SubcommandNameChain: []string{subcmdName},
			}
			*perms = append(*perms, subcommandPerm)
			return nil
		}

		return subCmdDesc.ForEachEntry(func(deepSubCmdName string, subCmdDescPropVal Serializable) error {

			if _, err := strconv.Atoi(deepSubCmdName); err == nil {
				return errors.New(text.CMD_NAME_ERROR)
			}

			nestedSubCmdDesc, ok := subCmdDescPropVal.(*Object)
			if !ok {
				return errors.New(text.CMD_NAME_ERROR)
			}

			if len(nestedSubCmdDesc.keys) != 0 {
				return errors.New(text.INVALID_COMMANDS_PREFIX + "the subcommand chain has a maximum length of 2")
			}

			subcommandPerm := CommandPermission{
				CommandName:         cmdName,
				SubcommandNameChain: []string{subcmdName, deepSubCmdName},
			}
			*perms = append(*perms, subcommandPerm)
			return nil
		})
	})
}

func getVisibilityPerms(desc Value) ([]Permission, error) {
	var perms []Permission
	values := make([]Serializable, 0)

	switch v := desc.(type) {
	case *List:
		values = append(values, v.GetOrBuildElements(nil)...)
	default:
		values = append(values, v.(Serializable))
	}

	for _, val := range values {
		patt, ok := val.(Pattern)
		if !ok {
			return nil, fmt.Errorf("invalid value in visibility section, .values should be a pattern or a list of patterns")
		}
		perms = append(perms, ValueVisibilityPermission{Pattern: patt})
	}

	return perms, nil
}
