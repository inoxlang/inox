package internal

import (
	"errors"
	"fmt"
	"sync/atomic"

	parse "github.com/inoxlang/inox/internal/parse"
)

type Mapping struct {
	NoReprMixin
	//key representation (pattern or value) => key
	keys map[string]Value

	//key representation (pattern or value) => value
	preComputedStaticEntryValues map[string]Value

	//key representation (pattern or value) => value
	staticEntries map[string]*parse.StaticMappingEntry

	//key representation (pattern or value) => value
	dynamicEntries map[string]*parse.DynamicMappingEntry

	//pattern => key representation
	patterns []struct {
		string
		Pattern
	}

	shared          atomic.Bool
	staticCheck     bool //TODO: remove at some point, static checks should be mandatory
	staticData      *MappingStaticData
	capturedGlobals map[string]Value
}

func NewMapping(expr *parse.MappingExpression, state *GlobalState) (*Mapping, error) {

	mapping := &Mapping{
		keys:                         map[string]Value{},
		dynamicEntries:               map[string]*parse.DynamicMappingEntry{},
		preComputedStaticEntryValues: map[string]Value{},
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
		case *parse.StaticMappingEntry:
			var key Value
			var err error

			if valueLit, ok := e.Key.(parse.SimpleValueLiteral); ok && !parse.NodeIs(valueLit, (*parse.IdentifierLiteral)(nil)) {
				key, err = evalSimpleValueLiteral(valueLit, state)
			} else {
				//TODO: check has representation
				key, err = resolvePattern(e.Key, state)
			}

			if err != nil {
				return nil, err
			}

			repr := string(GetRepresentation(key, state.Ctx))
			mapping.keys[repr] = key

			if valueLit, ok := e.Value.(parse.SimpleValueLiteral); ok && !parse.NodeIs(valueLit, (*parse.IdentifierLiteral)(nil)) {
				v, err := evalSimpleValueLiteral(valueLit, state)
				if err != nil {
					return nil, err
				}
				mapping.preComputedStaticEntryValues[repr] = v
			} else {
				if mapping.staticEntries == nil {
					mapping.staticEntries = map[string]*parse.StaticMappingEntry{}
				}
				mapping.staticEntries[repr] = e
			}

			if patt, ok := key.(Pattern); ok {
				mapping.patterns = append(mapping.patterns, struct {
					string
					Pattern
				}{repr, patt})
			}

		case *parse.DynamicMappingEntry:
			var key Value
			var err error

			if valueLit, ok := e.Key.(parse.SimpleValueLiteral); ok {
				key, err = evalSimpleValueLiteral(valueLit, state)
			} else {
				//TODO: check has representation
				key, err = resolvePattern(e.Key, state)
			}

			if err != nil {
				return nil, err
			}

			repr := string(GetRepresentation(key, state.Ctx))
			mapping.keys[repr] = key
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

func (m *Mapping) Compute(ctx *Context, key Value) Value {
	repr := string(GetRepresentation(key, ctx))

	if _, ok := key.(Pattern); ok {
		panic(errors.New("mapping.compute: cannot compute value for a pattern"))
	}

	shared := m.shared.Load()

	computeStaticKeyEntryValue := func(entry *parse.StaticMappingEntry) Value {
		callingState := ctx.GetClosestState()

		var globalVars map[string]Value

		if shared {
			globalVars = m.capturedGlobals
		} else {
			globalVars = callingState.Globals.Entries()
		}

		//TODO: optimize
		evalState := NewTreeWalkState(callingState.Ctx.BoundChild(), globalVars)
		evalState.Global.Out = callingState.Out
		evalState.Global.Logger = callingState.Logger

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

	computeDynKeyEntryValue := func(patt Pattern, entry *parse.DynamicMappingEntry) Value {
		callingState := ctx.GetClosestState()
		varName := entry.KeyVar.(*parse.IdentifierLiteral).Name

		var globalVars map[string]Value // this value should be modified as it could be read by several goroutines

		if shared {
			globalVars = m.capturedGlobals
		} else {
			globalVars = callingState.Globals.Entries()
		}

		//TODO: optimize
		state := NewTreeWalkState(ctx.BoundChild(), globalVars)
		state.Global.Out = callingState.Out
		state.Global.Logger = callingState.Logger

		// state.entryComputeFn = func(k Value) (Value, error) {

		// }

		state.SetGlobal(varName, key, GlobalConst)

		if patt != nil && entry.GroupMatchingVariable != nil {
			name := entry.GroupMatchingVariable.(*parse.IdentifierLiteral).Name
			groups, ok, err := patt.(GroupPattern).MatchGroups(ctx, key)
			if err != nil {
				panic(err)
			}

			var obj *Object
			if ok {
				obj = NewObjectFromMap(groups, state.Global.Ctx)
			} else {
				obj = NewObjectFromMap(ValMap{"0": key}, state.Global.Ctx)
			}
			state.SetGlobal(name, obj, GlobalConst)
		}

		v, err := TreeWalkEval(entry.ValueComputation, state)
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
		return false, fmt.Sprintf("mapping is not sharable because static data is missing")
	}
	if m.staticData != nil && len(m.staticData.referencedGlobals) > 0 {
		staticData := m.staticData
		for _, name := range staticData.referencedGlobals {
			if ok, expl := IsSharable(originState.Globals.Get(name), originState); !ok { // TODO: fix: globals could change after call to .IsSharable()
				return false, fmt.Sprintf("mapping is not sharable because referenced global %s is not sharable: %s", name, expl)
			}
		}
	}
	return true, ""
}

func (m *Mapping) Share(originState *GlobalState) {
	if m.shared.CompareAndSwap(false, true) {
		if m.staticData != nil && len(m.staticData.referencedGlobals) > 0 {
			staticData := m.staticData
			m.capturedGlobals = make(map[string]Value, len(staticData.referencedGlobals))
			for _, name := range staticData.referencedGlobals {
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

func (*Mapping) ForceLock() {

}
func (*Mapping) ForceUnlock() {

}

func (m *Mapping) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "compute":
		return &GoFunction{fn: m.Compute}, true
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
	return []string{"compute"}
}
