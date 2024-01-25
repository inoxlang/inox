package core

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"

	"github.com/inoxlang/inox/internal/commonfmt"
	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

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
	txIsolator     StrongTransactionIsolator
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

func (obj *Object) waitForOtherTxsToTerminate(ctx *Context, requiredRunningTx bool) (currentTx *Transaction) {
	if !obj.hasAdditionalFields() {
		return
	}
	tx, err := obj.txIsolator.WaitForOtherTxsToTerminate(ctx, requiredRunningTx)
	if err != nil {
		panic(err)
	}
	return tx
}

func (obj *Object) Prop(ctx *Context, name string) Value {
	return obj.prop(ctx, name, true)
}

func (obj *Object) PropNotStored(ctx *Context, name string) Value {
	return obj.prop(ctx, name, false)
}

func (obj *Object) prop(ctx *Context, name string, stored bool) Value {
	obj.waitForOtherTxsToTerminate(ctx, !stored)

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

	obj.waitForOtherTxsToTerminate(ctx, false)

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
	obj.waitForOtherTxsToTerminate(ctx, false)

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	return obj.keys
}

func (obj *Object) HasProp(ctx *Context, name string) bool {
	obj.waitForOtherTxsToTerminate(ctx, false)

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
	obj.waitForOtherTxsToTerminate(ctx, false)

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
		obj.waitForOtherTxsToTerminate(ctx, false)

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
		obj.waitForOtherTxsToTerminate(ctx, false)

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
	obj.waitForOtherTxsToTerminate(ctx, false)

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	return obj.keys
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
