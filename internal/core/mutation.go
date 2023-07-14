package core

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	MUTATION_KIND_NAMES = [...]string{
		UnspecifiedMutation:   "unspecified-mutation",
		AddProp:               "add-prop",
		UpdateProp:            "update-prop",
		InsertElemAtIndex:     "insert-elem-at-index",
		SetElemAtIndex:        "set-elem-at-index",
		SetSliceAtRange:       "set-slice-at-range",
		InsertSequenceAtIndex: "insert-seq-at-index",
		RemovePosition:        "remove-pos",
		RemovePositionRange:   "remove-pos-range",
		SpecificMutation:      "specific-mutation",
	}

	ErrCannotApplyIncompleteMutation = errors.New("cannot apply an incomplete mutation")
	ErrNotSupportedSpecificMutation  = errors.New("not supported specific mutation")
	ErrEmptyMutationPrefixSymbol     = errors.New("empty mutation prefix symbol")
	ErrInvalidMutationPrefixSymbol   = errors.New("invalid mutation prefix symbol")

	_ = []Value{Mutation{}}

	mutationCallbackPool = utils.Must(NewArrayPool[mutationCallback](100_000, 10))
)

const (
	DEFAULT_MICROTASK_ARRAY_SIZE = 4
)

// A Mutation stores the data (or part of the data) about the modification of a value, it is immutable and implements Value.
type Mutation struct {
	NotClonableMixin

	Kind                    MutationKind
	Complete                bool                    // true if the Mutation contains all the data necessary to be applied
	SpecificMutationVersion SpecificMutationVersion // set only if specific mutation
	SpecificMutationKind    SpecificMutationKind    // set only if specific mutation

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

func WriteSingleRepresentation(ctx *Context, v Serializable) ([]byte, [6]int32, error) {
	buf := bytes.NewBuffer(nil)
	config := &ReprConfig{AllVisible: true}
	if err := WriteRepresentation(buf, v, config, ctx); err != nil {
		return nil, [6]int32{}, err
	}
	return buf.Bytes(), [6]int32{int32(buf.Len())}, nil
}

func WriteConcatenatedRepresentations(ctx *Context, values ...Serializable) ([]byte, [6]int32, error) {
	buf := bytes.NewBuffer(nil)
	config := &ReprConfig{AllVisible: true}

	var sizes [6]int32

	if len(values) > len(sizes) {
		panic(fmt.Errorf("too many representations to write: %d", len(values)))
	}

	for i, val := range values {
		prevBufSize := buf.Len()

		if err := WriteRepresentation(buf, val, config, ctx); err != nil {
			return nil, [6]int32{}, err
		}

		elemSize := buf.Len() - prevBufSize
		sizes[i] = int32(elemSize)
	}

	return buf.Bytes(), sizes, nil
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
	data, sizes, err := WriteConcatenatedRepresentations(ctx, Str(name), value)

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
	data, sizes, err := WriteConcatenatedRepresentations(ctx, Str(name), newValue)

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
		Kind:               InsertElemAtIndex,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewRemovePositionMutation(ctx *Context, index int, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteSingleRepresentation(ctx, Int(index))

	return Mutation{
		Kind:               RemovePosition,
		Complete:           err == nil,
		Data:               data,
		DataElementLengths: sizes,
		Depth:              depth,
		Path:               path,
	}
}

func NewRemovePositionRangeMutation(ctx *Context, intRange IntRange, depth WatchingDepth, path Path) Mutation {
	data, sizes, err := WriteSingleRepresentation(ctx, intRange)

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
	Kind    SpecificMutationKind
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
	v, err := ParseRepr(ctx, b)
	//TODO: cache result (evict quickly)
	if err != nil {
		panic(err)
	}
	return v
}

func (m Mutation) AffectedProperty(ctx *Context) string {
	switch m.Kind {
	case AddProp, UpdateProp:
		return string(m.DataElem(ctx, 0).(Str))
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
		v.(MutableLengthSequence).setSlice(ctx, int(intRange.Start), int(intRange.End), m.Element(ctx))
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
	InsertElemAtIndex
	SetElemAtIndex
	SetSliceAtRange
	InsertSequenceAtIndex
	RemovePosition
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
	date     Date
}

func NewChange(mutation Mutation, date Date) Change {
	return Change{
		mutation: mutation,
		date:     date,
	}
}

func (c Change) Date() Date {
	return c.date
}

// MutationCallbacks is used by watchables that implement OnMutation.
type MutationCallbacks struct {
	ownedSlice  bool
	nextIndex   int
	nextHandle  CallbackHandle
	callbacks   []mutationCallback
	initialized bool
	lock        sync.Mutex
}

type mutationCallback struct {
	config MutationWatchingConfiguration
	fn     MutationCallbackMicrotask
	handle CallbackHandle
}

func NewMutationCallbackMicrotasks() *MutationCallbacks {
	return &MutationCallbacks{
		nextIndex:  0,
		nextHandle: FIRST_VALID_CALLBACK_HANDLE,
	}
}

func (t *MutationCallbacks) Functions() []mutationCallback {
	return t.callbacks
}

func (t *MutationCallbacks) init() {
	t.callbacks = utils.Must(mutationCallbackPool.GetArray())
	t.initialized = true
}

func (t *MutationCallbacks) AddMicrotask(m MutationCallbackMicrotask, config MutationWatchingConfiguration) (handle CallbackHandle) {
	if m == nil {
		return
	}

	t.lock.Lock() // possible deadlock due to microtasks
	defer t.lock.Unlock()

	if !t.initialized {
		t.init()
	}

	handle = t.nextHandle
	t.nextHandle++

	callback := mutationCallback{
		fn:     m,
		config: config,
		handle: handle,
	}

	if t.nextIndex >= len(t.callbacks) {
		if t.nextIndex == 0 {
			t.callbacks = utils.Must(mutationCallbackPool.GetArray())
			t.callbacks[0] = callback
		} else if t.ownedSlice {
			t.callbacks = append(t.callbacks, callback)
		} else {
			callbacks := make([]mutationCallback, len(t.callbacks)+1)
			copy(callbacks, t.callbacks)
			mutationCallbackPool.ReleaseArray(t.callbacks)
			t.callbacks = callbacks
			t.ownedSlice = true
		}
	} else {
		t.callbacks[t.nextIndex] = callback
	}
	t.updateNextIndex()

	return
}

func (t *MutationCallbacks) RemoveMicrotasks() {
	if t == nil {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	if !t.initialized {
		return
	}

	for i := range t.callbacks {
		t.callbacks[i] = mutationCallback{}
	}

	t.updateNextIndex()
}

func (t *MutationCallbacks) RemoveMicrotask(handle CallbackHandle) {
	if t == nil {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	if !t.initialized {
		return
	}

	for i := range t.callbacks {
		if t.callbacks[i].handle == handle {
			t.callbacks[i] = mutationCallback{}
			break
		}
	}

	t.updateNextIndex()
}

// CallMicrotasks calls the registered tasks that have a configured depth greater or equal to the depth at which the mutation
// happened (depth argument).
func (t *MutationCallbacks) CallMicrotasks(ctx *Context, m Mutation) {
	if t == nil {
		return
	}

	t.lock.Lock() // possible deadlock due to microtasks
	defer t.lock.Unlock()

	for i, callback := range t.callbacks {
		if callback.fn == nil || (m.Depth != UnspecifiedWatchingDepth && callback.config.Depth < m.Depth) {
			continue
		}

		func() {
			defer func() {
				if recover() != nil { //TODO: log errors
					t.callbacks[i].fn = nil
				}
			}()
			if !callback.fn(ctx, m) {
				t.callbacks[i].fn = nil
			}
		}()
	}

	t.updateNextIndex()
}

func (t *MutationCallbacks) updateNextIndex() {
	for i, callback := range t.callbacks {
		if callback.fn == nil {
			t.nextIndex = i
			return
		}
	}
	t.nextIndex = len(t.callbacks)
}

func (obj *Object) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	state := ctx.GetClosestState()
	obj.Lock(state)
	defer obj.Unlock(state)

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
					return -1, err
				}
			}
		}
	}

	if obj.mutationCallbacks == nil {
		obj.mutationCallbacks = NewMutationCallbackMicrotasks()
	}

	handle := obj.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
}

func (obj *Object) addPropMutationCallbackNoLock(ctx *Context, index int, val Value) error {
	if watchable, ok := val.(Watchable); ok {
		key := obj.keys[index]

		config := MutationWatchingConfiguration{
			Depth: obj.watchingDepth.MustMinusOne(), // depth at which we watch the property
		}
		path := Path("/" + key)

		handle, err := watchable.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			state := ctx.GetClosestState()
			obj.Lock(state)
			callbacks := obj.mutationCallbacks
			objWatchingDepth := obj.watchingDepth
			obj.Unlock(state)

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
	state := ctx.GetClosestState()
	obj.Lock(state)
	defer obj.Unlock(state)

	if obj.mutationCallbacks == nil {
		return
	}

	obj.mutationCallbacks.RemoveMicrotasks()
}

func (obj *Object) RemoveMutationCallback(ctx *Context, handle CallbackHandle) {
	state := ctx.GetClosestState()
	obj.Lock(state)
	defer obj.Unlock(state)

	obj.mutationCallbacks.RemoveMicrotask(handle)
}

func (l *List) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = ShallowWatching
	}

	if config.Depth >= IntermediateDepthWatching {
		return 0, ErrIntermediateDepthWatchingNotSupported
	}

	if l.mutationCallbacks == nil {
		l.mutationCallbacks = NewMutationCallbackMicrotasks()
	}

	handle := l.mutationCallbacks.AddMicrotask(microtask, config)

	return handle, nil
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
		s.mutationCallbacks = NewMutationCallbackMicrotasks()
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

func (dyn *DynamicValue) OnMutation(ctx *Context, microtask MutationCallbackMicrotask, config MutationWatchingConfiguration) (CallbackHandle, error) {
	dyn.lock.Lock()
	defer dyn.lock.Unlock()

	if config.Depth == UnspecifiedWatchingDepth {
		config.Depth = DeepWatching
	}

	if dyn.mutationCallbacks == nil {
		dyn.mutationCallbacks = NewMutationCallbackMicrotasks()
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
		g.mutationCallbacks = NewMutationCallbackMicrotasks()
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
		text := string(m.DataElem(ctx, 1).(Str))
		g.addEventNoLock(ptr, text)

	case SG_AddNode:
		g.nodes.lock.Lock()
		defer g.nodes.lock.Unlock()

		if g.isFrozen {
			return ErrAttemptToMutateFrozenValue
		}

		name := m.DataElem(ctx, 0).(Str)
		typename := m.DataElem(ctx, 1).(Str)
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
			case Str:
				edgeKind := m.DataElem(ctx, 5).(Int)
				g.addEdgeNoLock(string(textOrTuple), childNode, parentNode, SystemGraphEdgeKind(edgeKind))
			case *Tuple:
				for i := 0; i < len(textOrTuple.elements); i += 2 {
					edgeText := textOrTuple.elements[i].(Str)
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
		edgeText := m.DataElem(ctx, 2).(Str)
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
