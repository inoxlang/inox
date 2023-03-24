package internal

import core "github.com/inox-project/inox/internal/core"

func (kvs *LocalDatabase) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return nil, core.ErrNotClonable
}
