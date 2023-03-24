package internal

import core "github.com/inox-project/inox/internal/core"

func (kvs *LocalDatabase) Equal(ctx *core.Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherKVS, ok := other.(*LocalDatabase)
	return ok && kvs == otherKVS
}
