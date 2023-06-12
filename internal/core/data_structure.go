package core

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/inoxlang/inox/internal/commonfmt"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []underylingList{&ValueList{}, &IntList{}}
)

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

	watchers              *ValueWatchers
	mutationCallbacks     *MutationCallbacks
	messageHandlers       *SynchronousMessageHandlers
	watchingDepth         WatchingDepth
	propMutationCallbacks []CallbackHandle

	jobs *ValueLifetimeJobs

	keys   []string
	values []Value

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
		values: make([]Value, count),
	}
}

type ValMap map[string]Value

// helper function to create an object, lifetime jobs and system parts are NOT initialized
func objFrom(entryMap ValMap) *Object {
	keys := make([]string, len(entryMap))
	values := make([]Value, len(entryMap))

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

func objFromLists(keys []string, values []Value) *Object {

	//handle index keys ? or ignore

	obj := &Object{keys: keys, values: values}
	obj.sortProps()
	return obj
}

func (obj *Object) sortProps() {
	obj.keys, obj.values = sortProps(obj.keys, obj.values)
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
		jobs := NewValueLifetimeJobs(obj, jobs)
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

func (obj *Object) Prop(ctx *Context, name string) Value {
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
	closestState := ctx.GetClosestState()

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

			obj.values[i] = value

			// check constraints

			if constraint != nil && !constraint.(*ObjectPattern).Test(ctx, obj) {
				obj.values[i] = prevValue
				return ErrConstraintViolation
			}

			// update object

			obj.sortProps()

			mutation := NewUpdatePropMutation(ctx, name, value, ShallowWatching, Path("/"+name))

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
	obj.values = append(obj.values, value)

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

	//inform watchers & microtasks about the update

	mutation := NewAddPropMutation(ctx, name, value, ShallowWatching, Path("/"+name))
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
	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	return obj.keys
}

func (obj *Object) HasProp(ctx *Context, name string) bool {
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

func (obj *Object) EntryMap() map[string]Value {
	if obj == nil {
		return nil
	}
	obj.Lock(nil)
	defer obj.Unlock(nil)

	map_ := map[string]Value{}
	for i, v := range obj.values {
		map_[obj.keys[i]] = v
	}
	return map_
}

// Indexed returns the list of indexed properties
func (obj *Object) Indexed() []Value {
	if obj.IsShared() {
		panic(errors.New("Object.Indexed() can only be called on objects that are not shared"))
	}

	values := make([]Value, obj.implicitPropCount)

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

func (obj *Object) ForEachEntry(fn func(k string, v Value) error) error {
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

func (obj *Object) Keys() []string {
	return obj.keys
}

// Record is the immutable equivalent of an Object, Record implements Value.
type Record struct {
	implicitPropCount int //TODO: rename to indexedPropCount ? forbid explicit index keys ?
	visibilityId      VisibilityId
	url               URL //can be empty
	keys              []string
	values            []Value
}

func NewEmptyRecord() *Record {
	return &Record{}
}

func NewRecordFromMap(entryMap ValMap) *Record {
	keys := make([]string, len(entryMap))
	values := make([]Value, len(entryMap))

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

func NewRecordFromKeyValLists(keys []string, values []Value) *Record {
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

func (rec *Record) ForEachEntry(fn func(k string, v Value) error) error {
	for i, v := range rec.values {
		if err := fn(rec.keys[i], v); err != nil {
			return err
		}
	}
	return nil
}

func (rec *Record) sortProps() {
	rec.keys, rec.values = sortProps(rec.keys, rec.values)
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

func (rec *Record) EntryMap() map[string]Value {
	if rec == nil {
		return nil
	}
	map_ := map[string]Value{}
	for i, v := range rec.values {
		map_[rec.keys[i]] = v
	}
	return map_
}

// A Dictionnary maps representable values (keys) to any values, Dictionar implements Value.
type Dictionary struct {
	Entries map[string]Value
	Keys    map[string]Value
}

func convertKeyReprToValue(repr string) Value {
	keyNode, ok := parse.ParseExpression(repr)
	if !ok {
		panic(fmt.Errorf("invalid key representation: %s", repr))
	}
	//TODO: refactor
	key, err := TreeWalkEval(keyNode, NewTreeWalkStateWithGlobal(&GlobalState{}))
	if err != nil {
		panic(err)
	}
	return key
}

func NewDictionary(entries ValMap) *Dictionary {
	dict := &Dictionary{
		Entries: map[string]Value{},
		Keys:    map[string]Value{},
	}
	for keyRepresentation, v := range entries {
		dict.Entries[keyRepresentation] = v
		dict.Keys[keyRepresentation] = convertKeyReprToValue(keyRepresentation)
	}

	return dict
}

func (d *Dictionary) Value(ctx *Context, key Value) (Value, bool) {
	if !key.HasRepresentation(map[uintptr]int{}, nil) {
		return nil, false
	}
	v, ok := d.Entries[string(GetRepresentation(key, ctx))]
	return v, ok
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

	lock              sync.Mutex
	mutationCallbacks *MutationCallbacks
	watchers          *ValueWatchers
}

func newList(underylingList underylingList) *List {
	return &List{underylingList: underylingList}
}

func WrapUnderylingList(l underylingList) *List {
	return &List{underylingList: l}
}

// the caller can modify the result.
func (list *List) GetOrBuildElements(ctx *Context) []Value {
	entries := IterateAll(ctx, list.Iterator(ctx, IteratorConfiguration{}))

	values := make([]Value, len(entries))
	for i, e := range entries {
		values[i] = e[1]
	}
	return values
}

func (l *List) set(ctx *Context, i int, v Value) {
	l.underylingList.set(ctx, i, v)

	mutation := NewSetElemAtIndexMutation(ctx, i, v, ShallowWatching, Path("/"+strconv.Itoa(i)))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) insertElement(ctx *Context, v Value, i Int) {
	l.underylingList.insertElement(ctx, v, i)

	mutation := NewInsertElemAtIndexMutation(ctx, int(i), v, ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

// Tuple is the immutable equivalent of a List, Tuple implements Value.
type Tuple struct {
	elements     []Value
	constraintId ConstraintId
}

func NewTuple(elements []Value) *Tuple {
	for i, e := range elements {
		if e.IsMutable() {
			panic(fmt.Errorf("value at index [%d] is mutable", i))
		}
	}
	return &Tuple{elements: elements}
}

func NewTupleVariadic(elements ...Value) *Tuple {
	for i, e := range elements {
		if e.IsMutable() {
			panic(fmt.Errorf("value at index [%d] is mutable", i))
		}
	}
	return &Tuple{elements: elements}
}

// the caller can modify the result
func (tuple *Tuple) GetOrBuildElements(ctx *Context) []Value {
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
	elements := make([]Value, len(tuple.elements)+len(other.elements))

	copy(elements, tuple.elements)
	copy(elements[len(tuple.elements):], other.elements)

	return NewTuple(elements)
}

// UData is used to represent any hiearchical data, UData implements Value and is immutable.
type UData struct {
	NoReprMixin
	Root            Value
	HiearchyEntries []UDataHiearchyEntry
}

// UDataHiearchyEntry represents a hiearchical entry in a Udata,
// UDataHiearchyEntry implements Value but is never accessible by Inox code.
type UDataHiearchyEntry struct {
	NoReprMixin
	Value    Value
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

func sortProps(keys []string, values []Value) ([]string, []Value) {
	if len(keys) == 0 {
		return nil, nil
	}
	newKeys := utils.CopySlice(keys)
	sort.Strings(newKeys)
	newValues := make([]Value, len(values))

	for i := 0; i < len(keys); i++ {
		newIndex := sort.SearchStrings(newKeys, keys[i])
		newValues[newIndex] = values[i]
	}

	return newKeys, newValues
}
