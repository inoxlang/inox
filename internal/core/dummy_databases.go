package core

import (
	"errors"
	"strings"
	"sync/atomic"
)

var (
	_ Database = (*FailedToOpenDatabase)(nil)
	_ Database = (*DummyDatabase)(nil)
)

type FailedToOpenDatabase struct {
	resource SchemeHolder
}

func NewFailedToOpenDatabase(resource SchemeHolder) *FailedToOpenDatabase {
	return &FailedToOpenDatabase{resource: resource}
}

func (db *FailedToOpenDatabase) Resource() SchemeHolder {
	return db.resource
}

func (db *FailedToOpenDatabase) Schema() *ObjectPattern {
	return EMPTY_INEXACT_OBJECT_PATTERN
}

func (db *FailedToOpenDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern, handlers MigrationOpHandlers) {
	panic(ErrNotImplemented)
}

func (db *FailedToOpenDatabase) LoadTopLevelEntities(_ *Context) (map[string]Serializable, error) {
	return nil, nil
}

func (db *FailedToOpenDatabase) Close(ctx *Context) error {
	return ErrNotImplemented
}

type DummyDatabase struct {
	Resource_        SchemeHolder
	SchemaUpdated    bool
	CurrentSchema    *ObjectPattern //if nil EMPTY_INEXACT_OBJECT_PATTERN is the schema.
	TopLevelEntities map[string]Serializable
	LoadError        error
	Closed           atomic.Bool
}

func (db *DummyDatabase) Resource() SchemeHolder {
	return db.Resource_
}

func (db *DummyDatabase) Schema() *ObjectPattern {
	if db.Closed.Load() {
		panic(ErrDatabaseClosed)
	}
	if db.CurrentSchema != nil {
		return db.CurrentSchema
	}
	return EMPTY_INEXACT_OBJECT_PATTERN
}

func (db *DummyDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern, handlers MigrationOpHandlers) {
	if db.SchemaUpdated {
		panic(ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed)
	}
	if db.Closed.Load() {
		panic(ErrDatabaseClosed)
	}
	db.SchemaUpdated = true
	db.CurrentSchema = schema

	state := ctx.MustGetClosestState()

	if len(handlers.Deletions)+len(handlers.Initializations)+len(handlers.Replacements) > 0 {
		panic(errors.New("only inclusion handlers are supported"))
	}

	for pattern, handler := range handlers.Inclusions {
		if strings.Count(string(pattern), "/") != 1 {
			panic(errors.New("only shallow inclusion handlers are supported"))
		}
		result := handler.GetResult(ctx, state)
		db.TopLevelEntities[string(pattern[1:])] = result.(Serializable)
	}
}

func (db *DummyDatabase) LoadTopLevelEntities(_ *Context) (map[string]Serializable, error) {
	if db.Closed.Load() {
		return nil, ErrDatabaseClosed
	}
	if db.LoadError != nil {
		return nil, db.LoadError
	}
	return db.TopLevelEntities, nil
}

func (db *DummyDatabase) Close(ctx *Context) error {
	db.Closed.Store(true)
	return nil
}
