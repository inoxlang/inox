package sql_ns

import (
	core "github.com/inoxlang/inox/internal/core"
)

func init() {
	targetSpecificInit()
}

func NewSQLNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{})
}
