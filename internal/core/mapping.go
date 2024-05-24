package core

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/symbolic"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

type Mapping struct {
	//key representation (pattern or value) => keyt
	keys map[string]Serializable

	//key representation (pattern or value) => value
	preComputedStaticEntryValues map[string]Serializable

	//key representation (pattern or value) => value
	staticEntries map[string]*ast.StaticMappingEntry

	//key representation (pattern or value) => value
	dynamicEntries map[string]*ast.DynamicMappingEntry

	//pattern => key representation
	patterns []struct {
		string
		Pattern
	}

	shared          atomic.Bool
	staticCheck     bool //TODO: remove at some point, static checks should be mandatory
	staticData      *staticcheck.MappingData
	capturedGlobals map[string]Value
}

func NewMapping(expr *ast.MappingExpression, state *GlobalState) (*Mapping, error) {

	mapping := &Mapping{
		keys:                         map[string]Serializable{},
		dynamicEntries:               map[string]*ast.DynamicMappingEntry{},
		preComputedStaticEntryValues: map[string]Serializable{},
		patterns: []struct {
			string
			Pattern
		}{},
	}

	if state.StaticCheckData != nil {
		staticData := state.StaticCheckData.GetMappingData(expr)
		mapping.staticData = staticData
		mapping.staticCheck = true
	}

	for _, entry := range expr.Entries {

		switch e := entry.(type) {
		case *ast.StaticMappingEntry:
			var key Value
			var err error

			if valueLit, ok := e.Key.(ast.SimpleValueLiteral); ok && !utils.Implements[*ast.IdentifierLiteral](valueLit) {
				key, err = EvalSimpleValueLiteral(valueLit, state)
			} else {
				//TODO: check has representation
				key, err = resolvePattern(e.Key, state)
			}

			if err != nil {
				return nil, err
			}
			repr := MustGetJSONRepresentationWithConfig(key.(Serializable), state.Ctx, JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG})
			mapping.keys[repr] = key.(Serializable)

			if valueLit, ok := e.Value.(ast.SimpleValueLiteral); ok && !utils.Implements[*ast.IdentifierLiteral](valueLit) {
				v, err := EvalSimpleValueLiteral(valueLit, state)
				if err != nil {
					return nil, err
				}
				mapping.preComputedStaticEntryValues[repr] = v
			} else {
				if mapping.staticEntries == nil {
					mapping.staticEntries = map[string]*ast.StaticMappingEntry{}
				}
				mapping.staticEntries[repr] = e
			}

			if patt, ok := key.(Pattern); ok {
				mapping.patterns = append(mapping.patterns, struct {
					string
					Pattern
				}{repr, patt})
			}

		case *ast.DynamicMappingEntry:
			var key Value
			var err error

			if valueLit, ok := e.Key.(ast.SimpleValueLiteral); ok {
				key, err = EvalSimpleValueLiteral(valueLit, state)
			} else {
				//TODO: check has representation
				key, err = resolvePattern(e.Key, state)
			}

			if err != nil {
				return nil, err
			}

			repr := MustGetJSONRepresentationWithConfig(key.(Serializable), state.Ctx, JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG})
			mapping.keys[repr] = key.(Serializable)
			mapping.dynamicEntries[repr] = e

			if patt, ok := key.(Pattern); ok {
				mapping.patterns = append(mapping.patterns, struct {
					string
					Pattern
				}{repr, patt})
			}
		}
	}

	return mapping, nil
}

func (m *Mapping) Compute(ctx *Context, key Serializable) Value {
	repr := MustGetJSONRepresentationWithConfig(key.(Serializable), ctx, JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG})

	if _, ok := key.(Pattern); ok {
		panic(errors.New("mapping.compute: cannot compute value for a pattern"))
	}
	shared := m.shared.Load()

	computeStaticKeyEntryValue := func(entry *ast.StaticMappingEntry) Value {
		callingState := ctx.MustGetClosestState()

		//TODO: optimize
		var globalConstants map[string]Value
		if !shared {
			globalConstants = callingState.Globals.Constants()
		}

		evalState := NewTreeWalkState(callingState.Ctx.BoundChild(), globalConstants)
		defer evalState.Global.Ctx.CancelGracefully()

		// set global variables
		if shared {
			for k, v := range m.capturedGlobals {
				evalState.Global.Globals.Set(k, v)
			}
		} else {
			callingState.Globals.Foreach(func(name string, v Value, isStartConstant bool) error {
				if !isStartConstant {
					evalState.Global.Globals.Set(name, v)
				}
				return nil
			})
		}

		evalState.Global.Out = callingState.Out
		evalState.Global.Logger = callingState.Logger
		evalState.Global.OutputFieldsInitialized.Store(true)

		val, err := TreeWalkEval(entry.Value, evalState)
		if err != nil {
			//log.Println(err)
			callingState.Logger.Print("mapping.compute: ", err)
			return Nil
		}
		return val
	}

	if v, ok := m.preComputedStaticEntryValues[repr]; ok {
		if v == nil {
			entry := m.staticEntries[repr]
			return computeStaticKeyEntryValue(entry)
		}
		return v
	}

	computeDynKeyEntryValue := func(patt Pattern, entry *ast.DynamicMappingEntry) Value {
		callingState := ctx.MustGetClosestState()
		varName := entry.KeyVar.(*ast.IdentifierLiteral).Name

		var globalConstants map[string]Value
		if !shared {
			globalConstants = callingState.Globals.Constants()
		}

		evalState := NewTreeWalkState(callingState.Ctx.BoundChild(), globalConstants)
		defer evalState.Global.Ctx.CancelGracefully()

		// set global variables
		if shared {
			for k, v := range m.capturedGlobals {
				evalState.Global.Globals.Set(k, v)
			}
		} else {
			callingState.Globals.Foreach(func(name string, v Value, isStartConstant bool) error {
				if !isStartConstant {
					evalState.Global.Globals.Set(name, v)
				}
				return nil
			})
		}

		evalState.Global.Out = callingState.Out
		evalState.Global.Logger = callingState.Logger
		evalState.Global.OutputFieldsInitialized.Store(true)

		// state.entryComputeFn = func(k Value) (Value, error) {

		// }

		evalState.SetGlobal(varName, key, GlobalConst)

		if patt != nil && entry.GroupMatchingVariable != nil {
			name := entry.GroupMatchingVariable.(*ast.IdentifierLiteral).Name
			groups, ok, err := patt.(GroupPattern).MatchGroups(ctx, key)
			if err != nil {
				panic(err)
			}

			var obj *Object
			if ok {
				obj = NewObjectFromMap(groups, evalState.Global.Ctx)
			} else {
				obj = NewObjectFromMap(ValMap{"0": key}, evalState.Global.Ctx)
			}
			evalState.SetGlobal(name, obj, GlobalConst)
		}

		v, err := TreeWalkEval(entry.ValueComputation, evalState)
		if err != nil {
			callingState.Logger.Print("mapping.compute: ", err)
			return Nil
		}
		return v
	}

	if entry, ok := m.staticEntries[repr]; ok {
		return computeStaticKeyEntryValue(entry)
	}

	if entry, ok := m.dynamicEntries[repr]; ok {
		return computeDynKeyEntryValue(nil, entry)
	}

	for _, info := range m.patterns {
		keyRepr := info.string
		patt := info.Pattern
		if patt.Test(ctx, key) {
			if v, ok := m.preComputedStaticEntryValues[keyRepr]; ok {
				return v
			}
			if entry, ok := m.staticEntries[keyRepr]; ok {
				return computeStaticKeyEntryValue(entry)
			}
			if entry, ok := m.dynamicEntries[keyRepr]; ok {
				return computeDynKeyEntryValue(patt, entry)
			}
		}
	}

	return Nil
}

func (m *Mapping) IsSharable(originState *GlobalState) (bool, string) {
	if !m.staticCheck {
		return false, "mapping is not sharable because static data is missing"
	}
	if m.staticData != nil && len(m.staticData.ReferencedGlobals()) > 0 {
		staticData := m.staticData
		for _, name := range staticData.ReferencedGlobals() {
			if ok, expl := IsSharableOrClonable(originState.Globals.Get(name), originState); !ok { // TODO: fix: globals could change after call to .IsSharable()
				return false, fmt.Sprintf("mapping is not sharable because referenced global %s is not sharable/clonable: %s", name, expl)
			}
		}
	}
	return true, ""
}

func (m *Mapping) Share(originState *GlobalState) {
	if m.shared.CompareAndSwap(false, true) {
		if m.staticData != nil && len(m.staticData.ReferencedGlobals()) > 0 {
			referencedGlobals := m.staticData.ReferencedGlobals()

			m.capturedGlobals = make(map[string]Value, len(referencedGlobals))
			for _, name := range referencedGlobals {
				val := originState.Globals.Get(name)
				val, err := ShareOrClone(val, originState)
				if err != nil {
					panic(err)
				}
				m.capturedGlobals[name] = val
			}
		}
	}
}

func (m *Mapping) IsShared() bool {
	return m.shared.Load()
}

func (*Mapping) SmartLock(state *GlobalState) {

}

func (*Mapping) SmartUnlock(state *GlobalState) {

}

func (m *Mapping) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "compute":
		return WrapGoMethod(m.Compute), true
	}
	return nil, false
}

func (m *Mapping) Prop(ctx *Context, name string) Value {
	method, ok := m.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, m))
	}
	return method
}

func (*Mapping) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*Mapping) PropertyNames(ctx *Context) []string {
	return symbolic.MAPPING_PROPNAMES
}
