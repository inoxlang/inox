package core

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"slices"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []underlyingList{(*ValueList)(nil), (*IntList)(nil), (*FloatList)(nil)}
	_ = []IProps{(*Object)(nil), (*Record)(nil), (*Namespace)(nil), (*Dictionary)(nil), (*List)(nil)}

	_ Sequence = (*Array)(nil)
)

func init() {
	RegisterSymbolicGoFunction(NewArray, func(ctx *symbolic.Context, elements ...symbolic.Value) *symbolic.Array {
		return symbolic.NewArray(elements...)
	})
}

// Object implements Value.
type Object struct {
	keys   []string
	values []Serializable

	//TODO: Refactor to use a single []objectEntry slice if possible: this would reduce unsafe.Sizeof(Object) from 56 to 24.
	//      This would also make sortProps faster because a single slice would be modified.

	//Additional fields that are allocated if the object becomes more 'complex' or is watched.
	//Note: additionalObjectFields should never be set back to nil.
	*additionalObjectFields
}

type additionalObjectFields struct {
	lock SmartLock

	implicitPropCount int

	url        URL //can be empty
	txIsolator TransactionIsolator

	sysgraph SystemGraphPointer

	constraintId ConstraintId
	visibilityId VisibilityId

	jobs *ValueLifetimeJobs //can be nil

	//Watching related fields

	watchers              *ValueWatchers
	mutationCallbacks     *MutationCallbacks
	messageHandlers       *SynchronousMessageHandlers
	watchingDepth         WatchingDepth
	propMutationCallbacks []CallbackHandle
}

// NewObject creates an empty object.
func NewObject() *Object {
	return &Object{}
}

// helper function to create an object, lifetime jobs are initialized.
func NewObjectFromMap(valMap ValMap, ctx *Context) *Object {
	obj := objFrom(valMap)
	obj.addMessageHandlers(ctx) // add handlers before because jobs can mutate the object
	obj.instantiateLifetimeJobs(ctx)
	return obj
}

// helper function to create an object, lifetime jobs are not initialized.
func NewObjectFromMapNoInit(valMap ValMap) *Object {
	obj := objFrom(valMap)
	return obj
}

func newUnitializedObjectWithPropCount(count int) *Object {
	return &Object{
		keys:   make([]string, count),
		values: make([]Serializable, count),
	}
}

type ValMap map[string]Serializable

// helper function to create an object, lifetime jobs and system parts are NOT initialized
func objFrom(entryMap ValMap) *Object {
	keys := make([]string, len(entryMap))
	values := make([]Serializable, len(entryMap))

	maxKeyIndex := -1

	i := 0
	for k, v := range entryMap {
		if IsIndexKey(k) {
			maxKeyIndex = max(maxKeyIndex, utils.Must(strconv.Atoi(k)))
		}
		keys[i] = k
		values[i] = v
		i++
	}

	obj := &Object{keys: keys, values: values}
	obj.setImplicitPropCount(maxKeyIndex + 1)
	obj.sortProps()
	// NOTE: jobs not started
	return obj
}

func objFromLists(keys []string, values []Serializable) *Object {

	//handle index keys ? or ignore

	obj := &Object{keys: keys, values: values}
	obj.sortProps()
	return obj
}

func (obj *Object) ensureAdditionalFields() {
	if obj.additionalObjectFields == nil {
		obj.additionalObjectFields = &additionalObjectFields{}
	}
}

func (obj *Object) hasAdditionalFields() bool {
	return obj.additionalObjectFields != nil
}

func (obj *Object) hasWatchersOrMutationCallbacks() bool {
	return obj.additionalObjectFields != nil && (obj.mutationCallbacks != nil || obj.watchers != nil && len(obj.watchers.watchers) > 0)
}

func (obj *Object) hasPropMutationCallbacks() bool {
	return obj.additionalObjectFields != nil && obj.propMutationCallbacks != nil
}

func (obj *Object) sortProps() {
	if obj.hasPropMutationCallbacks() {
		for len(obj.propMutationCallbacks) < len(obj.keys) {
			obj.propMutationCallbacks = append(obj.propMutationCallbacks, FIRST_VALID_CALLBACK_HANDLE-1)
		}
	}

	keys, values, newIndexes := sortProps(obj.keys, obj.values)
	obj.keys, obj.values = keys, values

	if obj.hasPropMutationCallbacks() {
		newPropMutationCallbacks := make([]CallbackHandle, len(obj.propMutationCallbacks))
		for i, newIndex := range newIndexes {
			newPropMutationCallbacks[newIndex] = obj.propMutationCallbacks[i]
		}
	}
}

func (obj *Object) setImplicitPropCount(n int) {
	if obj.additionalObjectFields == nil && n == 0 {
		return
	}
	obj.ensureAdditionalFields()
	obj.implicitPropCount = n
}

func (obj *Object) getImplicitPropCount() int {
	if obj.additionalObjectFields == nil {
		return 0
	}
	return obj.additionalObjectFields.implicitPropCount
}

func (obj *Object) indexOfKey(k string) int {
	for i, key := range obj.keys {
		if key == k {
			return i
		}
	}
	panic(fmt.Errorf("unknown key %s", k))
}

// this function is called during object creation
func (obj *Object) instantiateLifetimeJobs(ctx *Context) error {
	var jobs []*LifetimeJob
	state := ctx.GetClosestState()

	for i, key := range obj.keys {
		if !IsIndexKey(key) {
			continue
		}

		if job, ok := obj.values[i].(*LifetimeJob); ok && job.subjectPattern == nil {
			jobs = append(jobs, job)
		}
	}

	if len(jobs) != 0 {
		obj.Share(state)
		obj.ensureAdditionalFields()
		jobs := NewValueLifetimeJobs(ctx, obj, jobs)
		if err := jobs.InstantiateJobs(ctx); err != nil {
			return err
		}
		obj.jobs = jobs
	}
	return nil
}

// this function is called during object creation
func (obj *Object) addMessageHandlers(ctx *Context) error {
	var handlers []*SynchronousMessageHandler

	for i, key := range obj.keys {
		if !IsIndexKey(key) {
			continue
		}

		if handler, ok := obj.values[i].(*SynchronousMessageHandler); ok {
			handlers = append(handlers, handler)
		}
	}

	if len(handlers) != 0 {
		obj.ensureAdditionalFields()
		obj.messageHandlers = NewSynchronousMessageHandlers(handlers...)
	}

	return nil
}

// LifetimeJobs returns the lifetime jobs bound to the object, the returned pointer can be nil.
func (obj *Object) LifetimeJobs() *ValueLifetimeJobs {
	if obj.hasAdditionalFields() {
		return nil
	}
	return obj.jobs
}

func (obj *Object) IsSharable(originState *GlobalState) (bool, string) {
	if obj.hasAdditionalFields() && obj.lock.IsValueShared() {
		return true, ""
	}
	for i, v := range obj.values {
		k := obj.keys[i]
		if ok, expl := IsSharableOrClonable(v, originState); !ok {
			return false, commonfmt.FmtNotSharableBecausePropertyNotSharable(k, expl)
		}
	}
	return true, ""
}

func (obj *Object) Share(originState *GlobalState) {
	obj.ensureAdditionalFields()
	obj.lock.Share(originState, func() {
		for i, v := range obj.values {
			obj.values[i] = utils.Must(ShareOrClone(v, originState)).(Serializable)
		}
		//Allocating the additional fields enables transaction isolation.
		obj.ensureAdditionalFields()
	})
}

func (obj *Object) IsShared() bool {
	if !obj.hasAdditionalFields() {
		return false
	}
	return obj.lock.IsValueShared()
}

func (obj *Object) Lock(state *GlobalState) {
	if obj.additionalObjectFields == nil { //not shared.
		return
	}
	obj.lock.Lock(state, obj)
}

func (obj *Object) Unlock(state *GlobalState) {
	if obj.additionalObjectFields == nil { //not shared.
		return
	}
	obj.lock.Unlock(state, obj)
}

func (obj *Object) ForceLock() {
	obj.ensureAdditionalFields()
	obj.lock.ForceLock()
}

func (obj *Object) ForceUnlock() {
	if obj.additionalObjectFields == nil {
		panic(errors.New("unexpected Object.ForceUnlock call: object is not locked because it does not have additional fields"))
	}
	obj.lock.ForceUnlock()
}

func (obj *Object) jobInstances() []*LifetimeJobInstance {
	if !obj.hasAdditionalFields() {
		return nil
	}
	return obj.jobs.Instances()
}

func (obj *Object) waitIfOtherTransaction(ctx *Context, requiredRunningTx bool) {
	if !obj.hasAdditionalFields() {
		return
	}
	err := obj.txIsolator.WaitIfOtherTransaction(ctx, requiredRunningTx)
	if err != nil {
		panic(err)
	}
}

func (obj *Object) Prop(ctx *Context, name string) Value {
	return obj.prop(ctx, name, true)
}

func (obj *Object) PropNotStored(ctx *Context, name string) Value {
	return obj.prop(ctx, name, false)
}

func (obj *Object) prop(ctx *Context, name string, stored bool) Value {
	obj.waitIfOtherTransaction(ctx, !stored)

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	if obj.hasAdditionalFields() && obj.url != "" {
		perm := DatabasePermission{
			Kind_:  permkind.Read,
			Entity: obj.url.ToDirURL().AppendRelativePath("./" + Path(name)),
		}

		if err := ctx.CheckHasPermission(perm); err != nil {
			panic(err)
		}
	}

	for i, key := range obj.keys {
		if key == name {
			v := obj.values[i]

			if obj.IsShared() {
				if stored {
					return utils.Must(CheckSharedOrClone(v, map[uintptr]Clonable{}, 0)).(Serializable)
				}
			}
			return v
		}
	}
	panic(FormatErrPropertyDoesNotExist(name, obj))
}

func (obj *Object) SetProp(ctx *Context, name string, value Value) error {

	serializableVal, ok := value.(Serializable)
	if !ok {
		return fmt.Errorf("value is not serializable")
	}

	obj.waitIfOtherTransaction(ctx, false)

	closestState := ctx.GetClosestState()

	if obj.IsShared() {
		newVal, err := ShareOrClone(value, closestState)
		if err != nil {
			return fmt.Errorf("failed to share/clone value when setting property %s: %w", name, err)
		}
		serializableVal = newVal.(Serializable)
	}

	unlock := true
	obj.Lock(closestState)
	defer func() {
		if unlock {
			obj.Unlock(closestState)
		}
	}()

	var constraint Pattern
	if obj.hasAdditionalFields() && obj.constraintId.HasConstraint() {
		constraint, _ = GetConstraint(obj.constraintId)
	}

	if IsIndexKey(name) {
		panic(ErrCannotSetValOfIndexKeyProp)
	}

	if obj.hasAdditionalFields() && obj.url != "" {
		perm := DatabasePermission{
			Kind_:  permkind.Write,
			Entity: obj.url.ToDirURL().AppendRelativePath("./" + Path(name)),
		}

		if err := ctx.CheckHasPermission(perm); err != nil {
			return err
		}
	}

	for i, key := range obj.keys {
		if key == name { // property is already present
			prevValue := obj.values[i]
			obj.values[i] = serializableVal

			// check constraints

			if constraint != nil && !constraint.(*ObjectPattern).Test(ctx, obj) {
				obj.values[i] = prevValue
				return ErrConstraintViolation
			}

			// update object

			obj.sortProps()

			if !obj.hasAdditionalFields() {
				return nil
			}

			obj.sysgraph.AddEvent(ctx, "prop updated: "+name, obj)

			if obj.hasPropMutationCallbacks() {
				//Remove the mutation callback of the previous value and a mutation callback
				//for the new value.
				index := obj.indexOfKey(name)
				obj.removePropMutationCallbackNoLock(ctx, index, prevValue)
				if err := obj.addPropMutationCallbackNoLock(ctx, index, serializableVal); err != nil {
					return fmt.Errorf("failed to add mutation callback for updated object property %s: %w", name, err)
				}
			}

			if obj.hasWatchersOrMutationCallbacks() {
				//Create mutation and inform watchers about it.
				mutation := NewUpdatePropMutation(ctx, name, serializableVal, ShallowWatching, Path("/"+name))

				obj.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)

				if obj.mutationCallbacks != nil {
					unlock = false
					obj.Unlock(closestState)

					obj.mutationCallbacks.CallMicrotasks(ctx, mutation)
				}
			}

			return nil
		}
	}

	// add new property
	obj.keys = append(obj.keys, name)
	obj.values = append(obj.values, serializableVal)

	//check constraint
	if constraint != nil && !constraint.(*ObjectPattern).Test(ctx, obj) {
		obj.keys = obj.keys[:len(obj.keys)-1]
		obj.values = obj.values[:len(obj.values)-1]
		return ErrConstraintViolation
	}

	obj.sortProps()

	if !obj.hasAdditionalFields() {
		return nil
	}

	//---------------------------------------

	if obj.hasPropMutationCallbacks() {
		if err := obj.addPropMutationCallbackNoLock(ctx, len(obj.keys)-1, serializableVal); err != nil {
			return fmt.Errorf("failed to add mutation callback for new object property %s: %w", name, err)
		}
	}

	obj.sysgraph.AddEvent(ctx, "new prop: "+name, obj)

	if obj.hasWatchersOrMutationCallbacks() {
		//Inform watchers & microtasks about the update.
		mutation := NewAddPropMutation(ctx, name, serializableVal, ShallowWatching, Path("/"+name))
		obj.sysgraph.AddEvent(ctx, "new prop: "+name, obj)

		obj.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)

		if obj.mutationCallbacks != nil {
			unlock = false
			obj.Unlock(closestState)

			obj.mutationCallbacks.CallMicrotasks(ctx, mutation)
		}
	}

	return nil
}

func (obj *Object) PropertyNames(ctx *Context) []string {
	obj.waitIfOtherTransaction(ctx, false)

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	return obj.keys
}

func (obj *Object) HasProp(ctx *Context, name string) bool {
	obj.waitIfOtherTransaction(ctx, false)

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	for _, k := range obj.keys {
		if k == name {
			return true
		}
	}
	return false
}

func (obj *Object) HasPropValue(ctx *Context, value Value) bool {
	obj.waitIfOtherTransaction(ctx, false)

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	for _, v := range obj.values {
		if v.Equal(ctx, value, map[uintptr]uintptr{}, 0) {
			return true
		}
	}
	return false
}

func (obj *Object) EntryMap(ctx *Context) map[string]Serializable {
	if obj == nil {
		return nil
	}

	if ctx != nil {
		obj.waitIfOtherTransaction(ctx, false)

		closestState := ctx.GetClosestState()
		obj.Lock(closestState)
		defer obj.Unlock(closestState)
	} else {
		obj.Lock(nil)
		defer obj.Unlock(nil)
	}

	isShared := obj.IsShared()

	map_ := map[string]Serializable{}
	for i, v := range obj.values {
		if isShared {
			v = utils.Must(CheckSharedOrClone(v, map[uintptr]Clonable{}, 0)).(Serializable)
		}
		map_[obj.keys[i]] = v
	}
	return map_
}

func (obj *Object) ValueEntryMap(ctx *Context) map[string]Value {
	if obj == nil {
		return nil
	}

	if ctx != nil {
		obj.waitIfOtherTransaction(ctx, false)

		closestState := ctx.GetClosestState()
		obj.Lock(closestState)
		defer obj.Unlock(closestState)
	} else {
		obj.Lock(nil)
		defer obj.Unlock(nil)
	}

	isShared := obj.IsShared()

	map_ := map[string]Value{}
	for i, v := range obj.values {
		if isShared {
			v = utils.Must(CheckSharedOrClone(v, map[uintptr]Clonable{}, 0)).(Serializable)
		}
		map_[obj.keys[i]] = v
	}
	return map_
}

// Indexed returns the list of indexed properties
func (obj *Object) Indexed() []Serializable {
	if obj.IsShared() {
		panic(errors.New("Object.Indexed() can only be called on objects that are not shared"))
	}

	implicitPropCount := obj.getImplicitPropCount()
	values := make([]Serializable, implicitPropCount)

outer:
	for i := 0; i < implicitPropCount; i++ {
		searchedKey := strconv.Itoa(i)
		for i, key := range obj.keys {
			if key == searchedKey {
				values[i] = obj.values[i]
				continue outer
			}
		}
		panic(ErrUnreachable)
	}

	return values
}

func (obj *Object) ForEachEntry(fn func(k string, v Serializable) error) error {
	if obj.IsShared() {
		panic(errors.New("Object.ForEachEntry() can only be called on objects that are not shared"))
	}

	for i, v := range obj.values {
		if err := fn(obj.keys[i], v); err != nil {
			return err
		}
	}
	return nil
}

func (obj *Object) URL() (URL, bool) {
	if !obj.hasAdditionalFields() {
		return "", false
	}
	if obj.url != "" {
		return obj.url, true
	}
	return "", false
}

func (obj *Object) SetURLOnce(ctx *Context, u URL) error {
	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	obj.ensureAdditionalFields()

	if obj.url != "" {
		return ErrURLAlreadySet
	}

	obj.url = u
	return nil
}

// len returns the number of implicit properties
func (obj *Object) Len() int {
	if obj.hasAdditionalFields() {
		return obj.implicitPropCount
	}
	return 0
}

func (obj *Object) At(ctx *Context, i int) Value {
	return obj.Prop(ctx, strconv.Itoa(i))
}

func (obj *Object) Keys(ctx *Context) []string {
	obj.waitIfOtherTransaction(ctx, false)

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	return obj.keys
}

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

type Array []Value

func NewArrayFrom(elements ...Value) *Array {
	if elements == nil {
		elements = []Value{}
	}
	array := Array(elements)
	return &array
}

func NewArray(ctx *Context, elements ...Value) *Array {
	return NewArrayFrom(elements...)
}

func (a *Array) At(ctx *Context, i int) Value {
	return (*a)[i]
}

func (a *Array) Len() int {
	return len(*a)
}

func (a *Array) slice(start int, end int) Sequence {
	slice := (*a)[start:end]
	return &slice
}

type KeyList []string

type Indexable interface {
	Iterable

	// At should panic if the index is out of bounds.
	At(ctx *Context, i int) Value

	Len() int
}

// A List represents a sequence of elements, List implements Value.
// The elements are stored in an underlyingList that is suited for the number and kind of elements, for example
// if the elements are all integers the underlying list will (ideally) be an *IntList.
type List struct {
	underlyingList
	elemType Pattern

	lock                     sync.Mutex // exclusive access for initializing .watchers & .mutationCallbacks
	mutationCallbacks        *MutationCallbacks
	watchers                 *ValueWatchers
	watchingDepth            WatchingDepth
	elementMutationCallbacks []CallbackHandle
}

func newList(underlyingList underlyingList) *List {
	return &List{underlyingList: underlyingList}
}

func WrapUnderlyingList(l underlyingList) *List {
	return &List{underlyingList: l}
}

// the caller can modify the result.
func (list *List) GetOrBuildElements(ctx *Context) []Serializable {
	entries := IterateAll(ctx, list.Iterator(ctx, IteratorConfiguration{}))

	values := make([]Serializable, len(entries))
	for i, e := range entries {
		values[i] = e[1].(Serializable)
	}
	return values
}

func (l *List) Prop(ctx *Context, name string) Value {
	switch name {
	case "append":
		return WrapGoMethod(l.append)
	case "dequeue":
		return WrapGoMethod(l.Dequeue)
	case "pop":
		return WrapGoMethod(l.Pop)
	case "sorted":
		return WrapGoMethod(l.Sorted)
	case "sort_by":
		return WrapGoMethod(l.SortBy)
	case "len":
		return Int(l.Len())
	default:
		panic(FormatErrPropertyDoesNotExist(name, l))
	}
}

func (*List) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*List) PropertyNames(ctx *Context) []string {
	return symbolic.LIST_PROPNAMES
}

func (l *List) set(ctx *Context, i int, v Value) {
	prevElement := l.underlyingList.At(ctx, i)
	l.underlyingList.set(ctx, i, v)

	if l.elementMutationCallbacks != nil {
		l.removeElementMutationCallbackNoLock(ctx, i, prevElement.(Serializable))
		l.addElementMutationCallbackNoLock(ctx, i, v)
	}

	mutation := NewSetElemAtIndexMutation(ctx, i, v.(Serializable), ShallowWatching, Path("/"+strconv.Itoa(i)))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) SetSlice(ctx *Context, start, end int, seq Sequence) {
	if l.elementMutationCallbacks != nil {
		for i := start; i < end; i++ {
			prevElement := l.underlyingList.At(ctx, i)
			l.removeElementMutationCallbackNoLock(ctx, i, prevElement.(Serializable))
		}
	}

	l.underlyingList.SetSlice(ctx, start, end, seq)

	if l.elementMutationCallbacks != nil {
		for i := start; i < end; i++ {
			l.addElementMutationCallbackNoLock(ctx, i, l.underlyingList.At(ctx, i))
		}
	}

	path := Path("/" + strconv.Itoa(int(start)) + ".." + strconv.Itoa(int(end-1)))
	mutation := NewSetSliceAtRangeMutation(ctx, NewIncludedEndIntRange(int64(start), int64(end-1)), seq.(Serializable), ShallowWatching, path)

	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
	l.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (l *List) insertElement(ctx *Context, v Value, i Int) {
	l.underlyingList.insertElement(ctx, v, i)

	if l.elementMutationCallbacks != nil {
		l.elementMutationCallbacks = slices.Insert(l.elementMutationCallbacks, int(i), FIRST_VALID_CALLBACK_HANDLE-1)
		l.addElementMutationCallbackNoLock(ctx, int(i), v)
	}

	mutation := NewInsertElemAtIndexMutation(ctx, int(i), v.(Serializable), ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) insertSequence(ctx *Context, seq Sequence, i Int) {
	l.underlyingList.insertSequence(ctx, seq, i)

	if l.elementMutationCallbacks != nil {
		seqLen := seq.Len()
		l.elementMutationCallbacks = slices.Insert(l.elementMutationCallbacks, int(i), makeMutationCallbackHandles(seqLen)...)

		seqIndex := 0
		for index := i; index < i+Int(seqLen); index++ {
			l.addElementMutationCallbackNoLock(ctx, int(index), seq.At(ctx, seqIndex))
			seqIndex++
		}
	}

	mutation := NewInsertSequenceAtIndexMutation(ctx, int(i), seq, ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) appendSequence(ctx *Context, seq Sequence) {
	l.insertSequence(ctx, seq, Int(l.Len()))
}

func (l *List) append(ctx *Context, elements ...Serializable) {
	index := l.Len()
	l.underlyingList.append(ctx, elements...)

	seq := NewWrappedValueList(elements...)

	if l.elementMutationCallbacks != nil {
		seqLen := seq.Len()
		l.elementMutationCallbacks = slices.Insert(l.elementMutationCallbacks, index, makeMutationCallbackHandles(seqLen)...)

		for i := index; i < index+len(elements); i++ {
			l.addElementMutationCallbackNoLock(ctx, int(i), seq.At(ctx, int(i-index)))
		}
	}

	mutation := NewInsertSequenceAtIndexMutation(ctx, index, seq, ShallowWatching, Path("/"+strconv.Itoa(index)))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) removePosition(ctx *Context, i Int) {
	l.underlyingList.removePosition(ctx, i)

	if l.elementMutationCallbacks != nil {
		l.removeElementMutationCallbackNoLock(ctx, int(i), l.underlyingList.At(ctx, int(i)).(Serializable))
		l.elementMutationCallbacks = slices.Replace(l.elementMutationCallbacks, int(i), int(i+1))
	}

	mutation := NewRemovePositionMutation(ctx, int(i), ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) Dequeue(ctx *Context) Serializable {
	if l.Len() == 0 {
		panic(ErrCannotDequeueFromEmptyList)
	}
	elem := l.At(ctx, 0)
	l.removePosition(ctx, 0)
	return elem.(Serializable)
}

func (l *List) Pop(ctx *Context) Serializable {
	lastIndex := l.Len() - 1
	if lastIndex < 0 {
		panic(ErrCannotPopFromEmptyList)
	}
	elem := l.At(ctx, lastIndex)
	l.removePosition(ctx, Int(lastIndex))
	return elem.(Serializable)
}

func (l *List) removePositionRange(ctx *Context, r IntRange) {
	l.underlyingList.removePositionRange(ctx, r)

	if l.elementMutationCallbacks != nil {
		for index := int(r.start); index < int(r.end); index++ {
			l.removeElementMutationCallbackNoLock(ctx, index, l.underlyingList.At(ctx, index).(Serializable))
		}

		l.elementMutationCallbacks = slices.Replace(l.elementMutationCallbacks, int(r.start), int(r.end))
	}

	path := Path("/" + strconv.Itoa(int(r.KnownStart())) + ".." + strconv.Itoa(int(r.InclusiveEnd())))
	mutation := NewRemovePositionRangeMutation(ctx, r, ShallowWatching, path)

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func sortProps[V Value](keys []string, values []V) ([]string, []V, []int) {
	if len(keys) == 0 {
		return nil, nil, nil
	}
	newKeys := slices.Clone(keys)
	sort.Strings(newKeys)
	newValues := make([]V, len(values))
	newIndexes := make([]int, len(values))

	for i := 0; i < len(keys); i++ {
		newIndex := sort.SearchStrings(newKeys, keys[i])
		newValues[newIndex] = values[i]
		newIndexes[i] = newIndex
	}

	return newKeys, newValues, newIndexes
}
