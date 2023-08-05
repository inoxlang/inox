package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseIL(t *testing.T) {
	t.Run("", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": Int(1)},
		}

		dbIL := WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:      db,
			OwnerState: ctx.state,
		})

		assert.Equal(t, map[string]Serializable{"a": Int(1)}, dbIL.topLevelEntities)
	})

	t.Run("if a schema update is expected top level entiries should not be loaded", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": Int(1)},
		}

		dbIL := WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
		})

		assert.Nil(t, dbIL.topLevelEntities)
	})

	t.Run("only the owner state should be able to update the schema", func(t *testing.T) {
		ctx1 := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := WrapDatabase(ctx1, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx1.state,
			ExpectedSchemaUpdate: true,
		})

		ctx2 := NewContexWithEmptyState(ContextConfig{}, nil)

		assert.PanicsWithValue(t, ErrDatabaseSchemaOnlyUpdatableByOwnerState, func() {
			dbIL.UpdateSchema(ctx2, NewInexactObjectPattern(map[string]Pattern{}))
		})
	})

	t.Run("updating the schema while it not expected should cause a panic", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: false,
		})

		assert.PanicsWithValue(t, ErrNoneDatabaseSchemaUpdateExpected, func() {
			dbIL.UpdateSchema(ctx, NewInexactObjectPattern(map[string]Pattern{}))
		})
	})

	t.Run("updating the schema twice should cause a panic", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
		})

		dbIL.UpdateSchema(ctx, NewInexactObjectPattern(map[string]Pattern{}))

		assert.PanicsWithValue(t, ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed, func() {
			dbIL.UpdateSchema(ctx, NewInexactObjectPattern(map[string]Pattern{
				"a": INT_PATTERN,
			}))
		})
	})

	t.Run("accessing the database while its schema is not yet updated should cause a panic", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": Int(1)},
		}

		dbIL := WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
		})

		assert.PanicsWithValue(t, ErrInvalidAccessSchemaNotUpdatedYet, func() {
			dbIL.Prop(ctx, "a")
		})
	})
}
