//go:build wasm

package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	internal "github.com/inoxlang/inox/internal/core"
)

type underlying struct {
}

func openUnderlying(config LocalDatabaseConfig) (_ underlying, finalErr error) {
	return nil, ErrDatabaseNotSupported
}

func (u underlying) close() {

}

func (u underlying) isClosed() bool {
	return true
}

func (u underlying) get(ctx *Context, key Path, db any) (Value, Bool) {
	return nil, false
}

func (u underlying) has(ctx *Context, key Path, db any) Bool {
	return false
}

func (u underlying) set(ctx *Context, key Path, value Value, db any) {

}
