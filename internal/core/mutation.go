package internal

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/inox-project/inox/internal/utils"
)

var (
	MUTATION_KIND_NAMES = []string{"unspecified-mutation", "add-prop", "update-prop", "insert-elem-at-index", "set-elem-at-index"}

	ErrCannotApplyIncompleteMutation = errors.New("cannot apply an incomplete mutation")
	ErrNotSupportedSpecificMutation  = errors.New("not supported specific mutation")
	_                                = []Value{Mutation{}}

	mutationCallbackPool = utils.Must(NewArrayPool[mutationCallback](100_000, 10))
)

const (
	DEFAULT_MICROTASK_ARRAY_SIZE = 4
)

// A Mutation stores the data (or part of the data) about the modification of a value, it is immutable and implements Value.
type Mutation struct {
	NotClonableMixin
	NoReprMixin

	Kind                    MutationKind
	Complete                bool // true if the Mutation contains all the data necessary to be applied
	SpecificMutationVersion int8
	SpecificMutationKind    SpecificMutationKind

	Data0 []byte //immutable
	Data1 []byte //immutable
	Path  Path   // can be empty
	Depth WatchingDepth
}

type SpecificMutationKind int8

type SpecificMutationAcceptor interface {
	Value
	// ApplySpecificMutation should apply the mutation to the Value, ErrNotSupportedSpecificMutation should be returned
	// if it's not possible.
	ApplySpecificMutation(ctx *Context, m Mutation) error
}

func NewUnspecifiedMutation(depth WatchingDepth, path Path) Mutation {
	return Mutation{
		Kind:     UnspecifiedMutation,
		Complete: false,
		Depth:    depth,
		Path:     path,
	}
}

func NewAddPropMutation(ctx *Context, name string, value Value, depth WatchingDepth, path Path) Mutation {
	valueRepr, err := GetRepresentationWithConfig(value, &ReprConfig{allVisible: true}, ctx)

	return Mutation{
		Kind:     AddProp,
		Complete: err == nil,
		Data0:    utils.StringAsBytes(name),
		Data1:    valueRepr,
		Depth:    depth,
		Path:     path,
	}
}

func NewUpdatePropMutation(ctx *Context, name string, newValue Value, depth WatchingDepth, path Path) Mutation {
	valueRepr, err := GetRepresentationWithConfig(newValue, &ReprConfig{allVisible: true}, ctx)

	return Mutation{
		Kind:     UpdateProp,
		Complete: err == nil,
		Data0:    utils.StringAsBytes(name),
		Depth:    depth,
		Data1:    valueRepr,
		//TODO: Data1
		Path: path,
	}
}

func NewSetElemAtIndexMutation(ctx *Context, index int, elem Value, depth WatchingDepth, path Path) Mutation {
	elemRepr, err := GetRepresentationWithConfig(elem, &ReprConfig{allVisible: true}, ctx)

	return Mutation{
		Kind:     SetElemAtIndex,
		Complete: err == nil,
		Data0:    []byte(strconv.FormatInt(int64(index), 10)),
		Data1:    elemRepr,
		Depth:    depth,
		Path:     path,
	}
}

func NewInsertElemAtIndexMutation(ctx *Context, index int, elem Value, depth WatchingDepth, path Path) Mutation {
	elemRepr, err := GetRepresentationWithConfig(elem, &ReprConfig{allVisible: true}, ctx)

	return Mutation{
		Kind:     InsertElemAtIndex,
		Complete: err == nil,
		Data0:    []byte(strconv.FormatInt(int64(index), 10)),
		Data1:    elemRepr,
		Depth:    depth,
		Path:     path,
	}
}

func NewInsertSequenceAtIndexMutation(ctx *Context, index int, seq Sequence, depth WatchingDepth, path Path) Mutation {
	seqRepr, err := GetRepresentationWithConfig(seq, &ReprConfig{allVisible: true}, ctx)

	return Mutation{
		Kind:     InsertElemAtIndex,
		Complete: err == nil,
		Data0:    []byte(strconv.FormatInt(int64(index), 10)),
		Data1:    seqRepr,
		Depth:    depth,
		Path:     path,
	}
}

func NewRemovePositionMutation(index int, depth WatchingDepth, path Path) Mutation {
	return Mutation{
		Kind:     RemovePosition,
		Complete: true,
		Data0:    []byte(strconv.FormatInt(int64(index), 10)),
		Depth:    depth,
		Path:     path,
	}
}

func NewRemovePositionRangeMutation(ctx *Context, intRange IntRange, depth WatchingDepth, path Path) Mutation {
	return Mutation{
		Kind:     RemovePositionRange,
		Complete: true,
		Data0:    MustGetRepresentationWithConfig(intRange, &ReprConfig{allVisible: true}, ctx),
		Depth:    depth,
		Path:     path,
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

func (m Mutation) AffectedProperty() string {
	switch m.Kind {
	case AddProp, UpdateProp:
		return string(Str(utils.BytesAsString(m.Data0)))
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) AffectedIndex() int {
	switch m.Kind {
	case InsertElemAtIndex, SetElemAtIndex, InsertSequenceAtIndex, RemovePosition:
		//TODO: rework
		index, err := strconv.Atoi(utils.BytesAsString(m.Data0))
		if err != nil {
			panic(err)
		}
		return index
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) AffectedRange(ctx *Context) IntRange {
	switch m.Kind {
	case InsertElemAtIndex, SetElemAtIndex, InsertSequenceAtIndex, RemovePosition:
		return utils.Must(ParseRepr(ctx, m.Data0)).(IntRange)
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) PropValue(ctx *Context) Value {
	switch m.Kind {
	case AddProp, UpdateProp:
		return utils.Must(ParseRepr(ctx, m.Data1))
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) Element(ctx *Context) Value {
	switch m.Kind {
	case InsertElemAtIndex, SetElemAtIndex:
		return utils.Must(ParseRepr(ctx, m.Data1))
	default:
		panic(ErrUnreachable)
	}
}

func (m Mutation) Sequence(ctx *Context) Sequence {
	switch m.Kind {
	case InsertSequenceAtIndex:
		return utils.Must(ParseRepr(ctx, m.Data1)).(Sequence)
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
		v.(IProps).SetProp(ctx, m.AffectedProperty(), m.PropValue(ctx))
	case UpdateProp:
		v.(IProps).SetProp(ctx, m.AffectedProperty(), m.PropValue(ctx))
	case SetElemAtIndex:
		v.(MutableLengthSequence).set(ctx, m.AffectedIndex(), m.Element(ctx))
	case InsertElemAtIndex:
		v.(MutableLengthSequence).insertElement(ctx, m.Element(ctx), Int(m.AffectedIndex()))
	case InsertSequenceAtIndex:
		v.(MutableLengthSequence).insertSequence(ctx, m.Sequence(ctx), Int(m.AffectedIndex()))
	case RemovePosition:
		v.(MutableLengthSequence).removePosition(ctx, Int(m.AffectedIndex()))
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
				obj.Unlock(state)
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
