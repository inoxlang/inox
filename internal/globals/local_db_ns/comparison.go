package local_db_ns

import "github.com/inoxlang/inox/internal/core"

func (kvs *LocalDatabase) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherKVS, ok := other.(*LocalDatabase)
	return ok && kvs == otherKVS
}
