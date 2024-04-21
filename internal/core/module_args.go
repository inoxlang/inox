package core

import "golang.org/x/exp/maps"

var (
	EMPTY_MODULE_ARGS_PATTERN = NewModuleParamsPattern(nil, ModuleParameters{})

	_ IProps  = (*ModuleArgs)(nil)
	_ Pattern = (*ModuleParamsPattern)(nil)
)

// ModuleArgs contains the arguments passed to a module, ModuleArgs implements Value.
type ModuleArgs struct {
	pattern *ModuleParamsPattern
	values  map[string]Value
}

func NewEmptyModuleArgs() *ModuleArgs {
	return &ModuleArgs{pattern: EMPTY_MODULE_ARGS_PATTERN}
}

func NewModuleArgs(entries map[string]Value) *ModuleArgs {
	types := map[string]Pattern{}

	for k := range entries {
		types[k] = ANYVAL_PATTERN
	}

	return &ModuleArgs{
		pattern: NewModuleParamsPattern(types, ModuleParameters{}),
		values:  entries,
	}
}

func (s *ModuleArgs) Prop(ctx *Context, name string) Value {
	return s.values[name]
}

func (s *ModuleArgs) PropertyNames(*Context) []string {
	return maps.Keys(s.values)
}

func (s *ModuleArgs) SetProp(ctx *Context, name string, value Value) error {
	_, ok := s.values[name]
	if !ok {
		return FormatErrPropertyDoesNotExist(name, s)
	}

	s.values[name] = value
	return nil
}

func (s *ModuleArgs) ValueMap() map[string]Value {
	return maps.Clone(s.values)
}

// A ModuleParamsPattern is pattern for ModuleArgs values.
type ModuleParamsPattern struct {
	types        map[string]Pattern
	sourceParams ModuleParameters //may be not set

	NotCallablePatternMixin
}

func NewModuleParamsPattern(
	types map[string]Pattern,
	sourceParams ModuleParameters,
) *ModuleParamsPattern {
	return &ModuleParamsPattern{
		types:        types,
		sourceParams: sourceParams,
	}
}

func (p *ModuleParamsPattern) hasSourceParams() bool {
	return p.sourceParams.positional == nil && p.sourceParams.others == nil
}

func (p *ModuleParamsPattern) Test(ctx *Context, v Value) bool {
	_struct, ok := v.(*ModuleArgs)
	return ok && _struct.pattern == p
}

func (*ModuleParamsPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}
