package core

import (
	"errors"
	"sync"
)

var (
	openDbFnRegistry     = map[Scheme]OpenDBFn{}
	openDbFnRegistryLock sync.Mutex

	ErrNonUniqueDbOpenFnRegistration = errors.New("non unique open DB function registration")
)

var (
	DATABASE_PROPNAMES = []string{"update_schema", "close"}

	_ Value = (*DatabaseIL)(nil)
)

type DatabaseIL struct {
	inner Database

	NoReprMixin
	NotClonableMixin
}

type DbOpenConfiguration struct {
	Resource       SchemeHolder
	ResolutionData Value
	FullAccess     bool
}

type OpenDBFn func(ctx *Context, config DbOpenConfiguration) (Database, error)

type Database interface {
	Resource() SchemeHolder
	Schema() *ObjectPattern
	UpdateSchema(ctx *Context, schema *ObjectPattern) error
	TopLevelEntities() map[string]Value
	Close(ctx *Context) error
}

func WrapDatabase(inner Database) *DatabaseIL {
	return &DatabaseIL{
		inner: inner,
	}
}

func RegisterOpenDbFn(scheme Scheme, fn OpenDBFn) {
	openDbFnRegistryLock.Lock()
	defer openDbFnRegistryLock.Unlock()

	_, ok := openDbFnRegistry[scheme]
	if ok {
		panic(ErrNonUniqueDbOpenFnRegistration)
	}

	openDbFnRegistry[scheme] = fn
}

func GetOpenDbFn(scheme Scheme) (OpenDBFn, bool) {
	openDbFnRegistryLock.Lock()
	defer openDbFnRegistryLock.Unlock()

	fn, ok := openDbFnRegistry[scheme]

	return fn, ok
}

func (db *DatabaseIL) Resource() SchemeHolder {
	return db.inner.Resource()
}

func (db *DatabaseIL) UpdateSchema(ctx *Context, schema *ObjectPattern) error {
	return db.inner.UpdateSchema(ctx, schema)
}

func (db *DatabaseIL) Close(ctx *Context) error {
	return db.inner.Close(ctx)
}

func (db *DatabaseIL) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "update_schema":
		return WrapGoMethod(db.UpdateSchema), true
	case "close":
		return WrapGoMethod(db.Close), true
	}
	return nil, false
}

func (db *DatabaseIL) Prop(ctx *Context, name string) Value {
	switch name {
	}
	method, ok := db.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, db))
	}
	return method
}

func (*DatabaseIL) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*DatabaseIL) PropertyNames(ctx *Context) []string {
	return DATABASE_PROPNAMES
}
