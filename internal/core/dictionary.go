package core

import (
	"errors"
	"fmt"
	"sync"

	"github.com/inoxlang/inox/internal/core/symbolic"
)

// A Dictionnary maps representable values (keys) to any values, Dictionar implements Value.
type Dictionary struct {
	entries map[string]Serializable
	keys    map[string]Serializable

	lock                   sync.Mutex // exclusive access for initializing .watchers & .mutationCallbacks
	watchingDepth          WatchingDepth
	watchers               *ValueWatchers
	mutationCallbacks      *MutationCallbacks
	entryMutationCallbacks map[string]CallbackHandle
}

func NewDictionary(entries ValMap) *Dictionary {
	dict := &Dictionary{
		entries: map[string]Serializable{},
		keys:    map[string]Serializable{},
	}
	for keyRepresentation, v := range entries {
		dict.entries[keyRepresentation] = v
		key, err := ParseJSONRepresentation(nil, keyRepresentation, nil)
		if err != nil {
			panic(fmt.Errorf("invalid key representation for dictionary: %q", keyRepresentation))
		}
		dict.keys[keyRepresentation] = key
	}

	return dict
}

func NewDictionaryFromKeyValueLists(keys []Serializable, values []Serializable, ctx *Context) *Dictionary {
	if len(keys) != len(values) {
		panic(errors.New("the key list should have the same length as the value list"))
	}

	dict := &Dictionary{
		entries: map[string]Serializable{},
		keys:    map[string]Serializable{},
	}

	for i, key := range keys {
		keyRepr := dict.getKeyRepr(ctx, key)
		dict.entries[keyRepr] = values[i]
		dict.keys[keyRepr] = key
	}

	return dict
}

func (d *Dictionary) ForEachEntry(ctx *Context, fn func(keyRepr string, key Serializable, v Serializable) error) error {
	for keyRepr, val := range d.entries {
		key := d.keys[keyRepr]
		if err := fn(keyRepr, key, val); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dictionary) getKeyRepr(ctx *Context, key Serializable) string {
	return MustGetJSONRepresentationWithConfig(key, ctx, JSONSerializationConfig{ReprConfig: ALL_VISIBLE_REPR_CONFIG})
}

func (d *Dictionary) Value(ctx *Context, key Serializable) (Value, Bool) {
	v, ok := d.entries[d.getKeyRepr(ctx, key)]
	return v, Bool(ok)
}

func (d *Dictionary) SetValue(ctx *Context, key, value Serializable) {
	keyRepr := d.getKeyRepr(ctx, key)

	prevValue, alreadyPresent := d.entries[keyRepr]
	d.entries[keyRepr] = value
	if alreadyPresent {
		if d.entryMutationCallbacks != nil {
			d.removeEntryMutationCallbackNoLock(ctx, keyRepr, prevValue)
			if err := d.addEntryMutationCallbackNoLock(ctx, keyRepr, value); err != nil {
				panic(fmt.Errorf("failed to add mutation callback for updated dictionary entry %s: %w", keyRepr, err))
			}
		}

		mutation := NewUpdateEntryMutation(ctx, key, value, ShallowWatching, Path("/"+keyRepr))

		//inform watchers & microtasks about the update
		d.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)

		if d.mutationCallbacks != nil {
			d.mutationCallbacks.CallMicrotasks(ctx, mutation)
		}
	} else {
		if err := d.addEntryMutationCallbackNoLock(ctx, keyRepr, value); err != nil {
			panic(fmt.Errorf("failed to add mutation callback for added dictionary entry %s: %w", keyRepr, err))
		}

		mutation := NewAddEntryMutation(ctx, key, value, ShallowWatching, Path("/"+keyRepr))

		d.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)

		if d.mutationCallbacks != nil {
			d.mutationCallbacks.CallMicrotasks(ctx, mutation)
		}
	}

}

func (d *Dictionary) Prop(ctx *Context, name string) Value {
	switch name {
	case "get":
		return WrapGoMethod(d.Value)
	case "set":
		return WrapGoMethod(d.SetValue)
	default:
		panic(FormatErrPropertyDoesNotExist(name, d))
	}
}

func (*Dictionary) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*Dictionary) PropertyNames(ctx *Context) []string {
	return symbolic.DICTIONARY_PROPNAMES
}
