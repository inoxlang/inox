package internal

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	//the initial working dir is the working dir at the start of the program execution.
	INITIAL_WORKING_DIR_PATH         Path
	INITIAL_WORKING_DIR_PATH_PATTERN PathPattern
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
	RequiredPermissions []Permission
	Limitations         []Limitation
	HostResolutions     map[Host]Value
	EnvPattern          *ObjectPattern
	Parameters          ModuleParameters
}

func NewEmptyManifest() *Manifest {
	return &Manifest{}
}

type ModuleParameters struct {
	positional []moduleParameter
	others     []moduleParameter
}

func (p *ModuleParameters) GetArguments(ctx *Context, argObj *Object) (*Object, error) {
	positionalArgs := argObj.Indexed()
	resultEntries := map[string]Value{}

	for i, param := range p.positional {
		if param.rest {
			list := NewWrappedValueList(positionalArgs[i:]...)
			if !param.pattern.Test(ctx, list) {
				return nil, fmt.Errorf("invalid value for rest positional argument %s", param.name)
			}
			resultEntries[string(param.name)] = list
		} else {
			if i >= len(positionalArgs) {
				return nil, fmt.Errorf("missing value for positional argument %s", param.name)
			}
			arg := positionalArgs[i]
			if !param.pattern.Test(ctx, arg) {
				return nil, fmt.Errorf("invalid value for positional argument %s", param.name)
			}
			resultEntries[string(param.name)] = arg
		}
	}

	argObj.ForEachEntry(func(k string, arg Value) error {
		if IsIndexKey(k) { //positional arguments are already processed
			return nil
		}

		for _, param := range p.others {
			if string(param.name) == k {
				if !param.pattern.Test(ctx, arg) {
					return fmt.Errorf("invalid value for non positional argument %s", param.name)
				}
				resultEntries[k] = arg
			}
		}

		return nil
	})

	for _, param := range p.others {
		if _, ok := resultEntries[string(param.name)]; !ok {
			return nil, fmt.Errorf("missing value for non positional argument %s", param.name)
		}
	}

	return objFrom(resultEntries), nil
}

func (p *ModuleParameters) GetArgumentsFromCliArgs(ctx *Context, cliArgs []string) (*Object, error) {
	var remainingCliArgs []string
	entries := map[string]Value{}

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

		remainingCliArgs = append(remainingCliArgs, cliArg)
	}

	//default values
	for _, param := range p.others {
		_, ok := entries[string(param.name)]
		if !ok {
			defaultVal, ok := param.DefaultValue()
			if !ok {

				return nil, fmt.Errorf("missing value for argument %s (%s)", param.name, param.CliArgNames())
			}
			entries[string(param.name)] = defaultVal
		}
	}

	for i, param := range p.positional {
		if param.rest {
			paramValue, err := param.GetRestArgumentFromCliArgs(ctx, remainingCliArgs[i:])
			if err != nil {
				return nil, fmt.Errorf("invalid value for rest argument %s: %w", param.name, err)
			}
			entries[string(param.name)] = paramValue
		} else {
			if i >= len(remainingCliArgs) {
				return nil, ErrNotEnoughCliArgs
			}
			paramValue, _, err := param.GetArgumentFromCliArg(ctx, remainingCliArgs[i])
			if err != nil {
				return nil, fmt.Errorf("invalid value for argument %s: %w", param.name, err)
			}
			entries[string(param.name)] = paramValue
		}
	}

	return objFrom(entries), nil
}

func (p *ModuleParameters) GetSymbolicArguments() *symbolic.Object {
	resultEntries := map[string]symbolic.SymbolicValue{}
	encountered := map[uintptr]symbolic.SymbolicValue{}

	for _, param := range p.others {
		symbolicPatt := utils.Must(param.pattern.ToSymbolicValue(false, encountered)).(symbolic.Pattern)
		resultEntries[string(param.name)] = symbolicPatt.SymbolicValue()
	}

	for _, param := range p.positional {
		symbolicPatt := utils.Must(param.pattern.ToSymbolicValue(false, encountered)).(symbolic.Pattern)
		resultEntries[string(param.name)] = symbolicPatt.SymbolicValue()
	}

	return symbolic.NewObject(resultEntries, nil)
}

type moduleParameter struct {
	name                   Identifier
	singleLetterCliArgName rune
	cliArgName             string
	rest                   bool
	positional             bool
	pattern                Pattern
	description            string
	defaultVal             Value
}

func (p moduleParameter) DefaultValue() (Value, bool) {
	if p.defaultVal != nil {
		return utils.Must(p.defaultVal.Clone(map[uintptr]map[int]Value{})), true
	}
	if p.pattern == BOOL_PATTERN {
		return False, true
	}
	return nil, false
}

func (p moduleParameter) Required() bool {
	_, hasDefault := p.DefaultValue()
	return !hasDefault
}

func (p moduleParameter) StringifiedPattern() string {
	symb := utils.Must(p.pattern.ToSymbolicValue(false, map[uintptr]symbolic.SymbolicValue{}))
	return symbolic.Stringify(symb.(symbolic.Pattern).SymbolicValue())
}

func (p moduleParameter) StringifiedPatternNoPercent() string {
	return strings.ReplaceAll(p.StringifiedPattern(), "%", "")
}

func (p moduleParameter) CliArgNames() string {
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

func (p moduleParameter) GetArgumentFromCliArg(ctx *Context, s string) (v Value, handled bool, err error) {
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

func (p moduleParameter) GetRestArgumentFromCliArgs(ctx *Context, args []string) (v Value, err error) {
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

	elements := make([]Value, len(args))

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

func (m *Manifest) Usage() string {
	buf := bytes.NewBuffer(nil)
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
		if !param.Required() {
			buf.WriteString(" [")
		} else {
			buf.WriteByte(' ')
		}

		buf.WriteString(param.CliArgNames())

		if param.pattern != BOOL_PATTERN {
			buf.WriteByte('=')
			buf.WriteString(param.StringifiedPatternNoPercent())
		}

		if !param.Required() {
			buf.WriteByte(']')
		}
	}

	buf.WriteString("\n\nrequired:\n")
	leftPadding := "  "
	tripleLeftPadding := "      "

	for _, param := range m.Parameters.positional {
		if param.description == "" {
			buf.WriteString(
				fmt.Sprintf("\n%s%s: %s\n", leftPadding, param.name, param.StringifiedPatternNoPercent()))
		} else {
			buf.WriteString(fmt.Sprintf("\n%s%s: %s\n%s%s\n", leftPadding, param.name, param.StringifiedPattern(), tripleLeftPadding, param.description))
		}
	}

	for _, param := range m.Parameters.others {
		if !param.Required() {
			continue
		}
		buf.WriteString(
			fmt.Sprintf("\n%s%s (%s): %s\n%s%s\n", leftPadding, param.name, param.CliArgNames(), param.StringifiedPatternNoPercent(), tripleLeftPadding, param.description))
	}

	buf.WriteString("\noptions:\n")

	for _, param := range m.Parameters.others {
		if param.Required() {
			continue
		}
		buf.WriteString(
			fmt.Sprintf("\n%s%s (%s): %s\n%s%s\n", leftPadding, param.name, param.CliArgNames(), param.StringifiedPatternNoPercent(), tripleLeftPadding, param.description))
	}

	return buf.String()
}

// EvaluatePermissionListingObjectNode evaluates the object literal listing permissions in a permission drop statement.
func EvaluatePermissionListingObjectNode(n *parse.ObjectLiteral, config ManifestEvaluationConfig) (*Object, error) {

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
		ctx := NewContext(ContextConfig{Permissions: []Permission{GlobalVarPermission{ReadPerm, "*"}}})
		state = NewTreeWalkState(ctx, getGlobalsAccessibleFromManifest().EntryMap())

		if config.GlobalConsts != nil {
			for _, decl := range config.GlobalConsts.Declarations {
				state.SetGlobal(decl.Left.Name, utils.Must(TreeWalkEval(decl.Right, state)), GlobalConst)
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
	defaultLimitations    []Limitation
	addDefaultPermissions bool
	handleCustomType      CustomPermissionTypeHandler //optional
	ignoreUnkownSections  bool
}

// createManifest gets permissions and limitations by evaluating an object literal.
// Custom permissions are handled by config.HandleCustomType
func createManifest(object *Object, config manifestObjectConfig) (*Manifest, error) {

	var (
		perms        []Permission
		envPattern   *ObjectPattern
		moduleParams ModuleParameters
	)
	permListing := NewObject()
	limitations := make([]Limitation, 0)
	hostResolutions := make(map[Host]Value, 0)
	defaultLimitationsToNotSet := make(map[string]bool)
	specifiedGlobalPermKinds := map[PermissionKind]bool{}

	for k, v := range object.EntryMap() {
		switch k {
		case "limits":
			l, err := getLimitations(v, defaultLimitationsToNotSet)
			if err != nil {
				return nil, err
			}
			limitations = append(limitations, l...)
		case "host_resolution":
			resolutions, err := getHostResolutions(v)
			if err != nil {
				return nil, err
			}
			hostResolutions = resolutions
		case "permissions":
			listing, ok := v.(*Object)
			if !ok {
				return nil, fmt.Errorf("invalid manifest, the 'permissions' section should have a value of type object")
			}
			permListing = listing
		case "env":
			patt, ok := v.(*ObjectPattern)
			if !ok {
				return nil, fmt.Errorf("invalid manifest, the 'env' section should have a value of type object pattern")
			}
			envPattern = patt
		case "parameters":
			params, err := getModuleParameters(v)
			if err != nil {
				return nil, err
			}
			moduleParams = params
		default:
			if config.ignoreUnkownSections {
				continue
			}
			return nil, fmt.Errorf("invalid manifest, unknown section '%s'", k)
		}
	}

	//add default limitations
	for _, limitation := range config.defaultLimitations {
		if defaultLimitationsToNotSet[limitation.Name] {
			continue
		}
		limitations = append(limitations, limitation)
	}

	perms, err := getPermissionsFromListing(permListing, specifiedGlobalPermKinds, config.handleCustomType, config.addDefaultPermissions)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &Manifest{
		RequiredPermissions: perms,
		Limitations:         limitations,
		HostResolutions:     hostResolutions,
		EnvPattern:          envPattern,
		Parameters:          moduleParams,
	}, nil
}

func getPermissionsFromListing(
	permDescriptions *Object, specifiedGlobalPermKinds map[PermissionKind]bool,
	handleCustomType CustomPermissionTypeHandler, addDefaultPermissions bool,
) ([]Permission, error) {
	perms := make([]Permission, 0)

	if specifiedGlobalPermKinds == nil {
		specifiedGlobalPermKinds = make(map[PermissionKind]bool)
	}

	for propName, propValue := range permDescriptions.EntryMap() {
		permKind, ok := PermissionKindFromString(propName)

		if ok {
			p, err := getSingleKindPermissions(permKind, propValue, specifiedGlobalPermKinds, handleCustomType)
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
		for _, kind := range []PermissionKind{ReadPerm, UsePerm, CreatePerm} {
			if !specifiedGlobalPermKinds[kind] {
				perms = append(perms, GlobalVarPermission{kind, "*"})
			}
		}
	}

	return perms, nil
}

func GetDefaultGlobalVarPermissions() (perms []Permission) {
	for _, kind := range []PermissionKind{ReadPerm, UsePerm, CreatePerm} {
		perms = append(perms, GlobalVarPermission{kind, "*"})
	}
	return
}

func getLimitations(desc Value, defaultLimitationsToNotSet map[string]bool) ([]Limitation, error) {
	var limitations []Limitation
	ctx := NewContext(ContextConfig{})

	limitObj, isObj := desc.(*Object)
	if !isObj {
		return nil, fmt.Errorf("invalid manifest, description of limits should be an object")
	}

	//add limits

	for limitName, limitPropValue := range limitObj.EntryMap() {

		var limitation Limitation
		defaultLimitationsToNotSet[limitName] = true

		switch v := limitPropValue.(type) {
		case Rate:
			limitation = Limitation{Name: limitName}

			switch r := v.(type) {
			case ByteRate:
				limitation.Kind = ByteRateLimitation
				limitation.Value = int64(r)
			case SimpleRate:
				limitation.Kind = SimpleRateLimitation
				limitation.Value = int64(r)
			default:
				return nil, fmt.Errorf("not a valid rate type %T", r)
			}

		case Int:
			limitation = Limitation{
				Name:  limitName,
				Kind:  TotalLimitation,
				Value: int64(v),
			}
		case Duration:
			limitation = Limitation{
				Name:  limitName,
				Kind:  TotalLimitation,
				Value: int64(v),
			}
		default:
			return nil, fmt.Errorf("invalid manifest, invalid value %s for a limit", GetRepresentation(v, ctx))
		}

		registeredKind, registeredMinimum, ok := LimRegistry.getLimitationInfo(limitName)
		if !ok {
			return nil, fmt.Errorf("invalid manifest, limits: '%s' is not a registered limitation", limitName)
		}
		if limitation.Kind != registeredKind {
			return nil, fmt.Errorf("invalid manifest, limits: value of '%s' has not a valid type", limitName)
		}
		if registeredMinimum > 0 && limitation.Value < registeredMinimum {
			return nil, fmt.Errorf("invalid manifest, limits: value for limitation '%s' is too low, minimum is %d", limitName, registeredMinimum)
		}

		limitations = append(limitations, limitation)
	}

	//check & postprocess limits

	for i, l := range limitations {
		switch l.Name {
		case EXECUTION_TOTAL_LIMIT_NAME:
			if l.Value == 0 {
				log.Panicf("invalid manifest, limits: %s should have a total value\n", EXECUTION_TOTAL_LIMIT_NAME)
			}
			l.DecrementFn = func(lastDecrementTime time.Time) int64 {
				v := TOKEN_BUCKET_CAPACITY_SCALE * time.Since(lastDecrementTime)
				return v.Nanoseconds()
			}
		}
		limitations[i] = l
	}

	return limitations, nil
}

func getHostResolutions(desc Value) (map[Host]Value, error) {

	resolutions := make(map[Host]Value)

	dict, ok := desc.(*Dictionary)
	if !ok {
		return nil, fmt.Errorf("invalid manifest, description of host_resolution should be an object")
	}

	for k, v := range dict.Entries {
		host, ok := dict.Keys[k].(Host)
		if !ok {
			return nil, fmt.Errorf("invalid manifest, keys of of host_resolution should be hosts")
		}
		// resource, ok := v.(ResourceName)
		// if !ok {
		// 	return nil, fmt.Errorf("invalid manifest, values of of host_resolution should be resource names")
		// }

		resolutions[host] = v
	}

	return resolutions, nil
}

func getSingleKindPermissions(
	permKind PermissionKind, desc Value, specifiedGlobalPermKinds map[PermissionKind]bool,
	handleCustomType CustomPermissionTypeHandler,
) ([]Permission, error) {

	name := permKind.String()

	var perms []Permission

	var list []Value
	switch v := desc.(type) {
	case *List:
		list = v.GetOrBuildElements(nil)
	case *Object:
		for propKey, propVal := range v.EntryMap() {

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
					if permKind != UsePerm {
						return nil, errors.New("permission 'commands' should be required in the 'use' section of permission")
					}

					newPerms, err := getCommandPermissions(propVal)
					if err != nil {
						return nil, err
					}
					perms = append(perms, newPerms...)
				case "values":
					if permKind != SeePerm {
						if permKind != ReadPerm {
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
		list = []Value{v}
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
			default:
				return nil, fmt.Errorf("invalid manifest, URL has a valid but unsupported scheme '%s'", v.Scheme())
			}

		case URLPattern:
			perms = append(perms, HttpPermission{
				Kind_:  permKind,
				Entity: v,
			})
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

func getModuleParameters(v Value) (ModuleParameters, error) {
	description, ok := v.(*Object)
	if !ok {
		return ModuleParameters{}, fmt.Errorf("invalid manifest, the 'parameters' section should have a value of type object")
	}

	var params ModuleParameters
	restParamFound := false

	err := description.ForEachEntry(func(k string, v Value) error {
		var param moduleParameter

		if IsIndexKey(k) { //positional parameter
			obj, ok := v.(*Object)
			if !ok {
				return errors.New("each positional parameter should be described with an object")
			}

			obj.ForEachEntry(func(propName string, propVal Value) error {
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
				}
				return nil
			})
			if param.pattern == nil {
				return errors.New("missing .pattern in description of positional parameter")
			}

			params.positional = append(params.positional, param)
		} else { // non positional parameyer
			param.name = Identifier(k)

			switch val := v.(type) {
			case *OptionPattern:
				if len(val.Name) == 1 {
					param.singleLetterCliArgName = rune(val.Name[0])
				} else {
					param.cliArgName = val.Name
				}
				param.pattern = val.Value
			case Pattern:
				param.cliArgName = string(param.name)
				param.pattern = val
			case *Object:
				paramDesc := val
				param.cliArgName = k

				paramDesc.ForEachEntry(func(propName string, propVal Value) error {
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

			params.others = append(params.others, param)
		}
		return nil
	})

	if err != nil {
		return ModuleParameters{}, fmt.Errorf("invalid manifest: 'parameters' section: %w", err)
	}

	return params, nil
}

// getDnsPermissions gets a list DNSPermission from an AST node
func getDnsPermissions(permKind PermissionKind, desc Value) ([]Permission, error) {
	var perms []Permission

	if permKind != ReadPerm {
		return nil, fmt.Errorf("invalid manifest, 'dns' is only defined for 'read'")
	}
	dnsReqNodes := make([]Value, 0)

	switch v := desc.(type) {
	case *List:
		dnsReqNodes = append(dnsReqNodes, v.GetOrBuildElements(nil)...)
	default:
		dnsReqNodes = append(dnsReqNodes, v)
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
	tcpRequirementNodes := make([]Value, 0)

	switch v := desc.(type) {
	case *List:
		tcpRequirementNodes = append(tcpRequirementNodes, v.GetOrBuildElements(nil)...)
	default:
		tcpRequirementNodes = append(tcpRequirementNodes, v)
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
	globalReqNodes := make([]Value, 0)

	switch v := desc.(type) {
	case *List:
		globalReqNodes = append(globalReqNodes, v.GetOrBuildElements(nil)...)
	default:
		globalReqNodes = append(globalReqNodes, v)
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
	envReqNodes := make([]Value, 0)

	switch v := desc.(type) {
	case *List:
		envReqNodes = append(envReqNodes, v.GetOrBuildElements(nil)...)
	default:
		envReqNodes = append(envReqNodes, v)
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

	for name, propValue := range topObject.EntryMap() {

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

		for subcmdName, cmdDescPropVal := range cmdDesc.EntryMap() {

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

			for deepSubCmdName, subCmdDescPropVal := range subCmdDesc.EntryMap() {

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
	values := make([]Value, 0)

	switch v := desc.(type) {
	case *List:
		values = append(values, v.GetOrBuildElements(nil)...)
	default:
		values = append(values, v)
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
		"IWD":        INITIAL_WORKING_DIR_PATH,
		"IWD_PREFIX": INITIAL_WORKING_DIR_PATH_PATTERN,
	})
}
