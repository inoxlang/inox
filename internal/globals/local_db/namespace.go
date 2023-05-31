package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
)

func init() {
	core.RegisterSymbolicGoFunction(openDatabase, func(ctx *symbolic.Context, r symbolic.ResourceName) (*SymbolicLocalDatabase, *symbolic.Error) {
		return &SymbolicLocalDatabase{}, nil
	})
}

func NewLocalDbNamespace() *Record {
	return core.NewRecordFromMap(core.ValMap{
		//
		"open": core.ValOf(openDatabase),
	})
}
