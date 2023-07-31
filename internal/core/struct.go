package core

import (
	"github.com/oklog/ulid/v2"
)

var (
	_ IProps  = (*Struct)(nil)
	_ Pattern = (*StructPattern)(nil)

	ANON_EMPTY_STRUCT_TYPE = NewStructPattern("", ulid.Make(), nil, nil)
)

type Struct struct {
	structType *StructPattern
	values     []Value
}

func NewEmptyStruct() *Struct {
	return &Struct{structType: ANON_EMPTY_STRUCT_TYPE}
}
func NewStructFromMap(fields map[string]Value) *Struct {
	var keys []string
	var patterns []Pattern
	var values []Value

	for k, v := range fields {
		keys = append(keys, k)
		patterns = append(patterns, ANYVAL_PATTERN)
		values = append(values, v)
	}
	return &Struct{
		structType: NewStructPattern("", ulid.Make(), keys, patterns),
		values:     values,
	}
}

func (s *Struct) Prop(ctx *Context, name string) Value {
	index, ok := s.structType.indexOfField(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return s.values[index]
}

func (s *Struct) PropertyNames(*Context) []string {
	return s.structType.keys
}

func (s *Struct) SetProp(ctx *Context, name string, value Value) error {
	index, ok := s.structType.indexOfField(name)
	if !ok {
		return FormatErrPropertyDoesNotExist(name, s)
	}

	s.values[index] = value
	return nil
}

func (s *Struct) ValueMap() map[string]Value {
	valueMap := map[string]Value{}
	for index, fieldVal := range s.values {
		valueMap[s.structType.keys[index]] = fieldVal
	}
	return valueMap
}

func (s *Struct) ForEachField(fn func(fieldName string, fieldValue Value) error) error {
	for i, v := range s.values {
		fieldName := s.structType.keys[i]
		if err := fn(fieldName, v); err != nil {
			return err
		}
	}
	return nil
}

// A StructPattern represents a struct type, it is nominal.
type StructPattern struct {
	name   string //empty if anonymous
	tempId ulid.ULID
	keys   []string
	types  []Pattern

	NotCallablePatternMixin
}

func NewStructPattern(
	name string,
	tempId ulid.ULID,
	keys []string,
	types []Pattern,
) *StructPattern {
	return &StructPattern{
		name:   name,
		tempId: tempId,
		keys:   keys,
		types:  types,
	}
}

func (p *StructPattern) Test(ctx *Context, v Value) bool {
	_struct, ok := v.(*Struct)
	return ok && _struct.structType == p
}

func (*StructPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (s *StructPattern) typeOfField(name string) (Pattern, bool) {
	ind, ok := s.indexOfField(name)
	if !ok {
		return nil, false
	}
	return s.types[ind], true
}

func (s *StructPattern) indexOfField(name string) (int, bool) {
	for index, key := range s.keys {
		if key == name {
			return index, true
		}
	}
	return -1, false
}
