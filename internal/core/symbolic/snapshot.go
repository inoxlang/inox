package internal

import "errors"

var (
	ANY_SNAPSHOT            = &Snapshot{}
	ANY_IN_MEM_SNAPSHOTABLE = &AnyInMemorySnapshotable{}
	SNAPSHOT_PROPNAMES      = []string{}
)

var (
	ErrFailedToSnapshot = errors.New("failed to snapshot")
	_                   = []InMemorySnapshotable{(*RuneSlice)(nil), (*DynamicValue)(nil), (*AnyInMemorySnapshotable)(nil)}
)

type InMemorySnapshotable interface {
	Watchable
	TakeInMemorySnapshot() (*Snapshot, error)
}

// An Snapshot represents a symbolic Snapshot.
type Snapshot struct {
	UnassignablePropsMixin
	_ int
}

func (m *Snapshot) Test(v SymbolicValue) bool {
	_, ok := v.(*Snapshot)

	return ok
}

func (m *Snapshot) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (m *Snapshot) IsWidenable() bool {
	return false
}

func (m *Snapshot) String() string {
	return "%snapshot"
}

func (m *Snapshot) WidestOfType() SymbolicValue {
	return ANY_SNAPSHOT
}

func (m *Snapshot) ReceiveSnapshot(SymbolicValue) error {
	return nil
}

func (m *Snapshot) Prop(name string) SymbolicValue {
	switch name {
	}
	panic(FormatErrPropertyDoesNotExist(name, m))
}

func (m *Snapshot) PropertyNames() []string {
	return SNAPSHOT_PROPNAMES
}

// An AnyInMemorySnapshotable represents a symbolic InMemorySnapshotable we do not know the concrete type.
type AnyInMemorySnapshotable struct {
	_ int
}

func (s *AnyInMemorySnapshotable) Test(v SymbolicValue) bool {
	_, ok := v.(InMemorySnapshotable)

	return ok
}

func (s *AnyInMemorySnapshotable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *AnyInMemorySnapshotable) IsWidenable() bool {
	return false
}

func (s *AnyInMemorySnapshotable) String() string {
	return "%in-memory-snapshotable"
}

func (s *AnyInMemorySnapshotable) WidestOfType() SymbolicValue {
	return ANY_IN_MEM_SNAPSHOTABLE
}

func (s *AnyInMemorySnapshotable) WatcherElement() SymbolicValue {
	return ANY
}

func (s *AnyInMemorySnapshotable) TakeInMemorySnapshot() (*Snapshot, error) {
	return ANY_SNAPSHOT, nil
}
