package containers

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

var (
	ErrMapEntryListShouldHaveEvenLength = errors.New(`flat map entry list should have an even length: ["k1", 1,  "k2", 2]`)
	ErrMapCanOnlyContainKeysWithFastId  = errors.New("a Map can only contain keys having a fast id")
)

func NewMap(ctx *core.Context, flatEntries *core.List) *Map {

	map_ := &Map{
		values: make(map[core.FastId]core.Value),
		keys:   make(map[core.FastId]core.Value),
	}

	if flatEntries.Len()%2 != 0 {
		panic(ErrMapEntryListShouldHaveEvenLength)
	}

	halfEntryCount := flatEntries.Len()
	for i := 0; i < halfEntryCount; i += 2 {

		key := flatEntries.At(ctx, i)
		value := flatEntries.At(ctx, i+1)

		id, ok := core.FastIdOf(ctx, key)
		if !ok {
			panic(ErrMapCanOnlyContainKeysWithFastId)
		}
		map_.values[id] = value
		map_.keys[id] = key
	}

	return map_
}

type Map struct {
	values map[core.FastId]core.Value
	keys   map[core.FastId]core.Value
}

func (m *Map) Insert(ctx *core.Context, k, v core.Value) {
	id, ok := core.FastIdOf(ctx, k)
	if !ok {
		panic(ErrMapCanOnlyContainKeysWithFastId)
	}
	if _, ok := m.values[id]; ok {
		panic(fmt.Errorf("cannot insert entry with key %s, it already exists", core.Stringify(k, ctx)))
	}
	m.values[id] = v
}

func (m *Map) Update(ctx *core.Context, k, v core.Value) {
	id, ok := core.FastIdOf(ctx, k)
	if !ok {
		panic(ErrMapCanOnlyContainKeysWithFastId)
	}
	if _, ok := m.values[id]; !ok {
		panic(fmt.Errorf("cannot update entry with key %s, it does not exist", core.Stringify(k, ctx)))
	}
	m.values[id] = v
}

func (m *Map) Remove(ctx *core.Context, k core.Value) {
	id, ok := core.FastIdOf(ctx, k)
	if !ok {
		panic(ErrMapCanOnlyContainKeysWithFastId)
	}
	delete(m.values, id)
}

func (m *Map) Get(ctx *core.Context, k core.Value) core.Value {
	id, ok := core.FastIdOf(ctx, k)
	if !ok {
		panic(ErrMapCanOnlyContainKeysWithFastId)
	}
	v, ok := m.values[id]
	if !ok {
		return core.Nil
	}
	return v
}

func (m *Map) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "insert":
		return core.WrapGoMethod(m.Insert), true
	case "update":
		return core.WrapGoMethod(m.Update), true
	case "remove":
		return core.WrapGoMethod(m.Remove), true
	case "get":
		return core.WrapGoMethod(m.Get), true
	}
	return nil, false
}

func (m *Map) Prop(ctx *core.Context, name string) core.Value {
	method, ok := m.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, m))
	}
	return method
}

func (*Map) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Map) PropertyNames(ctx *core.Context) []string {
	return coll_symbolic.MAP_PROPNAMES
}
