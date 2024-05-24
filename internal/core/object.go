package core

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/inoxconsts"
	utils "github.com/inoxlang/inox/internal/utils/common"
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
	url URL //can be empty

	sysgraph SystemGraphPointer

	constraintId ConstraintId
	visibilityId VisibilityId

	//Locking and transaction related fields

	lock SmartLock
	//pendingChanges []pendingObjectEntryChange //only visible by the current read-write tx
	//TODO: make sure the .IsEmpty and .Contains methods use them.

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

// helper function to create an object, message handlers are added.
func NewObjectFromMap(valMap ValMap, ctx *Context) *Object {
	obj := objFrom(valMap)
	obj.addMessageHandlers(ctx) // add handlers before because jobs can mutate the object
	return obj
}

// helper function to create an object, message handlers are not added.
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

// helper function to create an object, system parts are NOT initialized
func objFrom(entryMap ValMap) *Object {
	keys := make([]string, len(entryMap))
	values := make([]Serializable, len(entryMap))

	i := 0
	for k, v := range entryMap {
		keys[i] = k
		values[i] = v
		i++
	}

	obj := &Object{keys: keys, values: values}
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

func (obj *Object) indexOfKey(k string) int {
	for i, key := range obj.keys {
		if key == k {
			return i
		}
	}
	panic(fmt.Errorf("unknown key %s", k))
}

// this function is called during object creation
func (obj *Object) addMessageHandlers(ctx *Context) error {
	var handlers []*SynchronousMessageHandler

	for keyIndex, key := range obj.keys {
		if key != inoxconsts.IMPLICIT_PROP_NAME {
			continue
		}

		list, ok := obj.values[keyIndex].(*List)
		if !ok {
			return nil
		}

		for elemIndex := 0; elemIndex < list.Len(); elemIndex++ {
			if handler, ok := list.At(ctx, elemIndex).(*SynchronousMessageHandler); ok {
				handlers = append(handlers, handler)
			}
		}
	}

	if len(handlers) != 0 {
		obj.ensureAdditionalFields()
		obj.messageHandlers = NewSynchronousMessageHandlers(handlers...)
	}

	return nil
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

func (obj *Object) _lock(state *GlobalState) {
	if obj.additionalObjectFields == nil { //not shared.
		return
	}
	obj.lock.Lock(state, obj)
}

func (obj *Object) _unlock(state *GlobalState) {
	if obj.additionalObjectFields == nil { //not shared.
		return
	}
	obj.lock.Unlock(state, obj)
}

func (obj *Object) SmartLock(state *GlobalState) {
	obj.ensureAdditionalFields()
	obj.lock.Lock(state, obj, true)
}

func (obj *Object) SmartUnlock(state *GlobalState) {
	if obj.additionalObjectFields == nil {
		panic(errors.New("unexpected Object.SmartUnlock call: object is not locked because it does not have additional fields"))
	}
	obj.lock.Unlock(state, obj, true)
}

func (obj *Object) Prop(ctx *Context, name string) Value {
	return obj.prop(ctx, name, true)
}

func (obj *Object) PropNotStored(ctx *Context, name string) Value {
	return obj.prop(ctx, name, false)
}

func (obj *Object) prop(ctx *Context, name string, stored bool) (returnedValue Value) {

	closestState := ctx.MustGetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)

	if obj.hasAdditionalFields() {
		if obj.url != "" {
			perm := DatabasePermission{
				Kind_:  permbase.Read,
				Entity: obj.url.ToDirURL().AppendRelativePath("./" + Path(name)),
			}

			if err := ctx.CheckHasPermission(perm); err != nil {
				panic(err)
			}
		}
		// //If the current transaction is read-write we look at the pending changes
		// //before checking the actual entries.
		// if tx != nil && !tx.IsReadonly() {
		// 	for _, change := range obj.pendingChanges {
		// 		if change.key != name {
		// 			continue
		// 		}
		// 		if change.isDeletion {
		// 			panic(FormatErrPropertyDoesNotExist(name, obj))
		// 		}
		// 		returnedValue = change.value
		// 		break
		// 	}
		// }
	}

	//Iterate over entries.
	if returnedValue == nil {
		for i, key := range obj.keys {
			if key == name {
				returnedValue = obj.values[i]
				break
			}
		}
	}

	if returnedValue == nil {
		panic(FormatErrPropertyDoesNotExist(name, obj))
	}

	if obj.IsShared() && stored {
		//We use ShareOrClone and not CheckSharedOrClone because a not-yet-shared value (at any depth) may have been added
		//during a previous PropNotStored call.
		returnedValue = utils.Must(ShareOrClone(returnedValue, closestState)).(Serializable)
	}

	return
}

func (obj *Object) SetProp(ctx *Context, name string, value Value) error {

	serializableVal, ok := value.(Serializable)
	if !ok {
		return fmt.Errorf("value is not serializable")
	}

	closestState := ctx.MustGetClosestState()

	if obj.IsShared() {
		newVal, err := ShareOrClone(value, closestState)
		if err != nil {
			return fmt.Errorf("failed to share/clone value when setting property %s: %w", name, err)
		}
		serializableVal = newVal.(Serializable)
	}

	unlock := true
	obj._lock(closestState)
	defer func() {
		if unlock {
			obj._unlock(closestState)
		}
	}()

	var constraint Pattern
	if obj.hasAdditionalFields() && obj.constraintId.HasConstraint() {
		constraint, _ = GetConstraint(obj.constraintId)
	}

	if obj.hasAdditionalFields() && obj.url != "" {
		perm := DatabasePermission{
			Kind_:  permbase.Write,
			Entity: obj.url.ToDirURL().AppendRelativePath("./" + Path(name)),
		}

		if err := ctx.CheckHasPermission(perm); err != nil {
			return err
		}
	}

	tx := ctx.GetTx()

	//If the object is an entity and we are in a transaction we update the pending changes instead of directly mutating the entries.
	//If there is already a change for the entry we update it, else we add a new change.
	if tx != nil && obj.hasURL() {
		if tx.IsReadonly() {
			//TODO: also prevent deep mutation
			panic(fmt.Errorf("cannot mutate object (entity): %w", ErrEffectsNotAllowedInReadonlyTransaction))
		}

		// obj.ensureAdditionalFields()

		// needToAddChange := true

		// for i, change := range obj.pendingChanges {
		// 	if change.key != name {
		// 		continue
		// 	}
		// 	needToAddChange = false
		// 	if change.isDeletion {
		// 		obj.pendingChanges[i].isDeletion = false
		// 		obj.pendingChanges[i].value = serializableVal
		// 	} else {
		// 		obj.pendingChanges[i].value = serializableVal
		// 	}
		// 	break
		// }

		// if needToAddChange {
		// 	obj.pendingChanges = append(obj.pendingChanges, pendingObjectEntryChange{
		// 		key:   name,
		// 		value: serializableVal,
		// 	})
		// }

		//TODO: on transaction rollback: remove all pending changes.
		//TODO: on transaction validation (if constraints): apply changes, check constraints.
		//If fail: reverse changes
		//If success: inform watchers and call mutation callbacks
		//if no constraints: apply changes on commit
		//return nil
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
					obj._unlock(closestState)

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
			obj._unlock(closestState)

			obj.mutationCallbacks.CallMicrotasks(ctx, mutation)
		}
	}

	return nil
}

func (obj *Object) PropertyNames(ctx *Context) []string {

	closestState := ctx.MustGetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)
	return obj.keys
}

func (obj *Object) HasProp(ctx *Context, name string) bool {

	closestState := ctx.MustGetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)
	for _, k := range obj.keys {
		if k == name {
			return true
		}
	}
	return false
}

func (obj *Object) HasPropValue(ctx *Context, value Value) bool {

	closestState := ctx.MustGetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)
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

		closestState := ctx.MustGetClosestState()
		obj._lock(closestState)
		defer obj._unlock(closestState)
	} else if obj.IsShared() {
		panic(errors.New("nil context"))
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

		closestState := ctx.MustGetClosestState()
		obj._lock(closestState)
		defer obj._unlock(closestState)
	} else if obj.IsShared() {
		panic(errors.New("nil context"))
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

// ForEachElement iterates over the elements in the empty "" property, if the property's value is not a list
// the function does nothing.
func (obj *Object) ForEachElement(ctx *Context, fn func(index int, v Serializable) error) error {
	if obj.IsShared() {
		panic(errors.New("Object.ForEachElement() can only be called on objects that are not shared"))
	}

	for i, v := range obj.values {
		key := obj.keys[i]
		if key != inoxconsts.IMPLICIT_PROP_NAME {
			continue
		}

		list, ok := v.(*List)
		if !ok {
			return nil
		}
		for elemIndex := 0; elemIndex < list.Len(); elemIndex++ {
			if err := fn(elemIndex, list.At(ctx, elemIndex).(Serializable)); err != nil {
				return err
			}
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

func (obj *Object) hasURL() bool {
	_, ok := obj.URL()
	return ok
}

func (obj *Object) SetURLOnce(ctx *Context, u URL) error {
	closestState := ctx.MustGetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)

	obj.ensureAdditionalFields()

	if obj.url != "" {
		return ErrURLAlreadySet
	}

	obj.url = u
	return nil
}

func (obj *Object) Keys(ctx *Context) []string {

	closestState := ctx.MustGetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)

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

// A pendingObjectEntryChange represents a change for a shared object's entry,
// it is an update or a deletion.
type pendingObjectEntryChange struct {
	isDeletion bool //update if false
	key        string
	value      Serializable //nil if deletion
}
