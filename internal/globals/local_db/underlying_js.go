//go:build js

package internal

type underlying struct {
}

func openUnderlying(config LocalDatabaseConfig) (_ underlying, finalErr error) {
	return underlying{}, ErrDatabaseNotSupported
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
