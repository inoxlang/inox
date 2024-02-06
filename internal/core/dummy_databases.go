package core

import (
	"errors"
	"strings"
	"sync/atomic"
)

var (
	_ Database = (*FailedToOpenDatabase)(nil)
	_ Database = (*dummyDatabase)(nil)
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

type dummyDatabase struct {
	resource         SchemeHolder
	schemaUpdated    bool
	currentSchema    *ObjectPattern //if nil EMPTY_INEXACT_OBJECT_PATTERN is the schema.
	topLevelEntities map[string]Serializable
	loadError        error
	closed           atomic.Bool
}

func (db *dummyDatabase) Resource() SchemeHolder {
	return db.resource
}

func (db *dummyDatabase) Schema() *ObjectPattern {
	if db.closed.Load() {
		panic(ErrDatabaseClosed)
	}
	if db.currentSchema != nil {
		return db.currentSchema
	}
	return EMPTY_INEXACT_OBJECT_PATTERN
}

func (db *dummyDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern, handlers MigrationOpHandlers) {
	if db.schemaUpdated {
		panic(ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed)
	}
	if db.closed.Load() {
		panic(ErrDatabaseClosed)
	}
	db.schemaUpdated = true
	db.currentSchema = schema

	state := ctx.GetClosestState()

	if len(handlers.Deletions)+len(handlers.Initializations)+len(handlers.Replacements) > 0 {
		panic(errors.New("only inclusion handlers are supported"))
	}

	for pattern, handler := range handlers.Inclusions {
		if strings.Count(string(pattern), "/") != 1 {
			panic(errors.New("only shallow inclusion handlers are supported"))
		}
		result := handler.GetResult(ctx, state)
		db.topLevelEntities[string(pattern[1:])] = result.(Serializable)
	}
}

func (db *dummyDatabase) LoadTopLevelEntities(_ *Context) (map[string]Serializable, error) {
	if db.closed.Load() {
		return nil, ErrDatabaseClosed
	}
	if db.loadError != nil {
		return nil, db.loadError
	}
	return db.topLevelEntities, nil
}

func (db *dummyDatabase) Close(ctx *Context) error {
	db.closed.Store(true)
	return nil
}
