package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrFailedToSnapshot           = errors.New("failed to snapshot value")
	ErrAttemptToMutateFrozenValue = errors.New("attempt to mutate a frozen value")

	_ = []InMemorySnapshotable{(*RuneSlice)(nil), (*SystemGraph)(nil), (*DynamicValue)(nil)}
)

// Snapshot holds either the serialized representation of a Value or a in-memory FROZEN value.
type Snapshot struct {
	date     DateTime
	repr     ValueRepresentation
	inMemory Serializable //value should be either an InMemorySnapshotable or an immutable
}

func (s *Snapshot) Date() DateTime {
	return s.date
}

// implementations of InMemorySnapshotable are Watchables that can take an in-memory snapshot of themselves in a few milliseconds or less.
// the values in snapshots should be FROZEN and should NOT be connected to other live objects, they should be be able to be mutated again
// after being unfreezed.
type InMemorySnapshotable interface {
	Watchable
	Serializable
	TakeInMemorySnapshot(ctx *Context) (*Snapshot, error)
	IsFrozen() bool
	Unfreeze(ctx *Context) error
}

func TakeSnapshot(ctx *Context, v Serializable, mustBeSerialized bool) (*Snapshot, error) {
	now := DateTime(time.Now())
	if !v.IsMutable() {
		return &Snapshot{date: now, inMemory: v}, nil
	}

	var snapshotableErr error
	if snapshotable, ok := v.(InMemorySnapshotable); ok && !mustBeSerialized {
		snapshot, err := snapshotable.TakeInMemorySnapshot(ctx)
		if err == nil {
			val, ok := snapshot.inMemory.(InMemorySnapshotable)
			if !ok {
				return nil, fmt.Errorf("InMemorySnapshotable returned a snapshot containing a value that does not implement InMemorySnapshotable: type is: %T", snapshot.inMemory)
			}
			if !val.IsFrozen() {
				return nil, fmt.Errorf("InMemorySnapshotable returned a snapshot containing a value that is not frozen: type is: %T", val)
			}
			return snapshot, nil
		}
		snapshotableErr = err
	}

	repr, err := GetRepresentationWithConfig(v, &ReprConfig{AllVisible: true}, ctx)
	if err != nil {
		if snapshotableErr != nil {
			err = fmt.Errorf("%w AND value was an InMemorySnapshotable that returned this error when snapshoted: %w", err, snapshotableErr)
		}
		return nil, fmt.Errorf("failed to take snapshot of value of type %T: %w", v, err)
	}
	return &Snapshot{
		repr: repr,
		date: now,
	}, nil
}

func (s *Snapshot) InstantiateValue(ctx *Context) (Serializable, error) {
	if s.inMemory != nil {
		if s.inMemory.IsMutable() {
			snapshotable := s.inMemory.(InMemorySnapshotable)

			// TODO: refactor, *Snapshot is not necessary  here
			snap, _ := snapshotable.TakeInMemorySnapshot(ctx)
			snap.inMemory.(InMemorySnapshotable).Unfreeze(ctx)
			return snap.inMemory, nil
		}
		return s.inMemory, nil
	}
	v, err := ParseRepr(ctx, s.repr)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate snapshot's value: %w", err)
	}
	return v, nil
}

func (s *Snapshot) WithChangeApplied(ctx *Context, c Change) (*Snapshot, error) {
	v, err := s.InstantiateValue(ctx)
	if err != nil {
		return nil, err
	}

	snapshotDate := time.Time(s.date)
	changeDate := time.Time(c.datetime)

	if snapshotDate.After(changeDate) {
		return nil, fmt.Errorf("cannot apply a change (date: %s) on a snapshot that is more recent than the change: %s", changeDate, snapshotDate)
	}

	if err := c.mutation.ApplyTo(ctx, v); err != nil {
		return nil, err
	}

	if snapshotable, ok := v.(InMemorySnapshotable); ok {
		return &Snapshot{
			date:     c.datetime,
			inMemory: snapshotable,
		}, nil
	}

	repr, err := GetRepresentationWithConfig(v, &ReprConfig{AllVisible: true}, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take snapshot of value: %w", err)
	}

	return &Snapshot{
		date: c.datetime,
		repr: repr,
	}, nil
}

//

func (r *RuneSlice) TakeInMemorySnapshot(ctx *Context) (*Snapshot, error) {
	sliceClone, _ := r.clone()
	sliceClone.frozen = true

	return &Snapshot{
		date:     DateTime(time.Now()),
		inMemory: sliceClone,
	}, nil
}

func (r *RuneSlice) IsFrozen() bool {
	return r.frozen
}

func (r *RuneSlice) Unfreeze(ctx *Context) error {
	r.frozen = false
	return nil
}

func (d *DynamicValue) TakeInMemorySnapshot(ctx *Context) (*Snapshot, error) {
	if v, ok := d.Resolve(ctx).(InMemorySnapshotable); ok {
		return v.TakeInMemorySnapshot(ctx)
	}
	return nil, fmt.Errorf("%w: value to which dynamic value resolve is not an in memory snapshotable", ErrFailedToSnapshot)
}

func (d *DynamicValue) IsFrozen() bool {
	return false
}

func (d *DynamicValue) Unfreeze(ctx *Context) error {
	return nil
}

func (g *SystemGraph) takeSnapshot(ctx *Context) *SystemGraph {
	g.eventLogLock.Lock()
	defer g.eventLogLock.Unlock()

	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()

	newNodes := &SystemGraphNodes{
		list: utils.CopySlice(g.nodes.list),
	}

	origToCopy := make(map[*SystemGraphNode]*SystemGraphNode, len(newNodes.list))
	for i, origNode := range g.nodes.list {
		nodeCopy := *origNode
		if nodeCopy.edgesFrom != nil {
			nodeCopy.edgesFrom = utils.CopySlice(nodeCopy.edgesFrom)
		}
		newNodes.list[i] = &nodeCopy
		origToCopy[origNode] = &nodeCopy
	}

	newNodes.availableNodes = make([]*SystemGraphNode, len(g.nodes.availableNodes))
	for i, availableNode := range newNodes.availableNodes {
		newNodes.availableNodes[i] = origToCopy[availableNode]
	}

	newNodes.ptrToNode = make(map[uintptr]*SystemGraphNode, len(g.nodes.ptrToNode))
	for k, v := range g.nodes.ptrToNode {
		newNodes.ptrToNode[k] = origToCopy[v]
	}

	newGraph := &SystemGraph{
		isFrozen: true,
		nodes:    newNodes,
		eventLog: utils.CopySlice(g.eventLog),
	}
	newGraph.nodes.graph = newGraph

	g.lastSnapshot = newGraph

	return newGraph
}

func (g *SystemGraph) TakeInMemorySnapshot(ctx *Context) (*Snapshot, error) {
	return &Snapshot{
		inMemory: g.takeSnapshot(ctx),
		date:     DateTime(time.Now()),
	}, nil
}

func (g *SystemGraph) IsFrozen() bool {
	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()
	return g.isFrozen
}

func (g *SystemGraph) Unfreeze(ctx *Context) error {
	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()
	g.isFrozen = false
	return nil
}
