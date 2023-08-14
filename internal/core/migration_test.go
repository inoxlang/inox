package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectPatternGetMigrationOperations(t *testing.T) {

	ctx := NewContexWithEmptyState(ContextConfig{}, nil)

	t.Run("same empty object", func(t *testing.T) {
		empty1 := NewInexactObjectPattern(map[string]Pattern{})
		empty2 := NewInexactObjectPattern(map[string]Pattern{})

		migrations, err := empty1.GetMigrationOperations(ctx, empty2, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("new property", func(t *testing.T) {
		empty := NewInexactObjectPattern(map[string]Pattern{})
		singleProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})

		migrations, err := empty.GetMigrationOperations(ctx, singleProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			InclusionMigrationOp{
				Value:          INT_PATTERN,
				Optional:       false,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("same single-prop object", func(t *testing.T) {
		singleProp1 := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		singleProp2 := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})

		migrations, err := singleProp1.GetMigrationOperations(ctx, singleProp2, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("new optional property", func(t *testing.T) {
		empty := NewInexactObjectPattern(map[string]Pattern{})
		singleOptionalProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})

		migrations, err := empty.GetMigrationOperations(ctx, singleOptionalProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			InclusionMigrationOp{
				Value:          INT_PATTERN,
				Optional:       true,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("property removal", func(t *testing.T) {
		singleProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		empty := NewInexactObjectPattern(map[string]Pattern{})

		migrations, err := singleProp.GetMigrationOperations(ctx, empty, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			RemovalMigrationOp{
				Value:          INT_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("optional property removal", func(t *testing.T) {
		singleOptionalProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})
		empty := NewInexactObjectPattern(map[string]Pattern{})

		migrations, err := singleOptionalProp.GetMigrationOperations(ctx, empty, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			RemovalMigrationOp{
				Value:          INT_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("no longer optional prop", func(t *testing.T) {
		singleOptionalProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})
		singleRequiredProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})

		migrations, err := singleOptionalProp.GetMigrationOperations(ctx, singleRequiredProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			NillableInitializationMigrationOp{
				Value:          INT_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("no longer required prop", func(t *testing.T) {
		singleRequiredProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		singleOptionalProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})

		migrations, err := singleRequiredProp.GetMigrationOperations(ctx, singleOptionalProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("property type update", func(t *testing.T) {
		singleProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		singleOtherTypeProp := NewInexactObjectPattern(map[string]Pattern{"a": STR_PATTERN})

		migrations, err := singleProp.GetMigrationOperations(ctx, singleOtherTypeProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        INT_PATTERN,
				Next:           STR_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("optional property type update that is now required", func(t *testing.T) {
		singleProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})
		singleOtherTypeProp := NewInexactObjectPattern(map[string]Pattern{"a": STR_PATTERN})

		migrations, err := singleProp.GetMigrationOperations(ctx, singleOtherTypeProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        INT_PATTERN,
				Next:           STR_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("optional property type update + no longer required", func(t *testing.T) {
		singleProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		singleOtherTypeProp := NewInexactObjectPatternWithOptionalProps(map[string]Pattern{"a": STR_PATTERN}, map[string]struct{}{"a": {}})

		migrations, err := singleProp.GetMigrationOperations(ctx, singleOtherTypeProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        INT_PATTERN,
				Next:           STR_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("new property in inner object", func(t *testing.T) {
		empty := NewInexactObjectPattern(map[string]Pattern{"a": NewInexactObjectPattern(map[string]Pattern{})})
		singleProp := NewInexactObjectPattern(map[string]Pattern{
			"a": NewInexactObjectPattern(map[string]Pattern{"b": INT_PATTERN}),
		})

		migrations, err := empty.GetMigrationOperations(ctx, singleProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			InclusionMigrationOp{
				Value:          INT_PATTERN,
				Optional:       false,
				MigrationMixin: MigrationMixin{"/a/b"},
			},
		}, migrations)
	})

	t.Run("new property & all previous properties removed", func(t *testing.T) {
		singleProp := NewInexactObjectPattern(map[string]Pattern{"a": INT_PATTERN})
		nextSingleProp := NewInexactObjectPattern(map[string]Pattern{"b": INT_PATTERN})

		migrations, err := singleProp.GetMigrationOperations(ctx, nextSingleProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        singleProp,
				Next:           nextSingleProp,
				MigrationMixin: MigrationMixin{"/"},
			},
		}, migrations)
	})
}

func TestRecordPatternGetMigrationOperations(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)

	t.Run("same empty record", func(t *testing.T) {
		empty1 := NewInexactRecordPattern(map[string]Pattern{})
		empty2 := NewInexactRecordPattern(map[string]Pattern{})

		migrations, err := empty1.GetMigrationOperations(ctx, empty2, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("new property", func(t *testing.T) {
		empty := NewInexactRecordPattern(map[string]Pattern{})
		singleProp := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})

		migrations, err := empty.GetMigrationOperations(ctx, singleProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			InclusionMigrationOp{
				Value:          INT_PATTERN,
				Optional:       false,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("new optional property", func(t *testing.T) {
		empty := NewInexactRecordPattern(map[string]Pattern{})
		singleOptionalProp := NewInexactRecordPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})

		migrations, err := empty.GetMigrationOperations(ctx, singleOptionalProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			InclusionMigrationOp{
				Value:          INT_PATTERN,
				Optional:       true,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("same single-prop record", func(t *testing.T) {
		singleProp1 := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})
		singleProp2 := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})

		migrations, err := singleProp1.GetMigrationOperations(ctx, singleProp2, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("property removal", func(t *testing.T) {
		singleProp := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})
		empty := NewInexactRecordPattern(map[string]Pattern{})

		migrations, err := singleProp.GetMigrationOperations(ctx, empty, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			RemovalMigrationOp{
				Value:          INT_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("optional property removal", func(t *testing.T) {
		singleOptionalProp := NewInexactRecordPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})
		empty := NewInexactRecordPattern(map[string]Pattern{})

		migrations, err := singleOptionalProp.GetMigrationOperations(ctx, empty, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			RemovalMigrationOp{
				Value:          INT_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("no longer optional prop", func(t *testing.T) {
		singleOptionalProp := NewInexactRecordPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})
		singleRequiredProp := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})

		migrations, err := singleOptionalProp.GetMigrationOperations(ctx, singleRequiredProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			NillableInitializationMigrationOp{
				Value:          INT_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("no longer required prop", func(t *testing.T) {
		singleRequiredProp := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})
		singleOptionalProp := NewInexactRecordPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})

		migrations, err := singleRequiredProp.GetMigrationOperations(ctx, singleOptionalProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("property type update", func(t *testing.T) {
		singleProp := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})
		singleOtherTypeProp := NewInexactRecordPattern(map[string]Pattern{"a": STR_PATTERN})

		migrations, err := singleProp.GetMigrationOperations(ctx, singleOtherTypeProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        INT_PATTERN,
				Next:           STR_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("optional property type update that is now required", func(t *testing.T) {
		singleProp := NewInexactRecordPatternWithOptionalProps(map[string]Pattern{"a": INT_PATTERN}, map[string]struct{}{"a": {}})
		singleOtherTypeProp := NewInexactRecordPattern(map[string]Pattern{"a": STR_PATTERN})

		migrations, err := singleProp.GetMigrationOperations(ctx, singleOtherTypeProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        INT_PATTERN,
				Next:           STR_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("optional property type update + no longer required", func(t *testing.T) {
		singleProp := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})
		singleOtherTypeProp := NewInexactRecordPatternWithOptionalProps(map[string]Pattern{"a": STR_PATTERN}, map[string]struct{}{"a": {}})

		migrations, err := singleProp.GetMigrationOperations(ctx, singleOtherTypeProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        INT_PATTERN,
				Next:           STR_PATTERN,
				MigrationMixin: MigrationMixin{"/a"},
			},
		}, migrations)
	})

	t.Run("new property in inner object", func(t *testing.T) {
		empty := NewInexactRecordPattern(map[string]Pattern{"a": NewInexactRecordPattern(map[string]Pattern{})})
		singleProp := NewInexactRecordPattern(map[string]Pattern{
			"a": NewInexactRecordPattern(map[string]Pattern{"b": INT_PATTERN}),
		})

		migrations, err := empty.GetMigrationOperations(ctx, singleProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			InclusionMigrationOp{
				Value:          INT_PATTERN,
				Optional:       false,
				MigrationMixin: MigrationMixin{"/a/b"},
			},
		}, migrations)
	})

	t.Run("new property & all previous properties removed", func(t *testing.T) {
		singleProp := NewInexactRecordPattern(map[string]Pattern{"a": INT_PATTERN})
		nextSingleProp := NewInexactRecordPattern(map[string]Pattern{"b": INT_PATTERN})

		migrations, err := singleProp.GetMigrationOperations(ctx, nextSingleProp, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        singleProp,
				Next:           nextSingleProp,
				MigrationMixin: MigrationMixin{"/"},
			},
		}, migrations)
	})
}

func TestListPatternGetMigrationOperations(t *testing.T) {

	ctx := NewContexWithEmptyState(ContextConfig{}, nil)

	t.Run("same general element pattern", func(t *testing.T) {
		intList := NewListPatternOf(INT_PATTERN)
		intList2 := NewListPatternOf(INT_PATTERN)

		migrations, err := intList.GetMigrationOperations(ctx, intList2, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("general element pattern replaced by different type", func(t *testing.T) {
		intList := NewListPatternOf(INT_PATTERN)
		stringList := NewListPatternOf(STR_PATTERN)

		migrations, err := intList.GetMigrationOperations(ctx, stringList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        INT_PATTERN,
				Next:           STR_PATTERN,
				MigrationMixin: MigrationMixin{"/*"},
			},
		}, migrations)
	})

	t.Run("general element pattern replaced by super type", func(t *testing.T) {
		intList := NewListPatternOf(INT_PATTERN)
		serializableList := NewListPatternOf(SERIALIZABLE_PATTERN)

		migrations, err := intList.GetMigrationOperations(ctx, serializableList, "/")

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("general element pattern replaced by sub type", func(t *testing.T) {
		serializableList := NewListPatternOf(SERIALIZABLE_PATTERN)
		intList := NewListPatternOf(INT_PATTERN)

		migrations, err := serializableList.GetMigrationOperations(ctx, intList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        SERIALIZABLE_PATTERN,
				Next:           INT_PATTERN,
				MigrationMixin: MigrationMixin{"/*"},
			},
		}, migrations)
	})

	t.Run("general element pattern replaced by empty list", func(t *testing.T) {
		intList := NewListPatternOf(INT_PATTERN)
		emptyList := NewListPattern([]Pattern{})

		migrations, err := intList.GetMigrationOperations(ctx, emptyList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        intList,
				Next:           emptyList,
				MigrationMixin: MigrationMixin{"/"},
			},
		}, migrations)
	})

	t.Run("general element pattern replaced by single-elem list of same type", func(t *testing.T) {
		intList := NewListPatternOf(INT_PATTERN)
		singleItemList := NewListPattern([]Pattern{INT_PATTERN})

		migrations, err := intList.GetMigrationOperations(ctx, singleItemList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        intList,
				Next:           singleItemList,
				MigrationMixin: MigrationMixin{"/"},
			},
		}, migrations)
	})

	t.Run("general element pattern replaced by single-elem list of different type", func(t *testing.T) {
		intList := NewListPatternOf(INT_PATTERN)
		singleItemList := NewListPattern([]Pattern{STR_PATTERN})

		migrations, err := intList.GetMigrationOperations(ctx, singleItemList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        intList,
				Next:           singleItemList,
				MigrationMixin: MigrationMixin{"/"},
			},
		}, migrations)
	})

	t.Run("empty list replaced by general element pattern", func(t *testing.T) {
		emptyList := NewListPattern([]Pattern{})
		intList := NewListPatternOf(INT_PATTERN)

		migrations, err := emptyList.GetMigrationOperations(ctx, intList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("single-elem list replaced by general element pattern of different type", func(t *testing.T) {
		singleIntList := NewListPattern([]Pattern{INT_PATTERN})
		stringList := NewListPatternOf(STR_PATTERN)

		migrations, err := singleIntList.GetMigrationOperations(ctx, stringList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        INT_PATTERN,
				Next:           STR_PATTERN,
				MigrationMixin: MigrationMixin{"/0"},
			},
		}, migrations)
	})

	t.Run("single-elem list replaced by general element pattern that is the type of the element", func(t *testing.T) {
		singleIntList := NewListPattern([]Pattern{INT_PATTERN})
		stringList := NewListPatternOf(INT_PATTERN)

		migrations, err := singleIntList.GetMigrationOperations(ctx, stringList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("single-elem list replaced by general element pattern that is a super type of the element", func(t *testing.T) {
		singleIntList := NewListPattern([]Pattern{INT_PATTERN})
		serializableList := NewListPatternOf(SERIALIZABLE_PATTERN)

		migrations, err := singleIntList.GetMigrationOperations(ctx, serializableList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, migrations)
	})

	t.Run("single-elem list replaced by general element pattern that is a sub type of the element", func(t *testing.T) {
		intIntList := NewListPattern([]Pattern{SERIALIZABLE_PATTERN})
		serializableList := NewListPatternOf(INT_PATTERN)

		migrations, err := intIntList.GetMigrationOperations(ctx, serializableList, "/")
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []MigrationOp{
			ReplacementMigrationOp{
				Current:        SERIALIZABLE_PATTERN,
				Next:           INT_PATTERN,
				MigrationMixin: MigrationMixin{"/0"},
			},
		}, migrations)
	})
}

func TestGetMigrationOperations(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)

	intIntList := NewListPattern([]Pattern{SERIALIZABLE_PATTERN})
	serializableList := NewListPatternOf(INT_PATTERN)

	ops, err := GetMigrationOperations(ctx, intIntList, serializableList, "/users*")
	assert.ErrorIs(t, err, ErrInvalidMigrationPseudoPath)
	assert.Nil(t, ops)

	ops, err = GetMigrationOperations(ctx, intIntList, serializableList, "/users/x*")
	assert.ErrorIs(t, err, ErrInvalidMigrationPseudoPath)
	assert.Nil(t, ops)

	ops, err = GetMigrationOperations(ctx, intIntList, serializableList, "/users/*")
	assert.NoError(t, err)
	assert.NotEmpty(t, ops)
}

func TestMigrationOpHandlersFilterByPrefix(t *testing.T) {

	t.Run("root", func(t *testing.T) {
		handlers := MigrationOpHandlers{
			Deletions: map[PathPattern]*MigrationOpHandler{
				"/users/*": nil,
			},
		}

		filtered := handlers.FilterByPrefix("/")
		assert.Equal(t, MigrationOpHandlers{
			Deletions: map[PathPattern]*MigrationOpHandler{
				"/users/*": nil,
			},
		}, filtered)
	})

	t.Run("shallow", func(t *testing.T) {
		handlers := MigrationOpHandlers{
			Deletions: map[PathPattern]*MigrationOpHandler{
				"/users/*":    nil,
				"/messages/*": nil,
			},
		}

		filtered := handlers.FilterByPrefix("/users")
		assert.Equal(t, MigrationOpHandlers{
			Deletions: map[PathPattern]*MigrationOpHandler{
				"/users/*": nil,
			},
		}, filtered)
	})

	t.Run("deep", func(t *testing.T) {
		handlers := MigrationOpHandlers{
			Deletions: map[PathPattern]*MigrationOpHandler{
				"/users/x":    nil,
				"/users/y":    nil,
				"/messages/*": nil,
			},
		}

		filtered := handlers.FilterByPrefix("/users/x")
		assert.Equal(t, MigrationOpHandlers{
			Deletions: map[PathPattern]*MigrationOpHandler{
				"/users/x": nil,
			},
		}, filtered)
	})
}
