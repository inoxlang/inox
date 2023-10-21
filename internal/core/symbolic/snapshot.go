package symbolic

import (
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

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

func (m *Snapshot) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Snapshot)

	return ok
}

func (m *Snapshot) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("snapshot")
	return
}

func (m *Snapshot) WidestOfType() Value {
	return ANY_SNAPSHOT
}

func (m *Snapshot) ReceiveSnapshot(Value) error {
	return nil
}

func (m *Snapshot) Prop(name string) Value {
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

func (s *AnyInMemorySnapshotable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(InMemorySnapshotable)

	return ok
}

func (s *AnyInMemorySnapshotable) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("in-memory-snapshotable")
	return
}

func (s *AnyInMemorySnapshotable) WidestOfType() Value {
	return ANY_IN_MEM_SNAPSHOTABLE
}

func (s *AnyInMemorySnapshotable) WatcherElement() Value {
	return ANY
}

func (s *AnyInMemorySnapshotable) TakeInMemorySnapshot() (*Snapshot, error) {
	return ANY_SNAPSHOT, nil
}
