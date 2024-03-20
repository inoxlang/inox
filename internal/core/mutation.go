package core

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_MICROTASK_ARRAY_SIZE = 4
)

var (
	MUTATION_KIND_NAMES = [...]string{
		UnspecifiedMutation:   "unspecified-mutation",
		AddProp:               "add-prop",
		UpdateProp:            "update-prop",
		AddEntry:              "add-entry",
		UpdateEntry:           "update-entry",
		InsertElemAtIndex:     "insert-elem-at-index",
		SetElemAtIndex:        "set-elem-at-index",
		SetSliceAtRange:       "set-slice-at-range",
		InsertSequenceAtIndex: "insert-seq-at-index",
		RemovePosition:        "remove-position",
		RemovePositions:       "remove-positions",
		RemovePositionRange:   "remove-position-range",
		SpecificMutation:      "specific-mutation",
	}

	ErrCannotApplyIncompleteMutation = errors.New("cannot apply an incomplete mutation")
	ErrNotSupportedSpecificMutation  = errors.New("not supported specific mutation")
	ErrInvalidMutationPrefixSymbol   = errors.New("invalid mutation prefix symbol")

	mutationCallbackPool     *ArrayPool[mutationCallback]
	mutationCallbackPoolLock sync.Mutex

	_ = []Value{Mutation{}}
)

func init() {
	resetMutationCallbackPool()
}

func resetMutationCallbackPool() {
	if testing.Testing() {
		current := mutationCallbackPool
		if current != nil {
			current.lock.Lock()
			defer current.lock.Unlock()

		}

		mutationCallbackPoolLock.Lock()
		defer mutationCallbackPoolLock.Unlock()
	} else if mutationCallbackPool != nil {
		panic(errors.New("resetMutationCallbackPool is only available for testing"))
	}

	mutationCallbackPool = utils.Must(NewArrayPool[mutationCallback](100_000, 10, func(mc *mutationCallback) {
		*mc = mutationCallback{}
	}))
}

// A Mutation stores the data (or part of the data) about the modification of a value, it is immutable and implements Value.
type Mutation struct {
	Kind                    MutationKind
	Complete                bool                    // true if the Mutation contains all the data necessary to be applied
	SpecificMutationVersion SpecificMutationVersion // set only if specific mutation
	SpecificMutationKind    SpecificMutationKind    // set only if specific mutation
	Tx                      *Transaction

	Data               []byte
	DataElementLengths [6]int32
	Path               Path // can be empty
	Depth              WatchingDepth
}

type SpecificMutationVersion int8
type SpecificMutationKind int8

type SpecificMutationAcceptor interface {
	Value
	// ApplySpecificMutation should apply the mutation to the Value, ErrNotSupportedSpecificMutation should be returned
	// if it's not possible.
	ApplySpecificMutation(ctx *Context, m Mutation) error
}

func WriteSingleJSONRepresentation(ctx *Context, v Serializable) ([]byte, [6]int32, error) {
	config := JSONSerializationConfig{ReprConfig: &ReprConfig{AllVisible: true}}
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

	err := v.WriteJSONRepresentation(ctx, stream, config, 0)
	if err != nil {
		return nil, [6]int32{}, err
	}
	buf := stream.Buffer()
	return buf, [6]int32{int32(len(buf))}, nil
}

func WriteConcatenatedRepresentations(ctx *Context, values ...Serializable) ([]byte, [6]int32, error) {
	config := JSONSerializationConfig{ReprConfig: &ReprConfig{AllVisible: true}}
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

	var sizes [6]int32

	if len(values) > len(sizes) {
		panic(fmt.Errorf("too many representations to write: %d", len(values)))
	}

	for i, val := range values {
		prevBufSize := len(stream.Buffer())

		err := val.WriteJSONRepresentation(ctx, stream, config, 0)
		if err != nil {
			return nil, [6]int32{}, err
		}

		elemSize := len(stream.Buffer()) - prevBufSize
		sizes[i] = int32(elemSize)
	}

	buf := stream.Buffer()
	return buf, sizes, nil
}

func NewUnspecifiedMutation(depth WatchingDepth, path Path) Mutation {
	return Mutation{
		Kind:     UnspecifiedMutation,
		Complete: false,
		Depth:    depth,
		Path:     path,
	}
}

func NewAddPropMutation(ctx *Context, name string, value Serializable, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteConcatenatedRepresentations(ctx, String(name), value)

	return Mutation{
		Kind:               AddProp,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewUpdatePropMutation(ctx *Context, name string, newValue Serializable, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteConcatenatedRepresentations(ctx, String(name), newValue)

	return Mutation{
		Kind:               UpdateProp,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		//TODO: Data1
		Path: path,
	}
}

func NewAddEntryMutation(ctx *Context, key, value Serializable, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteConcatenatedRepresentations(ctx, key, value)

	return Mutation{
		Kind:               AddEntry,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewUpdateEntryMutation(ctx *Context, key, newValue Serializable, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteConcatenatedRepresentations(ctx, key, newValue)

	return Mutation{
		Kind:               UpdateEntry,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		//TODO: Data1
		Path: path,
	}
}

func NewSetElemAtIndexMutation(ctx *Context, index int, elem Serializable, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteConcatenatedRepresentations(ctx, Int(index), elem)

	return Mutation{
		Kind:               SetElemAtIndex,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewSetSliceAtRangeMutation(ctx *Context, intRange IntRange, slice Serializable, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteConcatenatedRepresentations(ctx, intRange, slice)

	return Mutation{
		Kind:               SetSliceAtRange,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewInsertElemAtIndexMutation(ctx *Context, index int, elem Serializable, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteConcatenatedRepresentations(ctx, Int(index), elem)

	return Mutation{
		Kind:               InsertElemAtIndex,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewInsertSequenceAtIndexMutation(ctx *Context, index int, seq Sequence, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteConcatenatedRepresentations(ctx, Int(index), seq.(Serializable))

	return Mutation{
		Kind:               InsertSequenceAtIndex,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewRemovePositionMutation(ctx *Context, index int, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteSingleJSONRepresentation(ctx, Int(index))

	return Mutation{
		Kind:               RemovePosition,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewRemovePositionsMutation(ctx *Context, indexes []Int, depth WatchingDepth, path Path) Mutation {
	indexList := NewWrappedIntListFrom(indexes)
	//TODO: do not create a temporary list
	data, sizes, err := WriteSingleJSONRepresentation(ctx, indexList)

	return Mutation{
		Kind:               RemovePositions,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewRemovePositionRangeMutation(ctx *Context, intRange IntRange, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteSingleJSONRepresentation(ctx, intRange)

	return Mutation{
		Kind:               RemovePositionRange,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

type SpecificMutationMetadata struct {
	Version SpecificMutationVersion
	Kind    SpecificMutationKind //Depends on the value on which the mutation is applied.
	Depth   WatchingDepth
	Path    Path
}

func NewSpecificMutation(ctx *Context, meta SpecificMutationMetadata, values ...Serializable) Mutation {
	data, sizes, err := WriteConcatenatedRepresentations(ctx, values...)

	return Mutation{
		Kind:                    SpecificMutation,
		SpecificMutationVersion: meta.Version,
		SpecificMutationKind:    meta.Kind,
		Complete:                err == nil,
		Data:                    data,
		DataElementLengths:      sizes,
		Depth:                   meta.Depth,
		Path:                    meta.Path,
	}
}

func NewSpecificIncompleteNoDataMutation(meta SpecificMutationMetadata) Mutation {
	return Mutation{
		Kind:                    SpecificMutation,
		SpecificMutationVersion: meta.Version,
		SpecificMutationKind:    meta.Kind,
		Complete:                false,
		Depth:                   meta.Depth,
		Path:                    meta.Path,
	}
}

func (m Mutation) Relocalized(parent Path) Mutation {
	new := m
	new.Path = parent + m.Path

	slashCount := strings.Count(string(parent), "/")
	newDepth, ok := m.Depth.Plus(uint(slashCount))
	if !ok {
		panic(fmt.Errorf("failed to relocalize mutation %#v, parent path is '%s'", m, parent))
	}
	new.Depth = newDepth

	return new
}

func (m Mutation) DataElem(ctx *Context, index int) Value {
	if index >= 6 || index < 0 {
		panic(ErrUnreachable)
	}
	length := m.DataElementLengths[index]
	start := int32(0)
	for i := 0; i < index; i++ {
		start += m.DataElementLengths[i]
	}

	b := m.Data[start : start+length]
	v, err := ParseJSONRepresentation(ctx, string(b), nil)
	//TODO: cache result (evict quickly)
	if err != nil {
		panic(err)
	}
	return v
}

func (m Mutation) AffectedProperty(ctx *Context) string {
	switch m.Kind {
	case AddProp, UpdateProp:
		return string(m.DataElem(ctx, 0).(String))
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) AffectedIndex(ctx *Context) Int {
	switch m.Kind {
	case InsertElemAtIndex, SetElemAtIndex, InsertSequenceAtIndex, RemovePosition:
		return m.DataElem(ctx, 0).(Int)
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) AffectedRange(ctx *Context) IntRange {
	switch m.Kind {
	case SetSliceAtRange, RemovePositionRange:
		return m.DataElem(ctx, 0).(IntRange)
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) PropValue(ctx *Context) Value {
	switch m.Kind {
	case AddProp, UpdateProp:
		return m.DataElem(ctx, 1)
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) Element(ctx *Context) Value {
	switch m.Kind {
	case InsertElemAtIndex, SetElemAtIndex:
		return m.DataElem(ctx, 1)
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) Sequence(ctx *Context) Sequence {
	switch m.Kind {
	case InsertSequenceAtIndex:
		return m.DataElem(ctx, 1).(Sequence)
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) ApplyTo(ctx *Context, v Value) error {
	if !m.Complete {
		return ErrCannotApplyIncompleteMutation
	}
	switch m.Kind {
	case AddProp:
		//TODO: check that property does not exist + add it in a same atomic operation
		v.(IProps).SetProp(ctx, m.AffectedProperty(ctx), m.PropValue(ctx))
	case UpdateProp:
		v.(IProps).SetProp(ctx, m.AffectedProperty(ctx), m.PropValue(ctx))
	case SetElemAtIndex:
		v.(MutableLengthSequence).set(ctx, int(m.AffectedIndex(ctx)), m.Element(ctx))
	case SetSliceAtRange:
		intRange := m.AffectedRange(ctx)
		v.(MutableLengthSequence).SetSlice(ctx, int(intRange.start), int(intRange.end), m.Element(ctx).(Sequence))
	case InsertElemAtIndex:
		v.(MutableLengthSequence).insertElement(ctx, m.Element(ctx), Int(m.AffectedIndex(ctx)))
	case InsertSequenceAtIndex:
		v.(MutableLengthSequence).insertSequence(ctx, m.Sequence(ctx), Int(m.AffectedIndex(ctx)))
	case RemovePosition:
		v.(MutableLengthSequence).removePosition(ctx, Int(m.AffectedIndex(ctx)))
	case RemovePositionRange:
		v.(MutableLengthSequence).removePositionRange(ctx, m.AffectedRange(ctx))
	case SpecificMutation:
		return v.(SpecificMutationAcceptor).ApplySpecificMutation(ctx, m)
	default:
		panic(ErrUnreachable)
	}
	return nil
}

type MutationKind int

const (
	UnspecifiedMutation MutationKind = iota + 1
	AddProp
	UpdateProp
	AddEntry
	UpdateEntry
	InsertElemAtIndex
	SetElemAtIndex
	SetSliceAtRange
	InsertSequenceAtIndex
	RemovePosition
	RemovePositions
	RemovePositionRange
	SpecificMutation
)

func (k MutationKind) String() string {
	return MUTATION_KIND_NAMES[k-1]
}

func mutationKindFromString(s string) (MutationKind, bool) {
	for i := 0; i < len(MUTATION_KIND_NAMES); i++ {
		if MUTATION_KIND_NAMES[i] == s {
			return MutationKind(i + 1), true
		}
	}
	return 0, false
}

// A Change is an immutable Value that stores the data about a modification (Mutation) and some metadata such as the
// moment in time where the mutation happpened.
type Change struct {
	mutation Mutation
	datetime DateTime
}

func NewChange(mutation Mutation, datetime DateTime) Change {
	return Change{
		mutation: mutation,
		datetime: datetime,
	}
}

func (c Change) DateTime() DateTime {
	return c.datetime
}

// MutationCallbacks is used by watchables that implement OnMutation.
type MutationCallbacks struct {
	ownedSlice bool
	nextIndex  int
	nextHandle CallbackHandle

	callbacks []mutationCallback

	initialized bool
	lock        sync.Mutex
}

type mutationCallback struct {
	config MutationWatchingConfiguration
	fn     MutationCallbackMicrotask
	handle CallbackHandle
}

func NewMutationCallbacks() *MutationCallbacks {
	return &MutationCallbacks{
		nextIndex:  0,
		nextHandle: FIRST_VALID_CALLBACK_HANDLE,
	}
}

func (c *MutationCallbacks) Functions() []mutationCallback {
	return c.callbacks
}

func (c *MutationCallbacks) init() {
	// TODO:
	// Store 1 or 2 callbacks in MutationCallbacks to avoid some allocations,
	// setting a finalizer to release an array with just a few items set makes no sense.

	if mutationCallbackPool.IsFull() {
		c.callbacks = make([]mutationCallback, mutationCallbackPool.arrayLen)
	} else {
		c.callbacks = utils.Must(mutationCallbackPool.GetArray())
	}
	c.initialized = true

	runtime.SetFinalizer(c, func(callbacks *MutationCallbacks) {
		if !callbacks.ownedSlice {
			mutationCallbackPool.ReleaseArray(callbacks.callbacks)
		}
	})
}

func (c *MutationCallbacks) tearDown() {
	c.lock.Lock() // possible deadlock due to microtasks
	defer c.lock.Unlock()

	if !c.ownedSlice {
		mutationCallbackPool.ReleaseArray(c.callbacks)
	}
	c.callbacks = nil
	c.ownedSlice = false
}

func (c *MutationCallbacks) AddMicrotask(m MutationCallbackMicrotask, config MutationWatchingConfiguration) (handle CallbackHandle) {
	if m == nil {
		return
	}

	c.lock.Lock() // possible deadlock due to microtasks
	defer c.lock.Unlock()

	if !c.initialized {
		c.init()
	}

	handle = c.nextHandle
	c.nextHandle++

	callback := mutationCallback{
		fn:     m,
		config: config,
		handle: handle,
	}

	if c.nextIndex >= len(c.callbacks) {
		if c.nextIndex == 0 {
			c.callbacks = utils.Must(mutationCallbackPool.GetArray())
			c.callbacks[0] = callback
		} else if c.ownedSlice {
			c.callbacks = append(c.callbacks, callback)
		} else {
			callbacks := make([]mutationCallback, len(c.callbacks)+1)
			copy(callbacks, c.callbacks)
			mutationCallbackPool.ReleaseArray(c.callbacks)
			c.callbacks = callbacks
			c.ownedSlice = true
		}
	} else {
		c.callbacks[c.nextIndex] = callback
	}
	c.updateNextIndex()

	return
}

func (c *MutationCallbacks) RemoveMicrotasks() {
	if c == nil {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.initialized {
		return
	}

	for i := range c.callbacks {
		c.callbacks[i] = mutationCallback{}
	}

	c.updateNextIndex()
}

func (c *MutationCallbacks) RemoveMicrotask(handle CallbackHandle) {
	if c == nil {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.initialized {
		return
	}

	for i := range c.callbacks {
		if c.callbacks[i].handle == handle {
			c.callbacks[i] = mutationCallback{}
			break
		}
	}

	c.updateNextIndex()
}

// CallMicrotasks calls the registered tasks that have a configured depth greater or equal to the depth at which the mutation
// happened (depth argument).
func (c *MutationCallbacks) CallMicrotasks(ctx *Context, m Mutation) {
	if c == nil {
		return
	}

	c.lock.Lock() // possible deadlock due to microtasks
	defer c.lock.Unlock()

	for i, callback := range c.callbacks {
		if callback.fn == nil || (m.Depth != UnspecifiedWatchingDepth && callback.config.Depth < m.Depth) {
			continue
		}

		func() {
			defer func() {
				if recover() != nil { //TODO: log errors
					c.callbacks[i].fn = nil
				}
			}()
			if !callback.fn(ctx, m) {
				c.callbacks[i].fn = nil
			}
		}()
	}

	c.updateNextIndex()
}

func (c *MutationCallbacks) updateNextIndex() {
	for i, callback := range c.callbacks {
		if callback.fn == nil {
			c.nextIndex = i
			return
		}
	}
	c.nextIndex = len(c.callbacks)
}

func (obj *Object) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	state := ctx.MustGetClosestState()
	obj._lock(state)
	defer obj._unlock(state)

	obj.ensureAdditionalFields()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		if config.Depth > obj.watchingDepth {
			obj.watchingDepth = config.Depth

			if obj.propMutationCallbacks == nil {
				obj.propMutationCallbacks = make([]CallbackHandle, len(obj.keys))
			}

			for i, val := range obj.values {
				if err := obj.addPropMutationCallbackNoLock(ctx, i, val); err != nil {
					return FIRST_VALID_CALLBACK_HANDLE - 1, err
				}
			}
		}
	}

	if obj.mutationCallbacks == nil {
		obj.mutationCallbacks = NewMutationCallbacks()
	}

	handle := obj.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (obj *Object) removePropMutationCallbackNoLock(ctx *Context, index int, previousValue Serializable) {
	obj.ensureAdditionalFields()

	if watchable, ok := previousValue.(Watchable); ok {
		if previousHandle := obj.propMutationCallbacks[index]; previousHandle.Valid() {
			watchable.RemoveMutationCallback(ctx, previousHandle)
			obj.propMutationCallbacks[index] = FIRST_VALID_CALLBACK_HANDLE - 1
		}
	}
}

func (obj *Object) addPropMutationCallbackNoLock(ctx *Context, index int, val Value) error {
	obj.ensureAdditionalFields()

	if watchable, ok := val.(Watchable); ok {
		key := obj.keys[index]

		config := MutationWatchingConfiguration{
			Depth: obj.watchingDepth.MustMinusOne(), // depth at which we watch the property
		}
		path := Path("/" + key)

		handle, err := watchable.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			state := ctx.MustGetClosestState()
			obj._lock(state)
			callbacks := obj.mutationCallbacks
			objWatchingDepth := obj.watchingDepth
			obj._unlock(state)

			if !objWatchingDepth.IsSpecified() { //defensive check
				return
			}

			if mutation.Depth > objWatchingDepth { //defensive check
				return
			}

			callbacks.CallMicrotasks(ctx, mutation.Relocalized(path))

			return
		}, config)

		if err != nil {
			return fmt.Errorf("failed to add mutation callback to property .%s: %w", key, err)
		}
		obj.propMutationCallbacks[index] = handle
	}
	return nil
}

func (obj *Object) RemoveMutationCallbackMicrotasks(ctx *Context) {
	state := ctx.MustGetClosestState()
	obj._lock(state)
	defer obj._unlock(state)

	if !obj.hasAdditionalFields() || obj.mutationCallbacks == nil {
		return
	}

	obj.mutationCallbacks.RemoveMicrotasks()
}

func (obj *Object) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	state := ctx.MustGetClosestState()
	obj._lock(state)
	defer obj._unlock(state)

	obj.ensureAdditionalFields()
	obj.mutationCallbacks.RemoveMicrotask(handle)
}

func (d *Dictionary) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		if config.Depth > d.watchingDepth {
			d.watchingDepth = config.Depth

			if d.entryMutationCallbacks == nil {
				d.entryMutationCallbacks = make(map[string]CallbackHandle, len(d.keys))
			}

			for keyRepr, val := range d.entries {
				if err := d.addEntryMutationCallbackNoLock(ctx, keyRepr, val); err != nil {
					return -1, err
				}
			}
		}
	}

	if d.mutationCallbacks == nil {
		d.mutationCallbacks = NewMutationCallbacks()
	}

	handle := d.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (d *Dictionary) removeEntryMutationCallbackNoLock(ctx *Context, keyRepr string, previousValue Serializable) {
	if watchable, ok := previousValue.(Watchable); ok {
		if previousHandle := d.entryMutationCallbacks[keyRepr]; previousHandle.Valid() {
			watchable.RemoveMutationCallback(ctx, previousHandle)
			d.entryMutationCallbacks[keyRepr] = FIRST_VALID_CALLBACK_HANDLE - 1
		}
	}
}

func (d *Dictionary) addEntryMutationCallbackNoLock(ctx *Context, keyRepr string, val Value) error {
	if watchable, ok := val.(Watchable); ok {
		config := MutationWatchingConfiguration{
			Depth: d.watchingDepth.MustMinusOne(), // depth at which we watch the property
		}
		path := Path("/" + keyRepr)

		handle, err := watchable.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			callbacks := d.mutationCallbacks
			objWatchingDepth := d.watchingDepth

			if !objWatchingDepth.IsSpecified() { //defensive check
				return
			}

			if mutation.Depth > objWatchingDepth { //defensive check
				return
			}

			callbacks.CallMicrotasks(ctx, mutation.Relocalized(path))

			return
		}, config)

		if err != nil {
			return fmt.Errorf("failed to add mutation callback to dictionary key %s: %w", keyRepr, err)
		}
		d.entryMutationCallbacks[keyRepr] = handle
	}
	return nil
}

func (d *Dictionary) RemoveMutationCallbackMicrotasks(ctx *Context) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.mutationCallbacks == nil {
		return
	}

	d.mutationCallbacks.RemoveMicrotasks()
}

func (d *Dictionary) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.mutationCallbacks.RemoveMicrotask(handle)
}

func (l *List) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		if config.Depth > l.watchingDepth {
			l.watchingDepth = config.Depth

			if l.elementMutationCallbacks == nil {
				l.elementMutationCallbacks = make([]CallbackHandle, l.Len())
			}

			for index, elem := range l.GetOrBuildElements(ctx) {
				if err := l.addElementMutationCallbackNoLock(ctx, index, elem); err != nil {
					return -1, err
				}
			}
		}
	}

	if l.mutationCallbacks == nil {
		l.mutationCallbacks = NewMutationCallbacks()
	}

	handle := l.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (l *List) removeElementMutationCallbackNoLock(ctx *Context, index int, previousValue Serializable) {
	if watchable, ok := previousValue.(Watchable); ok {
		if previousHandle := l.elementMutationCallbacks[index]; CallbackHandle(previousHandle).Valid() {
			watchable.RemoveMutationCallback(ctx, CallbackHandle(previousHandle))
			l.elementMutationCallbacks[index] = FIRST_VALID_CALLBACK_HANDLE - 1
		}
	}
}

func (l *List) addElementMutationCallbackNoLock(ctx *Context, index int, elem Value) error {
	if watchable, ok := elem.(Watchable); ok {
		config := MutationWatchingConfiguration{
			Depth: l.watchingDepth.MustMinusOne(), // depth at which we watch the property
		}
		path := Path("/" + strconv.Itoa(index))

		handle, err := watchable.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			callbacks := l.mutationCallbacks
			listWatchingDepth := l.watchingDepth

			if !listWatchingDepth.IsSpecified() { //defensive check
				return
			}

			if mutation.Depth > listWatchingDepth { //defensive check
				return
			}

			callbacks.CallMicrotasks(ctx, mutation.Relocalized(path))

			return
		}, config)

		if err != nil {
			return fmt.Errorf("failed to add mutation callback to element %d: %w", index, err)
		}
		l.elementMutationCallbacks[index] = handle
	}
	return nil
}

func (l *List) RemoveMutationCallbackMicrotasks(ctx *Context) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.mutationCallbacks == nil {
		return
	}

	l.mutationCallbacks.RemoveMicrotasks()
}

func (l *List) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.mutationCallbacks.RemoveMicrotask(handle)
}

func (s *RuneSlice) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		return 0, ErrIntermediateDepthWatchingNotSupported
	}

	if s.mutationCallbacks == nil {
		s.mutationCallbacks = NewMutationCallbacks()
	}

	handle := s.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (s *RuneSlice) RemoveMutationCallbackMicrotasks(ctx *Context) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.mutationCallbacks == nil {
		return
	}

	s.mutationCallbacks.RemoveMicrotasks()
}

func (s *RuneSlice) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.mutationCallbacks.RemoveMicrotask(handle)
}

func (s *ByteSlice) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		return 0, ErrIntermediateDepthWatchingNotSupported
	}

	if s.mutationCallbacks == nil {
		s.mutationCallbacks = NewMutationCallbacks()
	}

	handle := s.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (s *ByteSlice) RemoveMutationCallbackMicrotasks(ctx *Context) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.mutationCallbacks == nil {
		return
	}

	s.mutationCallbacks.RemoveMicrotasks()
}

func (s *ByteSlice) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.mutationCallbacks.RemoveMicrotask(handle)
}

func (dyn *DynamicValue) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	dyn.lock.Lock()
	defer dyn.lock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = DeepWatching
	}

	if dyn.mutationCallbacks == nil {
		dyn.mutationCallbacks = NewMutationCallbacks()
	}

	handle := dyn.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (dyn *DynamicValue) RemoveMutationCallbackMicrotasks(ctx *Context) {
	dyn.lock.Lock()
	defer dyn.lock.Unlock()

	if dyn.mutationCallbacks == nil {
		return
	}

	dyn.mutationCallbacks.RemoveMicrotasks()
}

func (dyn *DynamicValue) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	dyn.lock.Lock()
	defer dyn.lock.Unlock()

	dyn.mutationCallbacks.RemoveMicrotask(handle)
}

func (g *SystemGraph) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if g.mutationCallbacks == nil {
		g.mutationCallbacks = NewMutationCallbacks()
	}

	handle := g.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (g *SystemGraph) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()

	if g.mutationCallbacks == nil {
		return
	}

	g.mutationCallbacks.RemoveMicrotasks()
}

func (g *SystemGraph) RemoveMutationCallbackMicrotasks(ctx *Context) {
	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()

	g.mutationCallbacks.RemoveMicrotasks()
}

func (g *SystemGraph) ApplySpecificMutation(ctx *Context, m Mutation) error {

	if m.SpecificMutationVersion != 1 {
		return ErrNotSupportedSpecificMutation
	}

	switch m.SpecificMutationKind {
	case SG_AddEvent:
		g.nodes.lock.Lock()
		defer g.nodes.lock.Unlock()

		if g.isFrozen {
			return ErrAttemptToMutateFrozenValue
		}

		g.eventLogLock.Lock()
		defer g.eventLogLock.Unlock()

		ptr := uintptr(m.DataElem(ctx, 0).(Int))
		text := string(m.DataElem(ctx, 1).(String))
		g.addEventNoLock(ptr, text)

	case SG_AddNode:
		g.nodes.lock.Lock()
		defer g.nodes.lock.Unlock()

		if g.isFrozen {
			return ErrAttemptToMutateFrozenValue
		}

		name := m.DataElem(ctx, 0).(String)
		typename := m.DataElem(ctx, 1).(String)
		valuePtr := m.DataElem(ctx, 2).(Int)
		parentPtr := m.DataElem(ctx, 3).(Int)

		childNode := g.addNodeNoLock(ctx, uintptr(valuePtr), string(name), string(typename))

		if parentPtr > 0 {
			parentNode, ok := g.nodes.ptrToNode[uintptr(parentPtr)]
			if !ok {
				panic(fmt.Errorf("parent node does not exist"))
			}

			edgeTextOrEdgeTuple := m.DataElem(ctx, 4)
			switch textOrTuple := edgeTextOrEdgeTuple.(type) {
			case String:
				edgeKind := m.DataElem(ctx, 5).(Int)
				g.addEdgeNoLock(string(textOrTuple), childNode, parentNode, SystemGraphEdgeKind(edgeKind))
			case *Tuple:
				for i := 0; i < len(textOrTuple.elements); i += 2 {
					edgeText := textOrTuple.elements[i].(String)
					edgeKind := textOrTuple.elements[i+1].(Int)
					g.addEdgeNoLock(string(edgeText), childNode, parentNode, SystemGraphEdgeKind(edgeKind))
				}
			}

		}
	case SG_AddEdge:
		g.nodes.lock.Lock()
		defer g.nodes.lock.Unlock()

		if g.isFrozen {
			return ErrAttemptToMutateFrozenValue
		}

		edgeSourcePtr := m.DataElem(ctx, 0).(Int)
		edgeTargetPtr := m.DataElem(ctx, 1).(Int)
		edgeText := m.DataElem(ctx, 2).(String)
		edgeKind := m.DataElem(ctx, 3).(Int)

		sourceNode, ok := g.nodes.ptrToNode[uintptr(edgeSourcePtr)]
		if !ok {
			panic(fmt.Errorf("edge's source node does not exist"))
		}

		targetNode, ok := g.nodes.ptrToNode[uintptr(edgeTargetPtr)]
		if !ok {
			panic(fmt.Errorf("edge's parent node does not exist"))
		}

		g.addEdgeWithMutationNoLock(ctx, sourceNode, targetNode, SystemGraphEdgeKind(edgeKind), string(edgeText))
		_ = edgeText
	default:
		panic(ErrUnreachable)
	}

	g.mutationCallbacks.CallMicrotasks(ctx, m)
	return nil

}

func (f *InoxFunction) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	f.mutationFieldsLock.Lock()
	defer f.mutationFieldsLock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		if config.Depth > f.watchingDepth {
			f.watchingDepth = config.Depth

			// if f.compiledFunction != nil {
			// 	for index, localValue := range f.capturedLocals {
			// 		if watchable, ok := localValue.(Watchable); ok {
			// 			//TODO: use variable name instead (if possible)
			// 			path := Path("/" + strconv.Itoa(index))
			// 			f.addCapturedValueMutationCallback(ctx, watchable, path, config)
			// 		}
			// 	}
			// } else {
			for varName, localValue := range f.treeWalkCapturedLocals {
				if watchable, ok := localValue.(Watchable); ok {
					path := Path("/" + varName)
					f.addCapturedValueMutationCallback(ctx, watchable, path, config)
				}
			}
			//}

			for _, capturedGlobal := range f.capturedGlobals {
				if watchable, ok := capturedGlobal.value.(Watchable); ok {
					path := Path("/" + capturedGlobal.name)
					f.addCapturedValueMutationCallback(ctx, watchable, path, config)
				}
			}
		}
	}

	if f.mutationCallbacks == nil {
		f.mutationCallbacks = NewMutationCallbacks()
	}

	handle := f.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (f *InoxFunction) addCapturedValueMutationCallback(ctx *Context, watchable Watchable, path Path, config MutationWatchingConfiguration) (CallbackHandle, error) {
	return watchable.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
		registerAgain = true

		f.mutationFieldsLock.Lock()
		callbacks := f.mutationCallbacks
		functionWatchingDepth := f.watchingDepth
		f.mutationFieldsLock.Unlock()

		if !functionWatchingDepth.IsSpecified() { //defensive check
			return
		}

		if mutation.Depth > functionWatchingDepth { //defensive check
			return
		}

		callbacks.CallMicrotasks(ctx, mutation.Relocalized(path))

		return
	}, config)
}

func (f *InoxFunction) RemoveMutationCallbackMicrotasks(ctx *Context) {
	f.mutationFieldsLock.Lock()
	defer f.mutationFieldsLock.Unlock()

	if f.mutationCallbacks == nil {
		return
	}

	f.mutationCallbacks.RemoveMicrotasks()
}

func (f *InoxFunction) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	f.mutationFieldsLock.Lock()
	defer f.mutationFieldsLock.Unlock()

	f.mutationCallbacks.RemoveMicrotask(handle)
}

func (h *SynchronousMessageHandler) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	h.mutationFieldsLock.Lock()
	defer h.mutationFieldsLock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		if config.Depth > h.watchingDepth {
			h.watchingDepth = config.Depth

			path := Path("/handler")
			h.handler.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
				registerAgain = true

				h.mutationFieldsLock.Lock()
				callbacks := h.mutationCallbacks
				handlerWatchingDepth := h.watchingDepth
				h.mutationFieldsLock.Unlock()

				if !handlerWatchingDepth.IsSpecified() { //defensive check
					return
				}

				if mutation.Depth > handlerWatchingDepth { //defensive check
					return
				}

				callbacks.CallMicrotasks(ctx, mutation.Relocalized(path))

				return
			}, config)
		}
	}

	if h.mutationCallbacks == nil {
		h.mutationCallbacks = NewMutationCallbacks()
	}

	handle := h.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (h *SynchronousMessageHandler) RemoveMutationCallbackMicrotasks(ctx *Context) {
	h.mutationFieldsLock.Lock()
	defer h.mutationFieldsLock.Unlock()

	if h.mutationCallbacks == nil {
		return
	}

	h.mutationCallbacks.RemoveMicrotasks()
}

func (h *SynchronousMessageHandler) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	h.mutationFieldsLock.Lock()
	defer h.mutationFieldsLock.Unlock()

	h.mutationCallbacks.RemoveMicrotask(handle)
}

func makeMutationCallbackHandles(length int) []CallbackHandle {
	mutationCallbacks := make([]CallbackHandle, length)
	for i := range mutationCallbacks {
		mutationCallbacks[i] = FIRST_VALID_CALLBACK_HANDLE - 1
	}

	return mutationCallbacks
}
