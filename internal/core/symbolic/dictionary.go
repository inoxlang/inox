package symbolic

import (
	"errors"
	"sort"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"golang.org/x/exp/maps"
)

type Dictionary struct {
	//if nil, matches any dictionary, map (approximate key representation) -> value
	entries map[string]Serializable
	//map (approximate key representation) -> key
	keys map[string]Serializable

	SerializableMixin
	ClonableSerializableMixin

	UnassignablePropsMixin
}

func NewAnyDictionary() *Dictionary {
	return &Dictionary{}
}

func NewUnitializedDictionary() *Dictionary {
	return &Dictionary{}
}

func NewDictionary(entries map[string]Serializable, keys map[string]Serializable) *Dictionary {
	if entries == nil {
		entries = map[string]Serializable{}
	}
	return &Dictionary{
		entries: entries,
		keys:    keys,
	}
}

func InitializeDictionary(d *Dictionary, entries map[string]Serializable, keys map[string]Serializable) {
	if d.entries != nil || d.keys != nil {
		panic(errors.New("dictionary is already initialized"))
	}
	if entries == nil {
		entries = map[string]Serializable{}
	}
	d.entries = entries
	d.keys = keys
}

func (dict *Dictionary) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherDict, ok := v.(*Dictionary)
	if !ok {
		return false
	}

	if dict.entries == nil {
		return true
	}

	if len(dict.entries) != len(otherDict.entries) || otherDict.entries == nil {
		return false
	}

	for i, e := range dict.entries {
		if !e.Test(otherDict.entries[i], state) {
			return false
		}
	}
	return true
}

func (dict *Dictionary) IsConcretizable() bool {
	//TODO: support constraints

	if dict.entries == nil {
		return false
	}

	for _, v := range dict.entries {
		if !IsConcretizable(v) {
			return false
		}
	}

	for _, key := range dict.entries {
		if !IsConcretizable(key) {
			return false
		}
	}

	return true
}

func (dict *Dictionary) Concretize(ctx ConcreteContext) any {
	if !dict.IsConcretizable() {
		panic(ErrNotConcretizable)
	}

	concreteValues := make([]any, len(dict.entries))
	concreteKeys := make([]any, len(dict.entries))

	i := 0
	for keyRepr, value := range dict.entries {
		concreteValue := utils.Must(Concretize(value, ctx))
		concreteKey := utils.Must(Concretize(dict.keys[keyRepr], ctx))

		concreteValues[i] = concreteValue
		concreteKeys[i] = concreteKey
		i++
	}
	return extData.ConcreteValueFactories.CreateDictionary(concreteKeys, concreteValues, ctx)
}

func (dict *Dictionary) Entries() map[string]Serializable {
	return maps.Clone(dict.entries)
}

func (dict *Dictionary) Keys() map[string]Serializable {
	return maps.Clone(dict.keys)
}

func (dict *Dictionary) hasKey(keyRepr string) bool {
	if dict.entries == nil {
		return true
	}
	_, ok := dict.keys[keyRepr]
	return ok
}

func (dict *Dictionary) get(keyRepr string) (Value, bool) {
	if dict.entries == nil {
		return ANY, true
	}
	v, ok := dict.entries[keyRepr]
	return v, ok
}

func (dict *Dictionary) Get(ctx *Context, key Serializable) (Value, *Bool) {
	return ANY_SERIALIZABLE, ANY_BOOL
}

func (dict *Dictionary) SetValue(ctx *Context, key, value Serializable) {

}

func (dict *Dictionary) key() Value {
	if dict.entries != nil {
		if len(dict.entries) == 0 {
			return ANY
		}
		var keys []Value
		for _, k := range dict.keys {
			keys = append(keys, k)
		}
		return AsSerializableChecked(joinValues(keys))
	}
	return ANY
}

func (dict *Dictionary) ForEachEntry(fn func(key Serializable, k string, v Value) error) error {
	keyStrings := maps.Keys(dict.entries)
	sort.Strings(keyStrings)

	for _, keyString := range keyStrings {
		if err := fn(dict.keys[keyString], keyString, dict.entries[keyString]); err != nil {
			return err
		}
	}
	return nil
}

func (dict *Dictionary) AllKeysConcretizable() bool {
	for _, k := range dict.keys {
		if !IsConcretizable(k) {
			return false
		}
	}
	return true
}

func (dict *Dictionary) Prop(name string) Value {
	switch name {
	case "get":
		return WrapGoMethod(dict.Get)
	case "set":
		return WrapGoMethod(dict.SetValue)
	default:
		panic(FormatErrPropertyDoesNotExist(name, dict))
	}
}

func (dict *Dictionary) PropertyNames() []string {
	return DICTIONARY_PROPNAMES
}

func (dict *Dictionary) IteratorElementKey() Value {
	return dict.key()
}

func (dict *Dictionary) IteratorElementValue() Value {
	return ANY
}

func (dict *Dictionary) WatcherElement() Value {
	return ANY
}

func (dict *Dictionary) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w = w.IncrDepth()

	if dict.entries != nil {
		if w.Depth > config.MaxDepth && len(dict.entries) > 0 {
			w.WriteString(":{(...)}")
			return
		}

		w.WriteString(":{")

		var keys []string
		for k := range dict.entries {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for i, k := range keys {
			if !config.Compact {
				w.WriteEndOfLine()
				w.WriteInnerIndent()
			}

			//key
			if config.Colorize {
				w.WriteBytes(config.Colors.StringLiteral)

			}
			w.WriteString(k)

			if config.Colorize {
				w.WriteAnsiReset()
			}

			//colon
			w.WriteString(": ")

			//value
			v := dict.entries[k]

			v.PrettyPrint(w.IncrIndent(), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				w.WriteString(": ")
			}
		}

		if !config.Compact && len(keys) > 0 {
			w.WriteEndOfLine()
			w.WriteOuterIndent()
		}
		w.WriteByte('}')
		return
	}
	w.WriteName("dictionary")
}

func (d *Dictionary) WidestOfType() Value {
	return &Dictionary{}
}
