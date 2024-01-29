package core

var (
	EMPTY_MODULE_ARGS_PATTERN = NewModuleParamsPattern(nil, nil)

	_ IProps  = (*ModuleArgs)(nil)
	_ Pattern = (*ModuleParamsPattern)(nil)
)

// ModuleArgs contains the arguments passed to a module, ModuleArgs implements Value.
type ModuleArgs struct {
	pattern *ModuleParamsPattern
	values  []Value
}

func NewEmptyModuleArgs() *ModuleArgs {
	return &ModuleArgs{pattern: EMPTY_MODULE_ARGS_PATTERN}
}

func NewModuleArgs(fields map[string]Value) *ModuleArgs {
	var keys []string
	var patterns []Pattern
	var values []Value

	for k, v := range fields {
		keys = append(keys, k)
		patterns = append(patterns, ANYVAL_PATTERN)
		values = append(values, v)
	}
	return &ModuleArgs{
		pattern: NewModuleParamsPattern(keys, patterns),
		values:  values,
	}
}

func (s *ModuleArgs) Prop(ctx *Context, name string) Value {
	index, ok := s.pattern.indexOfField(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return s.values[index]
}

func (s *ModuleArgs) PropertyNames(*Context) []string {
	return s.pattern.keys
}

func (s *ModuleArgs) SetProp(ctx *Context, name string, value Value) error {
	index, ok := s.pattern.indexOfField(name)
	if !ok {
		return FormatErrPropertyDoesNotExist(name, s)
	}

	s.values[index] = value
	return nil
}

func (s *ModuleArgs) ValueMap() map[string]Value {
	valueMap := map[string]Value{}
	for index, fieldVal := range s.values {
		valueMap[s.pattern.keys[index]] = fieldVal
	}
	return valueMap
}

func (s *ModuleArgs) ForEachField(fn func(fieldName string, fieldValue Value) error) error {
	for i, v := range s.values {
		fieldName := s.pattern.keys[i]
		if err := fn(fieldName, v); err != nil {
			return err
		}
	}
	return nil
}

// A ModuleParamsPattern is pattern for ModuleArgs values.
type ModuleParamsPattern struct {
	keys  []string
	types []Pattern

	NotCallablePatternMixin
}

func NewModuleParamsPattern(
	keys []string,
	types []Pattern,
) *ModuleParamsPattern {
	return &ModuleParamsPattern{
		keys:  keys,
		types: types,
	}
}

func (p *ModuleParamsPattern) Test(ctx *Context, v Value) bool {
	_struct, ok := v.(*ModuleArgs)
	return ok && _struct.pattern == p
}

func (*ModuleParamsPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (s *ModuleParamsPattern) typeOfField(name string) (Pattern, bool) {
	ind, ok := s.indexOfField(name)
	if !ok {
		return nil, false
	}
	return s.types[ind], true
}

func (s *ModuleParamsPattern) indexOfField(name string) (int, bool) {
	for index, key := range s.keys {
		if key == name {
			return index, true
		}
	}
	return -1, false
}
