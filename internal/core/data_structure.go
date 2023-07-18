package core

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/slices"
)

var (
	_ = []underylingList{&ValueList{}, &IntList{}}

	_ Sequence = (*Array)(nil)
)

func init() {
	RegisterSymbolicGoFunction(NewArray, func(ctx *symbolic.Context, elements ...symbolic.SymbolicValue) *symbolic.Array {
		return symbolic.NewArray()
	})
}

// Object implements Value.
type Object struct {
	constraintId      ConstraintId
	visibilityId      VisibilityId
	implicitPropCount int
	lock              SmartLock

	url URL //can be empty

	// system
	sysLock     sync.Mutex
	isSystem    bool
	supersys    PotentialSystem // can be nil
	systemParts []SystemPart

	watchers               *ValueWatchers
	mutationCallbacks      *MutationCallbacks
	messageHandlers        *SynchronousMessageHandlers
	watchingDepth          WatchingDepth
	propMutationCallbacks  []CallbackHandle
	currentTransaction     *Transaction
	currentTransactionLock sync.Mutex

	jobs *ValueLifetimeJobs

	keys   []string
	values []Serializable

	sysgraph SystemGraphPointer
}

// NewObject creates an empty object.
func NewObject() *Object {
	return &Object{}
}

// helper function to create an object, lifetime jobs and system parts are initialized.
func NewObjectFromMap(valMap ValMap, ctx *Context) *Object {
	obj := objFrom(valMap)
	obj.initPartList(ctx)
	obj.addMessageHandlers(ctx) // add handlers before because jobs can mutate the object
	obj.instantiateLifetimeJobs(ctx)
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
			maxKeyIndex = utils.Max(maxKeyIndex, utils.Must(strconv.Atoi(k)))
		}
		keys[i] = k
		values[i] = v
		i++
	}

	obj := &Object{keys: keys, values: values, implicitPropCount: maxKeyIndex + 1}
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

func (obj *Object) sortProps() {
	if obj.propMutationCallbacks != nil {
		for len(obj.propMutationCallbacks) < len(obj.keys) {
			obj.propMutationCallbacks = append(obj.propMutationCallbacks, FIRST_VALID_CALLBACK_HANDLE-1)
		}
	}

	keys, values, newIndexes := sortProps(obj.keys, obj.values)
	obj.keys, obj.values = keys, values

	if obj.propMutationCallbacks != nil {
		newPropMutationCallbacks := make([]CallbackHandle, len(obj.propMutationCallbacks))
		for i, newIndex := range newIndexes {
			newPropMutationCallbacks[newIndex] = obj.propMutationCallbacks[i]
		}
	}
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
func (obj *Object) initPartList(ctx *Context) {
	shared := obj.IsShared()
	state := ctx.GetClosestState()

	obj.sysLock.Lock()
	defer obj.sysLock.Unlock()

	hasLifetimeJobs := false

	for _, val := range obj.values {
		if job, ok := val.(*LifetimeJob); ok && job.subjectPattern == nil {
			hasLifetimeJobs = true
			break
		}
	}

	// if the object has no lifetime jobs it is not considered a system
	if !hasLifetimeJobs {
		return
	}
	obj.isSystem = true

	for _, val := range obj.values {
		if part, ok := val.(SystemPart); ok {
			if !shared {
				obj.Share(state)
				shared = true
			}

			obj.systemParts = append(obj.systemParts, part)
			if err := part.AttachToSystem(obj); err != nil {
				panic(err)
			}
		}
	}
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
		obj.messageHandlers = NewSynchronousMessageHandlers(handlers...)
	}

	return nil
}

func (obj *Object) LifetimeJobs() *ValueLifetimeJobs {
	return obj.jobs
}

func (obj *Object) IsSharable(originState *GlobalState) (bool, string) {
	if obj.lock.IsValueShared() {
		return true, ""
	}
	for i, v := range obj.values {
		k := obj.keys[i]
		if ok, expl := IsSharable(v, originState); !ok {
			return false, commonfmt.FmtNotSharableBecausePropertyNotSharable(k, expl)
		}
	}
	return true, ""
}

func (obj *Object) Share(originState *GlobalState) {
	obj.lock.Share(originState, func() {
		for _, v := range obj.values {
			if potentiallySharable, ok := v.(PotentiallySharable); ok && v.IsMutable() {
				potentiallySharable.Share(originState)
			}
		}
	})
}

func (obj *Object) IsShared() bool {
	return obj.lock.IsValueShared()
}

func (obj *Object) Lock(state *GlobalState) {
	obj.lock.Lock(state, obj)
}

func (obj *Object) Unlock(state *GlobalState) {
	obj.lock.Unlock(state, obj)
}

func (obj *Object) ForceLock() {
	obj.lock.ForceLock()
}

func (obj *Object) ForceUnlock() {
	obj.lock.ForceUnlock()
}

func (obj *Object) jobInstances() []*LifetimeJobInstance {
	return obj.jobs.Instances()
}

func (obj *Object) SystemParts() []SystemPart {
	obj.sysLock.Lock()
	defer obj.sysLock.Unlock()
	return obj.systemParts
}

func (obj *Object) AttachToSystem(superSystem PotentialSystem) error {
	obj.sysLock.Lock()
	defer obj.sysLock.Unlock()

	if obj.supersys != nil {
		return ErrAlreadyAttachedToSupersystem
	}
	//TODO: add more checks
	obj.supersys = superSystem
	return nil
}

func (obj *Object) DetachFromSystem() error {
	obj.sysLock.Lock()
	defer obj.sysLock.Unlock()

	if obj.supersys == nil {
		return ErrNotAttachedToSupersystem
	}
	obj.supersys = nil
	return nil
}

func (obj *Object) System() (PotentialSystem, error) {
	obj.sysLock.Lock()
	defer obj.sysLock.Unlock()

	if obj.supersys == nil {
		return nil, ErrNotAttachedToSupersystem
	}
	return obj.supersys, nil
}

func (obj *Object) waitIfOtherTransaction(ctx *Context) error {
	//TODO: wait when accessing methods implementing the System interface ?

	obj.currentTransactionLock.Lock()

	tx := ctx.GetTx()

	if obj.currentTransaction != nil && obj.currentTransaction.IsFinished() {
		obj.currentTransaction = nil
	}

	if obj.currentTransaction == nil {
		if tx != nil && !tx.IsFinished() {
			obj.currentTransaction = tx
		}
		obj.currentTransactionLock.Unlock()
		return nil
	}

	if tx != obj.currentTransaction {
		select {
		case <-obj.currentTransaction.WaitFinished():
		case <-ctx.Done():
			return ctx.Err()
		}

		obj.currentTransaction = nil
		obj.currentTransactionLock.Unlock()

		return obj.waitIfOtherTransaction(ctx)
	}
	obj.currentTransactionLock.Unlock()

	return nil
}

func (obj *Object) Prop(ctx *Context, name string) Value {
	if err := obj.waitIfOtherTransaction(ctx); err != nil {
		panic(err)
	}

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	for i, key := range obj.keys {
		if key == name {
			return obj.values[i]
		}
	}
	panic(FormatErrPropertyDoesNotExist(name, obj))
}

func (obj *Object) SetProp(ctx *Context, name string, value Value) error {

	serializableVal, ok := value.(Serializable)
	if !ok {
		return fmt.Errorf("value is not serializable")
	}

	if err := obj.waitIfOtherTransaction(ctx); err != nil {
		return err
	}

	closestState := ctx.GetClosestState()

	// if obj.IsShared() {
	// 	newVal, err := ShareOrClone(value, closestState)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to share/clone value when setting property %s: %w", name, err)
	// 	}
	// 	value = newVal
	// }

	unlock := true
	obj.Lock(closestState)
	defer func() {
		if unlock {
			obj.Unlock(closestState)
		}
	}()

	var constraint Pattern
	if obj.constraintId.HasConstraint() {
		constraint, _ = GetConstraint(obj.constraintId)
	}

	if IsIndexKey(name) {
		panic(errors.New("cannot set value of index key property"))
	}

	for i, key := range obj.keys {
		if key == name { // property is already present
			prevValue := obj.values[i]

			if obj.isSystem {
				if prevPart, ok := prevValue.(SystemPart); ok {
					if err := prevPart.DetachFromSystem(); err != nil {
						return err
					}

					obj.sysLock.Lock()
					for i, part := range obj.systemParts {
						if part != prevPart {
							continue
						}

						if newPart, ok := value.(SystemPart); ok {
							obj.systemParts[i] = newPart
						} else {
							if i+1 != len(obj.systemParts) {
								copy(obj.systemParts[i:], obj.systemParts[i+1:])
							}
							obj.systemParts = obj.systemParts[:len(obj.systemParts)-1]
						}
						break
					}
					obj.sysLock.Unlock()
				}

				if part, ok := value.(SystemPart); ok {
					if err := part.AttachToSystem(obj); err != nil {
						return err
					}
				}
			}

			obj.values[i] = serializableVal

			// check constraints

			if constraint != nil && !constraint.(*ObjectPattern).Test(ctx, obj) {
				obj.values[i] = prevValue
				return ErrConstraintViolation
			}

			// update object

			obj.sortProps()

			if obj.propMutationCallbacks != nil {
				index := obj.indexOfKey(name)
				obj.removePropMutationCallbackNoLock(ctx, index, prevValue)
				if err := obj.addPropMutationCallbackNoLock(ctx, index, serializableVal); err != nil {
					return fmt.Errorf("failed to add mutation callback for updated object property %s: %w", name, err)
				}
			}

			mutation := NewUpdatePropMutation(ctx, name, serializableVal, ShallowWatching, Path("/"+name))

			obj.sysgraph.AddEvent(ctx, "prop updated: "+name, obj)

			//inform watchers & microtasks about the update
			obj.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)

			if obj.mutationCallbacks != nil {
				unlock = false
				obj.Unlock(closestState)

				obj.mutationCallbacks.CallMicrotasks(ctx, mutation)
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

	if obj.isSystem {
		if part, ok := value.(SystemPart); ok { //add new part
			obj.systemParts = append(obj.systemParts, part)
			if err := part.AttachToSystem(obj); err != nil {
				panic(err)
			}
		}
	}

	if obj.propMutationCallbacks != nil {
		if err := obj.addPropMutationCallbackNoLock(ctx, len(obj.keys)-1, serializableVal); err != nil {
			return fmt.Errorf("failed to add mutation callback for new object property %s: %w", name, err)
		}
	}

	//inform watchers & microtasks about the update

	mutation := NewAddPropMutation(ctx, name, serializableVal, ShallowWatching, Path("/"+name))
	obj.sysgraph.AddEvent(ctx, "new prop: "+name, obj)

	obj.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)

	if obj.mutationCallbacks != nil {
		unlock = false
		obj.Unlock(closestState)

		obj.mutationCallbacks.CallMicrotasks(ctx, mutation)
	}

	return nil
}

func (obj *Object) PropertyNames(ctx *Context) []string {
	if err := obj.waitIfOtherTransaction(ctx); err != nil {
		panic(err)
	}

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	return obj.keys
}

func (obj *Object) HasProp(ctx *Context, name string) bool {
	if err := obj.waitIfOtherTransaction(ctx); err != nil {
		panic(err)
	}

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
	if err := obj.waitIfOtherTransaction(ctx); err != nil {
		panic(err)
	}

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
		if err := obj.waitIfOtherTransaction(ctx); err != nil {
			panic(err)
		}

		closestState := ctx.GetClosestState()
		obj.Lock(closestState)
		defer obj.Unlock(closestState)
	} else {
		obj.Lock(nil)
		defer obj.Unlock(nil)
	}

	map_ := map[string]Serializable{}
	for i, v := range obj.values {
		map_[obj.keys[i]] = v
	}
	return map_
}

func (obj *Object) ValueEntryMap(ctx *Context) map[string]Value {
	if obj == nil {
		return nil
	}

	if ctx != nil {
		if err := obj.waitIfOtherTransaction(ctx); err != nil {
			panic(err)
		}

		closestState := ctx.GetClosestState()
		obj.Lock(closestState)
		defer obj.Unlock(closestState)
	} else {
		obj.Lock(nil)
		defer obj.Unlock(nil)
	}

	map_ := map[string]Value{}
	for i, v := range obj.values {
		map_[obj.keys[i]] = v
	}
	return map_
}

// Indexed returns the list of indexed properties
func (obj *Object) Indexed() []Serializable {
	if obj.IsShared() {
		panic(errors.New("Object.Indexed() can only be called on objects that are not shared"))
	}

	values := make([]Serializable, obj.implicitPropCount)

outer:
	for i := 0; i < obj.implicitPropCount; i++ {
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
	if obj.url != "" {
		return obj.url, true
	}
	return "", false
}

func (obj *Object) SetURLOnce(ctx *Context, u URL) error {
	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	if obj.url != "" {
		return ErrURLAlreadySet
	}
	obj.url = u
	return nil
}

// len returns the number of implicit properties
func (obj *Object) Len() int {
	return obj.implicitPropCount
}

func (obj *Object) At(ctx *Context, i int) Value {
	return obj.Prop(ctx, strconv.Itoa(i))
}

func (obj *Object) Keys(ctx *Context) []string {
	if err := obj.waitIfOtherTransaction(ctx); err != nil {
		panic(err)
	}

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	return obj.keys
}

// Record is the immutable equivalent of an Object, Record implements Value.
type Record struct {
	implicitPropCount int //TODO: rename to indexedPropCount ? forbid explicit index keys ?
	visibilityId      VisibilityId
	url               URL //can be empty
	keys              []string
	values            []Serializable
}

func NewEmptyRecord() *Record {
	return &Record{}
}

func NewRecordFromMap(entryMap ValMap) *Record {
	keys := make([]string, len(entryMap))
	values := make([]Serializable, len(entryMap))

	maxKeyIndex := -1

	i := 0
	for k, v := range entryMap {
		if v.IsMutable() {
			panic(fmt.Errorf("value of provided property .%s is mutable", k))
		}
		if IsIndexKey(k) {
			maxKeyIndex = utils.Max(maxKeyIndex, utils.Must(strconv.Atoi(k)))
		}
		keys[i] = k
		values[i] = v
		i++
	}

	rec := &Record{keys: keys, values: values, implicitPropCount: maxKeyIndex + 1}
	rec.sortProps()
	return rec
}

func NewRecordFromKeyValLists(keys []string, values []Serializable) *Record {
	maxKeyIndex := -1
	i := 0
	for ind, k := range keys {
		v := values[ind]
		if v.IsMutable() {
			panic(fmt.Errorf("value of provided property .%s is mutable", k))
		}

		if IsIndexKey(k) {
			maxKeyIndex = utils.Max(maxKeyIndex, utils.Must(strconv.Atoi(k)))
		}
		keys[i] = k
		values[i] = v
		i++
	}

	rec := &Record{keys: keys, values: values, implicitPropCount: maxKeyIndex + 1}
	rec.sortProps()
	return rec
}

func (rec *Record) Prop(ctx *Context, name string) Value {
	for i, key := range rec.keys {
		if key == name {
			return rec.values[i]
		}
	}
	panic(FormatErrPropertyDoesNotExist(name, rec))
}

func (rec Record) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (rec Record) PropertyNames(ctx *Context) []string {
	return rec.Keys()
}

func (rec *Record) HasProp(ctx *Context, name string) bool {
	for _, k := range rec.keys {
		if k == name {
			return true
		}
	}
	return false
}

func (rec *Record) ValueEntryMap() map[string]Value {
	if rec == nil {
		return nil
	}
	map_ := map[string]Value{}
	for i, v := range rec.values {
		map_[rec.keys[i]] = v
	}
	return map_
}

func (rec *Record) ForEachEntry(fn func(k string, v Value) error) error {
	for i, v := range rec.values {
		if err := fn(rec.keys[i], v); err != nil {
			return err
		}
	}
	return nil
}

func (rec *Record) sortProps() {
	rec.keys, rec.values, _ = sortProps(rec.keys, rec.values)
}

// len returns the number of implicit properties
func (rec *Record) Len() int {
	return rec.implicitPropCount
}

func (rec *Record) At(ctx *Context, i int) Value {
	return rec.Prop(nil, strconv.Itoa(i))
}

func (rec *Record) Keys() []string {
	return rec.keys
}

func (rec *Record) EntryMap() map[string]Serializable {
	if rec == nil {
		return nil
	}
	map_ := map[string]Serializable{}
	for i, v := range rec.values {
		map_[rec.keys[i]] = v
	}
	return map_
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

func convertKeyReprToValue(repr string) Serializable {
	keyNode, ok := parse.ParseExpression(repr)
	if !ok {
		panic(fmt.Errorf("invalid key representation: %s", repr))
	}
	//TODO: refactor
	key, err := TreeWalkEval(keyNode, NewTreeWalkStateWithGlobal(&GlobalState{}))
	if err != nil {
		panic(err)
	}
	return key.(Serializable)
}

func NewDictionary(entries ValMap) *Dictionary {
	dict := &Dictionary{
		entries: map[string]Serializable{},
		keys:    map[string]Serializable{},
	}
	for keyRepresentation, v := range entries {
		dict.entries[keyRepresentation] = v
		dict.keys[keyRepresentation] = convertKeyReprToValue(keyRepresentation)
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

func (d *Dictionary) Value(ctx *Context, key Serializable) (Value, Bool) {
	v, ok := d.entries[string(GetRepresentation(key, ctx))]
	return v, Bool(ok)
}

func (d *Dictionary) SetValue(ctx *Context, key, value Serializable) {
	keyRepr := string(GetRepresentation(key, ctx))

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
// The elements are stored in an underylingList that is suited for the number and kind of elements, for example
// if the elements are all integers the underyling list will (ideally) be an *IntList.
type List struct {
	underylingList
	elemType Pattern

	lock                     sync.Mutex // exclusive access for initializing .watchers & .mutationCallbacks
	mutationCallbacks        *MutationCallbacks
	watchers                 *ValueWatchers
	watchingDepth            WatchingDepth
	elementMutationCallbacks []CallbackHandle
}

func newList(underylingList underylingList) *List {
	return &List{underylingList: underylingList}
}

func WrapUnderylingList(l underylingList) *List {
	return &List{underylingList: l}
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

func (l *List) set(ctx *Context, i int, v Value) {
	prevElement := l.underylingList.At(ctx, i)
	l.underylingList.set(ctx, i, v)

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
			prevElement := l.underylingList.At(ctx, i)
			l.removeElementMutationCallbackNoLock(ctx, i, prevElement.(Serializable))
		}
	}

	l.underylingList.SetSlice(ctx, start, end, seq)

	if l.elementMutationCallbacks != nil {
		for i := start; i < end; i++ {
			l.addElementMutationCallbackNoLock(ctx, i, l.underylingList.At(ctx, i))
		}
	}

	path := Path("/" + strconv.Itoa(int(start)) + ".." + strconv.Itoa(int(end-1)))
	mutation := NewSetSliceAtRangeMutation(ctx, NewIncludedEndIntRange(int64(start), int64(end-1)), seq.(Serializable), ShallowWatching, path)

	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
	l.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (l *List) insertElement(ctx *Context, v Value, i Int) {
	l.underylingList.insertElement(ctx, v, i)

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
	l.underylingList.insertSequence(ctx, seq, i)

	if l.elementMutationCallbacks != nil {
		seqLen := seq.Len()
		l.elementMutationCallbacks = slices.Insert(l.elementMutationCallbacks, int(i), makeMutationCallbackHandles(seqLen)...)

		for index := i; index < i+Int(seqLen); i++ {
			l.addElementMutationCallbackNoLock(ctx, int(i), seq.At(ctx, int(index)))
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

func (l *List) removePosition(ctx *Context, i Int) {
	l.underylingList.removePosition(ctx, i)

	if l.elementMutationCallbacks != nil {
		l.removeElementMutationCallbackNoLock(ctx, int(i), l.underylingList.At(ctx, int(i)).(Serializable))
		l.elementMutationCallbacks = slices.Replace(l.elementMutationCallbacks, int(i), int(i+1))
	}

	mutation := NewRemovePositionMutation(ctx, int(i), ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) removePositionRange(ctx *Context, r IntRange) {
	l.underylingList.removePositionRange(ctx, r)

	if l.elementMutationCallbacks != nil {
		for index := int(r.Start); index < int(r.End); index++ {
			l.removeElementMutationCallbackNoLock(ctx, index, l.underylingList.At(ctx, index).(Serializable))
		}

		l.elementMutationCallbacks = slices.Replace(l.elementMutationCallbacks, int(r.Start), int(r.End))
	}

	path := Path("/" + strconv.Itoa(int(r.KnownStart())) + ".." + strconv.Itoa(int(r.InclusiveEnd())))
	mutation := NewRemovePositionRangeMutation(ctx, r, ShallowWatching, path)

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

// Tuple is the immutable equivalent of a List, Tuple implements Value.
type Tuple struct {
	elements     []Serializable
	constraintId ConstraintId
}

func NewTuple(elements []Serializable) *Tuple {
	for i, e := range elements {
		if e.IsMutable() {
			panic(fmt.Errorf("value at index [%d] is mutable", i))
		}
	}
	return &Tuple{elements: elements}
}

func NewTupleVariadic(elements ...Serializable) *Tuple {
	for i, e := range elements {
		if e.IsMutable() {
			panic(fmt.Errorf("value at index [%d] is mutable", i))
		}
	}
	return &Tuple{elements: elements}
}

// the caller can modify the result
func (tuple *Tuple) GetOrBuildElements(ctx *Context) []Serializable {
	return utils.CopySlice(tuple.elements)
}

func (tuple *Tuple) ContainsSimple(ctx *Context, v Value) bool {
	if !IsSimpleInoxVal(v) {
		panic("only simple values are expected")
	}

	for _, e := range tuple.elements {
		if v.Equal(nil, e, map[uintptr]uintptr{}, 0) {
			return true
		}
	}
	return false
}

func (tuple *Tuple) slice(start, end int) Sequence {
	return &Tuple{elements: tuple.elements[start:end]}
}

func (tuple *Tuple) Len() int {
	return len(tuple.elements)
}

func (tuple *Tuple) At(ctx *Context, i int) Value {
	return tuple.elements[i]
}

func (tuple *Tuple) Concat(other *Tuple) *Tuple {
	elements := make([]Serializable, len(tuple.elements)+len(other.elements))

	copy(elements, tuple.elements)
	copy(elements[len(tuple.elements):], other.elements)

	return NewTuple(elements)
}

// UData is used to represent any hiearchical data, UData implements Value and is immutable.
type UData struct {
	Root            Serializable
	HiearchyEntries []UDataHiearchyEntry
}

// UDataHiearchyEntry represents a hiearchical entry in a Udata,
// UDataHiearchyEntry implements Value but is never accessible by Inox code.
type UDataHiearchyEntry struct {
	Value    Serializable
	Children []UDataHiearchyEntry
}

func (d *UData) getEntryAtIndexes(indexesAfterRoot ...int32) (UDataHiearchyEntry, bool) {

	if len(indexesAfterRoot) == 0 {
		return UDataHiearchyEntry{}, false
	}

	firstIndex := int(indexesAfterRoot[0])
	if firstIndex >= len(d.HiearchyEntries) {
		return UDataHiearchyEntry{}, false
	}

	entry := d.HiearchyEntries[firstIndex]

	for _, index := range indexesAfterRoot[1:] {
		if int(index) >= len(entry.Children) {
			return UDataHiearchyEntry{}, false
		}
		entry = entry.Children[index]
	}
	return entry, true
}

func (d *UData) WalkEntriesDF(fn func(e UDataHiearchyEntry, index int, ancestorChain *[]UDataHiearchyEntry) error) error {
	var ancestorChain []UDataHiearchyEntry
	for i, child := range d.HiearchyEntries {
		if err := child.walkEntries(&ancestorChain, i, fn); err != nil {
			return err
		}
	}
	return nil
}

func (e UDataHiearchyEntry) walkEntries(ancestorChain *[]UDataHiearchyEntry, index int, fn func(e UDataHiearchyEntry, index int, ancestorChain *[]UDataHiearchyEntry) error) error {
	fn(e, index, ancestorChain)

	*ancestorChain = append(*ancestorChain, e)
	defer func() {
		*ancestorChain = (*ancestorChain)[:len(*ancestorChain)-1]
	}()

	for i, child := range e.Children {
		if err := child.walkEntries(ancestorChain, i, fn); err != nil {
			return err
		}
	}
	return nil
}

func sortProps[V Value](keys []string, values []V) ([]string, []V, []int) {
	if len(keys) == 0 {
		return nil, nil, nil
	}
	newKeys := utils.CopySlice(keys)
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
