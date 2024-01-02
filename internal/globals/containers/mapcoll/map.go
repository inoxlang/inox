package mapcoll

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
)

var (
	ErrMapEntryListShouldHaveEvenLength = errors.New(`flat map entry list should have an even length: ["k1", 1,  "k2", 2]`)
	ErrMapCanOnlyContainKeysWithFastId  = errors.New("a Map can only contain keys having a fast id")
)

func NewMap(ctx *core.Context, flatEntries *core.List) *Map {

	map_ := &Map{
		values: make(map[core.TransientID]core.Value),
		keys:   make(map[core.TransientID]core.Value),
	}

	if flatEntries.Len()%2 != 0 {
		panic(ErrMapEntryListShouldHaveEvenLength)
	}

	halfEntryCount := flatEntries.Len()
	for i := 0; i < halfEntryCount; i += 2 {

		key := flatEntries.At(ctx, i)
		value := flatEntries.At(ctx, i+1)

		id, ok := core.TransientIdOf(key)
		if !ok {
			panic(ErrMapCanOnlyContainKeysWithFastId)
		}
		map_.values[id] = value
		map_.keys[id] = key
	}

	return map_
}

type Map struct {
	values map[core.TransientID]core.Value
	keys   map[core.TransientID]core.Value
}

func (m *Map) Insert(ctx *core.Context, k, v core.Value) {
	id, ok := core.TransientIdOf(k)
	if !ok {
		panic(ErrMapCanOnlyContainKeysWithFastId)
	}
	if _, ok := m.values[id]; ok {
		panic(fmt.Errorf("cannot insert entry with key %s, it already exists", core.Stringify(k, ctx)))
	}
	m.values[id] = v
}

func (m *Map) Set(ctx *core.Context, k, v core.Value) {
	id, ok := core.TransientIdOf(k)
	if !ok {
		panic(ErrMapCanOnlyContainKeysWithFastId)
	}
	if _, ok := m.values[id]; !ok {
		panic(fmt.Errorf("cannot update entry with key %s, it does not exist", core.Stringify(k, ctx)))
	}
	m.values[id] = v
}

func (m *Map) Remove(ctx *core.Context, k core.Value) {
	id, ok := core.TransientIdOf(k)
	if !ok {
		panic(ErrMapCanOnlyContainKeysWithFastId)
	}
	delete(m.values, id)
}

func (m *Map) Get(ctx *core.Context, k core.Value) core.Value {
	id, ok := core.TransientIdOf(k)
	if !ok {
		panic(ErrMapCanOnlyContainKeysWithFastId)
	}
	v, ok := m.values[id]
	if !ok {
		return core.Nil
	}
	return v
}
