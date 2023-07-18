package core

import (
	"github.com/oklog/ulid/v2"
)

var (
	_ IProps  = (*Struct)(nil)
	_ Pattern = (*StructPattern)(nil)
)

type Struct struct {
	structType *StructPattern
	values     []Value
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
