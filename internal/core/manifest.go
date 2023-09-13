package core

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"slices"

	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/oklog/ulid/v2"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MANIFEST_ENV_SECTION_NAME             = "env"
	MANIFEST_PARAMS_SECTION_NAME          = "parameters"
	MANIFEST_PERMS_SECTION_NAME           = "permissions"
	MANIFEST_LIMITS_SECTION_NAME          = "limits"
	MANIFEST_HOST_RESOLUTION_SECTION_NAME = "host-resolution"

	//preinit-files section
	MANIFEST_PREINIT_FILES_SECTION_NAME      = "preinit-files"
	MANIFEST_PREINIT_FILE__PATTERN_PROP_NAME = "pattern"
	MANIFEST_PREINIT_FILE__PATH_PROP_NAME    = "path"

	MANIFEST_DATABASES_SECTION_NAME                     = "databases"
	MANIFEST_DATABASE__RESOURCE_PROP_NAME               = "resource"
	MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME        = "resolution-data"
	MANIFEST_DATABASE__EXPECTED_SCHEMA_UPDATE_PROP_NAME = "expected-schema-update"

	INITIAL_WORKING_DIR_VARNAME        = "IWD"
	INITIAL_WORKING_DIR_PREFIX_VARNAME = "IWD_PREFIX"
)

var (
	//the initial working dir is the working dir at the start of the program execution.
	INITIAL_WORKING_DIR_PATH         Path
	INITIAL_WORKING_DIR_PATH_PATTERN PathPattern

	MANIFEST_SECTION_NAMES = []string{
		MANIFEST_ENV_SECTION_NAME, MANIFEST_PARAMS_SECTION_NAME,
		MANIFEST_PERMS_SECTION_NAME, MANIFEST_LIMITS_SECTION_NAME,
		MANIFEST_HOST_RESOLUTION_SECTION_NAME, MANIFEST_PREINIT_FILES_SECTION_NAME,
		MANIFEST_DATABASES_SECTION_NAME,
	}
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
	//note: permissions required for reading the preinit files are in .PreinitFiles.
	RequiredPermissions []Permission
	Limits              []Limits

	HostResolutions map[Host]Value
	EnvPattern      *ObjectPattern
	Parameters      ModuleParameters
	PreinitFiles    PreinitFiles
	Databases       DatabaseConfigs
}

func NewEmptyManifest() *Manifest {
	return &Manifest{
		Parameters: ModuleParameters{
			structType: ANON_EMPTY_STRUCT_TYPE,
		},
	}
}

type ModuleParameters struct {
	positional        []ModuleParameter
	others            []ModuleParameter
	hasRequiredParams bool //true if at least one positional parameter or one required non-positional parameter
	hasOptions        bool //true if at least one optional non-positional parameter

	structType *StructPattern
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

type DatabaseConfig struct {
	Name                 string       //declared name, this is NOT the basename.
	Resource             SchemeHolder //URL or Host
	ResolutionData       ResourceName
	ExpectedSchemaUpdate bool
	Owned                bool

	Provided *DatabaseIL //optional (can be provided by parent state)
}

func (p *ModuleParameters) PositionalParameters() []ModuleParameter {
	return utils.CopySlice(p.positional)
}

func (p *ModuleParameters) NonPositionalParameters() []ModuleParameter {
	return utils.CopySlice(p.others)
}

func (p *ModuleParameters) GetArgumentsFromObject(ctx *Context, argObj *Object) (*Struct, error) {
	positionalArgs := argObj.Indexed()
	resultEntries := map[string]Value{}

	restParam := false

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
		if IsIndexKey(k) { //positional arguments are already processed
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

func (p *ModuleParameters) GetArgumentsFromStruct(ctx *Context, argStruct *Struct) (*Struct, error) {
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

func (p *ModuleParameters) getArguments(ctx *Context, entries map[string]Value) (*Struct, error) {
	for _, param := range p.others {
		if _, ok := entries[string(param.name)]; !ok {
			return nil, fmt.Errorf("missing value for non positional argument %s", param.name)
		}
	}

	structValues := make([]Value, len(p.structType.keys))
	for name, value := range entries {
		index, ok := p.structType.indexOfField(name)
		if !ok {
			panic(ErrUnreachable)
		}
		structValues[index] = value
	}

	return &Struct{
		structType: p.structType,
		values:     structValues,
	}, nil
}

func (p *ModuleParameters) GetArgumentsFromCliArgs(ctx *Context, cliArgs []string) (*Struct, error) {
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

	structValues := make([]Value, len(p.structType.keys))
	for name, value := range entries {
		index, ok := p.structType.indexOfField(name)
		if !ok {
			panic(ErrUnreachable)
		}
		structValues[index] = value
	}

	return &Struct{
		structType: p.structType,
		values:     structValues,
	}, nil
}

func (p *ModuleParameters) GetSymbolicArguments(ctx *Context) *symbolic.Struct {
	resultEntries := map[string]symbolic.SymbolicValue{}
	encountered := map[uintptr]symbolic.SymbolicValue{}

	for _, param := range p.others {
		symbolicPatt := utils.Must(param.pattern.ToSymbolicValue(nil, encountered)).(symbolic.Pattern)
		resultEntries[string(param.name)] = symbolicPatt.SymbolicValue()
	}

	for _, param := range p.positional {
		symbolicPatt := utils.Must(param.pattern.ToSymbolicValue(nil, encountered)).(symbolic.Pattern)
		resultEntries[string(param.name)] = symbolicPatt.SymbolicValue()
	}

	symbolicStructType := utils.Must(p.structType.ToSymbolicValue(ctx, encountered)).(*symbolic.StructPattern)
	return symbolic.NewStruct(symbolicStructType, resultEntries)
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

func (p ModuleParameter) Required(ctx *Context) bool {
	_, hasDefault := p.DefaultValue(ctx)
	return !hasDefault
}

func (p ModuleParameter) StringifiedPattern() string {
	symb := utils.Must(p.pattern.ToSymbolicValue(nil, map[uintptr]symbolic.SymbolicValue{}))
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
			return argValue.(Serializable), true, nil
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
		return argValue.(Serializable), true, nil
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
		elements[i] = argValue.(Serializable)
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
		if !param.Required(ctx) {
			buf.WriteString(" [")
		} else {
			buf.WriteByte(' ')
		}

		buf.WriteString(param.CliArgNames())

		if param.pattern != BOOL_PATTERN {
			buf.WriteByte('=')
			buf.WriteString(param.StringifiedPatternNoPercent())
		}

		if !param.Required(ctx) {
			buf.WriteByte(']')
		}
	}

	leftPadding := "  "
	tripleLeftPadding := "      "

	if m.Parameters.hasRequiredParams { //rest parameters count as required
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
			if !param.Required(ctx) {
				continue
			}
			buf.WriteString(
				fmt.Sprintf("\n%s%s (%s): %s\n%s%s\n", leftPadding, param.name, param.CliArgNames(), param.StringifiedPatternNoPercent(), tripleLeftPadding, param.description))
		}
	}

	if m.Parameters.hasOptions {
		buf.WriteString("\noptions:\n")

		for _, param := range m.Parameters.others {
			if param.Required(ctx) {
				continue
			}
			buf.WriteString(
				fmt.Sprintf("\n%s%s (%s): %s\n%s%s\n", leftPadding, param.name, param.CliArgNames(), param.StringifiedPatternNoPercent(), tripleLeftPadding, param.description))
		}
	}

	return buf.String()
}

// EvaluatePermissionListingObjectNode evaluates the object literal listing permissions in a permission drop statement.
func EvaluatePermissionListingObjectNode(n *parse.ObjectLiteral, config PreinitArgs) (*Object, error) {

	var state *TreeWalkState

	{
		var checkErr []error
		checkPermissionListingObject(n, func(n parse.Node, msg string) {
			checkErr = append(checkErr, errors.New(msg))
		})
		if len(checkErr) != 0 {
			return nil, combineErrors(checkErr...)
		}
	}

	//we create a temporary state to evaluate some parts of the permissions
	if config.RunningState == nil {
		ctx := NewContext(ContextConfig{Permissions: []Permission{GlobalVarPermission{permkind.Read, "*"}}})
		state = NewTreeWalkState(ctx, getGlobalsAccessibleFromManifest().ValueEntryMap(nil))

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
	defaultLimits         []Limits
	addDefaultPermissions bool
	handleCustomType      CustomPermissionTypeHandler //optional
	envPattern            *ObjectPattern              //pre-evaluated
	preinitFileConfigs    PreinitFiles                //pre-evaluated
	ignoreUnkownSections  bool
	parentState           *GlobalState //optional
}

// createManifest gets permissions and limits by evaluating an object literal.
// Custom permissions are handled by config.HandleCustomType
func (m *Module) createManifest(ctx *Context, object *Object, config manifestObjectConfig) (*Manifest, error) {
	var (
		perms        []Permission
		envPattern   *ObjectPattern
		moduleParams = ModuleParameters{
			structType: ANON_EMPTY_STRUCT_TYPE,
		}
		dbConfigs DatabaseConfigs
	)
	permListing := NewObject()
	limits := make([]Limits, 0)
	hostResolutions := make(map[Host]Value, 0)
	defaultLimitsToNotSet := make(map[string]bool)
	specifiedGlobalPermKinds := map[PermissionKind]bool{}

	for k, v := range object.EntryMap(nil) {
		switch k {
		case MANIFEST_LIMITS_SECTION_NAME:
			l, err := getLimits(v, defaultLimitsToNotSet)
			if err != nil {
				return nil, err
			}
			limits = append(limits, l...)
		case MANIFEST_HOST_RESOLUTION_SECTION_NAME:
			resolutions, err := getHostResolutions(v)
			if err != nil {
				return nil, err
			}
			hostResolutions = resolutions
		case MANIFEST_PERMS_SECTION_NAME:
			listing, ok := v.(*Object)
			if !ok {
				return nil, fmt.Errorf("invalid manifest, the " + MANIFEST_PERMS_SECTION_NAME + " section should have a value of type object")
			}
			permListing = listing
		case MANIFEST_ENV_SECTION_NAME:
			envPattern = config.envPattern
			if envPattern == nil {
				return nil, fmt.Errorf("missing pre-evaluated environment pattern")
			}
		case MANIFEST_PARAMS_SECTION_NAME:
			params, err := getModuleParameters(ctx, v)
			if err != nil {
				return nil, err
			}
			moduleParams = params
		case MANIFEST_PREINIT_FILES_SECTION_NAME:
			configs := config.preinitFileConfigs
			if configs == nil {
				return nil, fmt.Errorf("missing pre-evaluated description of %s", MANIFEST_PREINIT_FILES_SECTION_NAME)
			}
		case MANIFEST_DATABASES_SECTION_NAME:
			configs, err := getDatabaseConfigurations(v, config.parentState)
			if err != nil {
				return nil, err
			}
			dbConfigs = configs
		default:
			if config.ignoreUnkownSections {
				continue
			}
			return nil, fmt.Errorf("invalid manifest, unknown section '%s'", k)
		}
	}

	//add default limits
	for _, limit := range config.defaultLimits {
		if defaultLimitsToNotSet[limit.Name] {
			continue
		}
		limits = append(limits, limit)
	}

	var ownerDBPermissions []Permission
	//add permissions for accessing owned databases
	for _, db := range dbConfigs {
		if db.Owned {
			ownerDBPermissions = append(ownerDBPermissions,
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: db.Resource,
				},
				DatabasePermission{
					Kind_:  permkind.Write,
					Entity: db.Resource,
				})
		}
	}

	perms, err := getPermissionsFromListing(ctx, permListing, specifiedGlobalPermKinds, config.handleCustomType, config.addDefaultPermissions)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	perms = append(ownerDBPermissions, perms...)

	return &Manifest{
		RequiredPermissions: perms,
		Limits:              limits,
		HostResolutions:     hostResolutions,
		EnvPattern:          envPattern,
		Parameters:          moduleParams,
		PreinitFiles:        config.preinitFileConfigs,
		Databases:           dbConfigs,
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
		return nil, fmt.Errorf("invalid manifest, the " + MANIFEST_ENV_SECTION_NAME + " section should have a value of type object pattern")
	}
	err = patt.ForEachEntry(func(propName string, propPattern Pattern, _ bool) error {
		switch propPattern.(type) {
		case StringPattern, *SecretPattern:
			return nil
		case *TypePattern:
			if propPattern == STR_PATTERN {
				return nil
			}
		default:
		}
		return fmt.Errorf("invalid "+MANIFEST_ENV_SECTION_NAME+" section in manifest: invalid pattern type %T for environment variable '%s'", propPattern, propName)
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

	for propName, propValue := range permDescriptions.EntryMap(nil) {
		permKind, ok := permkind.PermissionKindFromString(propName)

		if ok {
			p, err := getSingleKindPermissions(ctx, permKind, propValue, specifiedGlobalPermKinds, handleCustomType)
			if err != nil {
				return nil, err
			}
			perms = append(perms, p...)
		} else {
			return nil, fmt.Errorf("invalid permission kind: %s", propName)
		}
	}

	if addDefaultPermissions {
		// for some permission kinds if no permissions are specified for globals we add a default lax permission
		for _, kind := range []PermissionKind{permkind.Read, permkind.Use, permkind.Create} {
			if !specifiedGlobalPermKinds[kind] {
				perms = append(perms, GlobalVarPermission{kind, "*"})
			}
		}
	}

	return perms, nil
}

func GetDefaultGlobalVarPermissions() (perms []Permission) {
	for _, kind := range []PermissionKind{permkind.Read, permkind.Use, permkind.Create} {
		perms = append(perms, GlobalVarPermission{kind, "*"})
	}
	return
}

func getLimits(desc Value, defaultLimitsToNotSet map[string]bool) ([]Limits, error) {
	var limits []Limits
	ctx := NewContext(ContextConfig{})

	limitObj, isObj := desc.(*Object)
	if !isObj {
		return nil, fmt.Errorf("invalid manifest, description of limits should be an object")
	}

	//add limits

	for limitName, limitPropValue := range limitObj.EntryMap(nil) {

		var limit Limits
		defaultLimitsToNotSet[limitName] = true

		switch v := limitPropValue.(type) {
		case Rate:
			limit = Limits{Name: limitName}

			switch r := v.(type) {
			case ByteRate:
				limit.Kind = ByteRateLimit
				limit.Value = int64(r)
			case SimpleRate:
				limit.Kind = SimpleRateLimit
				limit.Value = int64(r)
			default:
				return nil, fmt.Errorf("not a valid rate type %T", r)
			}

		case Int:
			limit = Limits{
				Name:  limitName,
				Kind:  TotalLimit,
				Value: int64(v),
			}
		case Duration:
			limit = Limits{
				Name:  limitName,
				Kind:  TotalLimit,
				Value: int64(v),
			}
		default:
			return nil, fmt.Errorf("invalid manifest, invalid value %s for a limit", GetRepresentation(v, ctx))
		}

		registeredKind, registeredMinimum, ok := LimRegistry.getLimitInfo(limitName)
		if !ok {
			return nil, fmt.Errorf("invalid manifest, limits: '%s' is not a registered limit", limitName)
		}
		if limit.Kind != registeredKind {
			return nil, fmt.Errorf("invalid manifest, limits: value of '%s' has not a valid type", limitName)
		}
		if registeredMinimum > 0 && limit.Value < registeredMinimum {
			return nil, fmt.Errorf("invalid manifest, limits: value for limit '%s' is too low, minimum is %d", limitName, registeredMinimum)
		}

		limits = append(limits, limit)
	}

	//check & postprocess limits

	for i, l := range limits {
		switch l.Name {
		case EXECUTION_TOTAL_LIMIT_NAME:
			if l.Value == 0 {
				log.Panicf("invalid manifest, limits: %s should have a total value\n", EXECUTION_TOTAL_LIMIT_NAME)
			}
			l.DecrementFn = func(lastDecrementTime time.Time) int64 {
				return time.Since(lastDecrementTime).Nanoseconds()
			}
		}
		limits[i] = l
	}

	return limits, nil
}

func getHostResolutions(desc Value) (map[Host]Value, error) {

	resolutions := make(map[Host]Value)

	dict, ok := desc.(*Dictionary)
	if !ok {
		return nil, fmt.Errorf("invalid manifest, description of %s should be an object", MANIFEST_HOST_RESOLUTION_SECTION_NAME)
	}

	for k, v := range dict.entries {
		host, ok := dict.keys[k].(Host)
		if !ok {
			return nil, fmt.Errorf("invalid manifest, keys of of %s should be hosts", MANIFEST_HOST_RESOLUTION_SECTION_NAME)
		}
		// resource, ok := v.(ResourceName)
		// if !ok {
		// 	return nil, fmt.Errorf("invalid manifest, values of of %s should be resource names", MANIFEST_HOST_RESOLUTION_SECTION_NAME)
		// }

		resolutions[host] = v
	}

	return resolutions, nil
}

func getSingleKindPermissions(
	ctx *Context, permKind PermissionKind, desc Value, specifiedGlobalPermKinds map[PermissionKind]bool,
	handleCustomType CustomPermissionTypeHandler,
) ([]Permission, error) {

	name := permKind.String()

	var perms []Permission

	var list []Serializable
	switch v := desc.(type) {
	case *List:
		list = v.GetOrBuildElements(nil)
	case *Object:
		for propKey, propVal := range v.EntryMap(nil) {

			if _, err := strconv.Atoi(propKey); err == nil {
				list = append(list, propVal)
			} else if propKey != IMPLICIT_KEY_LEN_KEY {
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
				case "routines":
					switch propVal.(type) {
					case *Object:
						perms = append(perms, RoutinePermission{permKind})
					default:
						return nil, errors.New("invalid permission, 'routines' should be followed by an object literal")
					}
				case "system-graph":
					switch propVal.(type) {
					case *Object:
						perms = append(perms, SystemGraphAccessPermission{permKind})
					default:
						return nil, errors.New("invalid permission, 'system-graph' should be followed by an object literal")
					}
				case "commands":
					if permKind != permkind.Use {
						return nil, errors.New("permission 'commands' should be required in the 'use' section of permission")
					}

					newPerms, err := getCommandPermissions(propVal)
					if err != nil {
						return nil, err
					}
					perms = append(perms, newPerms...)
				case "values":
					if permKind != permkind.See {
						if permKind != permkind.Read {
							return nil, fmt.Errorf("invalid manifest, invalid permissions: 'values' is only defined for 'see'")
						}
					}

					newPerms, err := getVisibilityPerms(propVal)
					if err != nil {
						return nil, err
					}
					perms = append(perms, newPerms...)
				default:
					if handleCustomType != nil {
						customPerms, handled, err := handleCustomType(permKind, typeName, propVal)
						if handled {
							if err != nil {
								return nil, fmt.Errorf("invalid manifest, cannot infer '%s' permission '%s': %s", name, typeName, err.Error())
							}
							perms = append(perms, customPerms...)
							break
						}
					}

					return nil, fmt.Errorf("invalid manifest, cannot infer '%s' permission '%s'", name, typeName)
				}

				if err != nil {
					return nil, err
				}
				perms = append(perms, p...)
			}
		}
	default:
		list = []Serializable{v.(Serializable)}
	}

	for _, e := range list {

		switch v := e.(type) {
		case URL:
			switch v.Scheme() {
			case "wss", "ws":
				perms = append(perms, WebsocketPermission{
					Kind_:    permKind,
					Endpoint: v,
				})
			case "http", "https":
				perms = append(perms, HttpPermission{
					Kind_:  permKind,
					Entity: v,
				})
			case "ldb", "odb":
				perms = append(perms, DatabasePermission{
					Kind_:  permKind,
					Entity: v,
				})
			default:
				return nil, fmt.Errorf("invalid manifest, URL has a valid but unsupported scheme '%s'", v.Scheme())
			}

		case URLPattern:
			switch v.Scheme() {
			case "http", "https":
				perms = append(perms, HttpPermission{
					Kind_:  permKind,
					Entity: v,
				})
			case "ldb", "odb":
				perms = append(perms, DatabasePermission{
					Kind_:  permKind,
					Entity: v,
				})
			}
		case Host:
			switch v.Scheme() {
			case "wss", "ws":
				perms = append(perms, WebsocketPermission{
					Kind_:    permKind,
					Endpoint: v,
				})
			case "http", "https":
				perms = append(perms, HttpPermission{
					Kind_:  permKind,
					Entity: v,
				})
			case "ldb", "odb":
				perms = append(perms, DatabasePermission{
					Kind_:  permKind,
					Entity: v,
				})
			default:
				return nil, fmt.Errorf("invalid manifest, Host %s has a valid scheme '%s' that makes no sense here", v, v.Scheme())
			}
		case HostPattern:
			switch v.Scheme() {
			case "http", "https":
				perms = append(perms, HttpPermission{
					Kind_:  permKind,
					Entity: v,
				})
			default:
				return nil, fmt.Errorf("invalid manifest, HostPattern %s has a valid scheme '%s' that makes no sense here", v, v.Scheme())
			}
		case Path:
			perms = append(perms, FilesystemPermission{
				Kind_:  permKind,
				Entity: v,
			})
			if !v.IsAbsolute() {
				return nil, fmt.Errorf("invalid manifest, only absolute paths are accepted: %s", v)
			}
		case PathPattern:
			perms = append(perms, FilesystemPermission{
				Kind_:  permKind,
				Entity: v,
			})
			if !v.IsAbsolute() {
				return nil, fmt.Errorf("invalid manifest, only absolute path patterns are accepted: %s", v)
			}
		default:
			return nil, fmt.Errorf("invalid manifest, cannot infer permission, value is a(n) %T", v)
		}
	}

	return perms, nil
}

func getModuleParameters(ctx *Context, v Value) (ModuleParameters, error) {
	description, ok := v.(*Object)
	if !ok {
		return ModuleParameters{}, fmt.Errorf("invalid manifest, the '%s' section should have a value of type object", MANIFEST_PARAMS_SECTION_NAME)
	}

	var params ModuleParameters
	restParamFound := false

	err := description.ForEachEntry(func(k string, v Serializable) error {
		var param ModuleParameter

		if IsIndexKey(k) { //positional parameter
			obj, ok := v.(*Object)
			if !ok {
				return errors.New("each positional parameter should be described with an object")
			}

			obj.ForEachEntry(func(propName string, propVal Serializable) error {
				switch propName {
				case "name":
					param.name = propVal.(Identifier)
					param.positional = true
				case "rest":
					rest := bool(propVal.(Bool))
					if rest && restParamFound {
						return errors.New("at most one positional parameter should be a rest parameter")
					}
					param.rest = rest
					restParamFound = rest
				case "pattern":
					patt := propVal.(Pattern)
					param.pattern = patt
				case "description":
					param.description = string(propVal.(Str))
				}
				return nil
			})
			if param.pattern == nil {
				return errors.New("missing .pattern in description of positional parameter")
			}

			params.positional = append(params.positional, param)
		} else { // non positional parameter
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
					switch propName {
					case "pattern":
						patt := propVal.(Pattern)
						param.pattern = patt
					case "default":
						param.defaultVal = propVal
					case "char-name":
						param.singleLetterCliArgName = rune(propVal.(Rune))
					case "description":
						param.description = string(propVal.(Str))
					}
					return nil
				})
			default:
				return errors.New("each non positional parameter should be described with a pattern or an object")
			}

			if param.pattern == nil {
				return errors.New("missing .pattern in description of non positional parameter")
			}
			if param.Required(ctx) {
				params.hasRequiredParams = true
			} else {
				params.hasOptions = true
			}

			params.others = append(params.others, param)
		}
		return nil
	})

	if len(params.positional) > 0 {
		params.hasRequiredParams = true
	}

	if err != nil {
		return ModuleParameters{}, fmt.Errorf("invalid manifest: '%s' section: %w", MANIFEST_PARAMS_SECTION_NAME, err)
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

	params.structType = NewStructPattern("", ulid.Make(), paramNames, paramPatterns)
	return params, nil
}

func getDatabaseConfigurations(v Value, parentState *GlobalState) (DatabaseConfigs, error) {
	var configs DatabaseConfigs

	if path, ok := v.(Path); ok {
		var provider *GlobalState

		if parentState != nil {
			if parentState.Module.MainChunk.Source.Name() == path.UnderlyingString() {
				provider = parentState
			} else {
				parentState.MainState.descendantStatesLock.Lock()
				provider = parentState.MainState.descendantStates[path]
				parentState.MainState.descendantStatesLock.Unlock()
			}
		}

		if provider == nil {
			return nil, fmt.Errorf("state of %s not found", path.UnderlyingString())
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
		return nil, fmt.Errorf("invalid manifest, the '%s' section should have a value of type object or path", MANIFEST_DATABASES_SECTION_NAME)
	}

	err := description.ForEachEntry(func(dbName string, desc Serializable) error {
		dbDesc, ok := desc.(*Object)
		if !ok {
			return errors.New("each database should be described with an object")
		}

		config := DatabaseConfig{Name: dbName, Owned: true}

		err := dbDesc.ForEachEntry(func(propName string, propVal Serializable) error {
			switch propName {
			case MANIFEST_DATABASE__RESOURCE_PROP_NAME:
				switch val := propVal.(type) {
				case Host:
					config.Resource = val
				case URL:
					config.Resource = val
				default:
					return fmt.Errorf("invalid value found for the .%s of a database description", MANIFEST_DATABASE__RESOURCE_PROP_NAME)
				}
			case MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME:
				switch val := propVal.(type) {
				case Path:
					config.ResolutionData = val
				case Host:
					config.ResolutionData = val
				default:
					return fmt.Errorf("invalid value found for the .%s of a database description", MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME)
				}
			case MANIFEST_DATABASE__EXPECTED_SCHEMA_UPDATE_PROP_NAME:
				switch val := propVal.(type) {
				case Bool:
					config.ExpectedSchemaUpdate = bool(val)
				default:
					return fmt.Errorf("invalid value found for the .%s of a database description", MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME)
				}
			}
			return nil
		})
		configs = append(configs, config)

		return err
	})

	if err != nil {
		return nil, fmt.Errorf("invalid manifest: '%s' section: %w", MANIFEST_DATABASES_SECTION_NAME, err)
	}

	return configs, nil
}

// getDnsPermissions gets a list DNSPermission from an AST node
func getDnsPermissions(permKind PermissionKind, desc Value) ([]Permission, error) {
	var perms []Permission

	if permKind != permkind.Read {
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
		var domain WrappedString

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
		var domain WrappedString

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
		nameOrAny, ok := gn.(Str)
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
		nameOrAny, ok := n.(Str)
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

	const ERR_PREFIX = "invalid manifest, use: commands: "
	const ERR = ERR_PREFIX + "a command (or subcommand) name should be followed by object literals with the next subcommands as keys (or empty)"

	var perms []Permission

	topObject, ok := n.(*Object)
	if !ok {
		return nil, errors.New(ERR)
	}

	for name, propValue := range topObject.EntryMap(nil) {

		if IsIndexKey(name) {
			return nil, errors.New(ERR_PREFIX + "implicit/index keys are not allowed")
		}

		var cmdNameKey = name
		var cmdName WrappedString
		if strings.HasPrefix(cmdNameKey, "./") || strings.HasPrefix(cmdNameKey, "/") || strings.HasPrefix(cmdNameKey, "%") {

			const PATH_ERR = ERR_PREFIX + "command starting with / or ./ should be valid paths"

			chunk, err := parse.ParseChunk(cmdNameKey, "")
			if err != nil || len(chunk.Statements) != 1 {
				return nil, errors.New(PATH_ERR)
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
				return nil, errors.New(PATH_ERR)
			}
		} else {
			cmdName = Str(cmdNameKey)
		}

		cmdDesc, ok := propValue.(*Object)
		if !ok {
			return nil, errors.New(ERR)
		}

		if len(cmdDesc.keys) == 0 {
			cmdPerm := CommandPermission{
				CommandName: cmdName,
			}
			perms = append(perms, cmdPerm)
			continue
		}

		for subcmdName, cmdDescPropVal := range cmdDesc.EntryMap(nil) {

			if _, err := strconv.Atoi(subcmdName); err == nil {
				return nil, errors.New(ERR_PREFIX + "implicit keys are not allowed")
			}

			subCmdDesc, ok := cmdDescPropVal.(*Object)
			if !ok {
				return nil, errors.New(ERR)
			}

			if len(subCmdDesc.keys) == 0 {
				subcommandPerm := CommandPermission{
					CommandName:         cmdName,
					SubcommandNameChain: []string{subcmdName},
				}
				perms = append(perms, subcommandPerm)
				continue
			}

			for deepSubCmdName, subCmdDescPropVal := range subCmdDesc.EntryMap(nil) {

				if _, err := strconv.Atoi(deepSubCmdName); err == nil {
					return nil, errors.New(ERR)
				}

				deepSubCmdDesc, ok := subCmdDescPropVal.(*Object)
				if !ok {
					return nil, errors.New(ERR)
				}

				if len(deepSubCmdDesc.keys) == 0 {
					subcommandPerm := CommandPermission{
						CommandName:         cmdName,
						SubcommandNameChain: []string{subcmdName, deepSubCmdName},
					}
					perms = append(perms, subcommandPerm)
					continue
				}

				return nil, errors.New(ERR_PREFIX + "the subcommand chain has a maximum length of 2")
			}
		}
	}

	return perms, nil
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

func getGlobalsAccessibleFromManifest() *Object {
	return objFrom(ValMap{
		INITIAL_WORKING_DIR_VARNAME:        INITIAL_WORKING_DIR_PATH,
		INITIAL_WORKING_DIR_PREFIX_VARNAME: INITIAL_WORKING_DIR_PATH_PATTERN,
	})
}
