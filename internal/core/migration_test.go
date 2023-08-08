package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectPatternGetMigrationOperations(t *testing.T) {

	t.Run("new property", func(t *testing.T) {
		empty := NewInexactObjectPattern(map[string]Pattern{})
		singleProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})

		migrations, err := empty.GetMigrationOperations(singleProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			InclusionMigrationOp{
				Value:          INT_PATTERN,
				Optional:       false,
				migrationMixin: migrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("new optional property", func(t *testing.T) {
		empty := NewInexactObjectPattern(map[string]Pattern{})
		singleOptionalProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})

		migrations, err := empty.GetMigrationOperations(singleOptionalProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			InclusionMigrationOp{
				Value:          INT_PATTERN,
				Optional:       true,
				migrationMixin: migrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("property removal", func(t *testing.T) {
		singleProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		empty := NewInexactObjectPattern(map[string]Pattern{})

		migrations, err := singleProp.GetMigrationOperations(empty, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			RemovalMigrationOp{
				Value:          INT_PATTERN,
				migrationMixin: migrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("optional property removal", func(t *testing.T) {
		singleOptionalProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})
		empty := NewInexactObjectPattern(map[string]Pattern{})

		migrations, err := singleOptionalProp.GetMigrationOperations(empty, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			RemovalMigrationOp{
				Value:          INT_PATTERN,
				migrationMixin: migrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("no longer optional prop", func(t *testing.T) {
		singleOptionalProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})
		singleRequiredProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})

		migrations, err := singleOptionalProp.GetMigrationOperations(singleRequiredProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			NillableInitializationMigrationOp{
				Value:          INT_PATTERN,
				migrationMixin: migrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("no longer required prop", func(t *testing.T) {
		singleRequiredProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		singleOptionalProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})

		migrations, err := singleRequiredProp.GetMigrationOperations(singleOptionalProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})
}
