package local_db_ns

import "github.com/inoxlang/inox/internal/core"

func (kvs *LocalDatabase) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return nil, core.ErrNotClonable
}
