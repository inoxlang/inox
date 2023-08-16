package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/commonfmt"
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

func TestObjectMigrate(t *testing.T) {

	t.Run("delete object: / key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		object := NewObjectFromMap(nil, ctx)
		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete object: /x key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		object := NewObjectFromMap(nil, ctx)
		val, err := object.Migrate(ctx, "/x", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/x": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		object := NewObjectFromMap(ValMap{"x": Int(0)}, ctx)
		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/x": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, object, val)
		assert.Equal(t, map[string]Serializable{}, object.EntryMap(ctx))
	})

	t.Run("delete inexisting property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		object := NewObjectFromMap(ValMap{}, ctx)
		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/x": nil,
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtValueAtPathSegmentsDoesNotExist([]string{"x"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete property of property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		object := NewObjectFromMap(ValMap{"a": NewObjectFromMap(ValMap{"b": Int(0)}, ctx)}, ctx)
		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/a/b": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, object, val)
		expectedInner := NewObjectFromMap(ValMap{}, ctx)
		expectedInner.keys = []string{}
		expectedInner.values = []Serializable{}
		assert.Equal(t, map[string]Serializable{"a": expectedInner}, object.EntryMap(ctx))
	})

	t.Run("replace object: / key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		object := NewObjectFromMap(nil, ctx)

		replacement := NewObjectFromMap(nil, ctx)

		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.NotSame(t, replacement, val)
	})

	t.Run("replace object: /x key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		object := NewObjectFromMap(nil, ctx)

		replacement := NewObjectFromMap(nil, ctx)

		val, err := object.Migrate(ctx, "/x", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/x": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.NotSame(t, replacement, val)
	})

	t.Run("replace property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewObjectFromMap(nil, ctx)

		object := NewObjectFromMap(ValMap{"a": NewObjectFromMap(ValMap{"b": Int(0)}, ctx)}, ctx)
		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/a": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Same(t, object, val) {
			return
		}
		if !assert.Equal(t, map[string]Serializable{"a": replacement}, object.EntryMap(ctx)) {
			return
		}

		assert.NotSame(t, replacement, object.Prop(ctx, "a"))
	})

	t.Run("replace inexisting property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewObjectFromMap(nil, ctx)

		object := NewObjectFromMap(ValMap{}, ctx)
		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/a": {InitialValue: replacement},
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtValueAtPathSegmentsDoesNotExist([]string{"a"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("replace property of property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		object := NewObjectFromMap(ValMap{"a": NewObjectFromMap(ValMap{"b": Int(0)}, ctx)}, ctx)
		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/a/b": {InitialValue: Int(1)},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Same(t, object, val) {
			return
		}
		expectedInner := NewObjectFromMap(ValMap{"b": Int(1)}, ctx)
		assert.Equal(t, map[string]Serializable{"a": expectedInner}, object.EntryMap(ctx))
	})

	t.Run("replace property of immutable property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		object := NewObjectFromMap(ValMap{"a": NewRecordFromMap(ValMap{"b": Int(0)})}, ctx)
		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/a/b": {InitialValue: Int(1)},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Same(t, object, val) {
			return
		}
		expectedInner := NewRecordFromMap(ValMap{"b": Int(1)})
		assert.Equal(t, map[string]Serializable{"a": expectedInner}, object.EntryMap(ctx))
	})

	t.Run("include property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		object := NewObjectFromMap(ValMap{}, ctx)
		val, err := object.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Inclusions: map[PathPattern]*MigrationOpHandler{
					"/a": {InitialValue: Int(1)},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, map[string]Serializable{"a": Int(1)}, val.(*Object).EntryMap(ctx))
	})
}

func TestRecordMigrate(t *testing.T) {

	t.Run("delete record: / key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		record := NewRecordFromMap(nil)
		val, err := record.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete record: /x key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		record := NewEmptyRecord()
		val, err := record.Migrate(ctx, "/x", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/x": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		record := NewRecordFromMap(ValMap{"x": Int(0)})
		val, err := record.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/x": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.NotSame(t, record, val)
		assert.Equal(t, map[string]Serializable{}, val.(*Record).EntryMap())
		//original record should not have changed
		assert.Equal(t, map[string]Serializable{"x": Int(0)}, record.EntryMap())
	})

	t.Run("delete inexisting property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		record := NewRecordFromMap(ValMap{})
		val, err := record.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/x": nil,
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtValueAtPathSegmentsDoesNotExist([]string{"x"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete property of property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		record := NewRecordFromMap(ValMap{"a": NewRecordFromMap(ValMap{"b": Int(0)})})
		val, err := record.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/a/b": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.NotSame(t, record, val)
		expectedInner := NewRecordFromMap(ValMap{})
		assert.Equal(t, map[string]Serializable{"a": expectedInner}, val.(*Record).EntryMap())
		//original record should not have changed
		assert.Equal(t, map[string]Serializable{"a": NewRecordFromMap(ValMap{"b": Int(0)})}, record.EntryMap())
	})

	t.Run("replace record: / key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		record := NewEmptyRecord()
		replacement := NewEmptyRecord()

		val, err := record.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.Same(t, replacement, val)
	})

	t.Run("replace record: /x key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		record := NewEmptyRecord()

		replacement := NewEmptyRecord()

		val, err := record.Migrate(ctx, "/x", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/x": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.Same(t, replacement, val)
	})

	t.Run("replace property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewEmptyRecord()

		record := NewRecordFromMap(ValMap{"a": NewRecordFromMap(ValMap{"b": Int(0)})})
		val, err := record.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/a": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotSame(t, record, val) {
			return
		}
		if assert.Equal(t, map[string]Serializable{"a": replacement}, val.(*Record).EntryMap()) {
			return
		}

		assert.Same(t, replacement, record.Prop(ctx, "a"))
		//original record should not have changed
		assert.Equal(t, map[string]Serializable{"a": NewRecordFromMap(ValMap{"b": Int(0)})}, record.EntryMap())
	})

	t.Run("replace inexisting property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewEmptyRecord()

		record := NewRecordFromMap(ValMap{})
		val, err := record.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/a": {InitialValue: replacement},
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtValueAtPathSegmentsDoesNotExist([]string{"a"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("replace property of property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		record := NewRecordFromMap(ValMap{"a": NewRecordFromMap(ValMap{"b": Int(0)})})
		val, err := record.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/a/b": {InitialValue: Int(1)},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotSame(t, record, val) {
			return
		}
		expectedInner := NewRecordFromMap(ValMap{"b": Int(1)})
		assert.Equal(t, map[string]Serializable{"a": expectedInner}, val.(*Record).EntryMap())
		//original record should not have changed
		assert.Equal(t, map[string]Serializable{"a": NewRecordFromMap(ValMap{"b": Int(0)})}, record.EntryMap())
	})

	t.Run("include property", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		record := NewRecordFromMap(ValMap{})
		val, err := record.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Inclusions: map[PathPattern]*MigrationOpHandler{
					"/a": {InitialValue: Int(1)},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, map[string]Serializable{"a": Int(1)}, val.(*Record).EntryMap())
		//original record should not have been updated
		assert.Equal(t, map[string]Serializable{}, record.EntryMap())
	})
}

func TestListMigrate(t *testing.T) {

	t.Run("delete list: / key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := NewWrappedValueList(nil)
		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete list: /x key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := NewWrappedValueList()
		val, err := list.Migrate(ctx, "/x", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/x": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := NewWrappedValueList(Int(0))
		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/0": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, list, val)
		assert.Equal(t, []Serializable{}, list.GetOrBuildElements(ctx))
	})

	t.Run("delete inexisting element (index >= len)", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := NewWrappedValueList(Int(0))
		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/1": nil,
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtLastSegmentOfMigrationPathIsOutOfBounds([]string{"1"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete inexisting element (index < 0)", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := NewWrappedValueList(Int(0))
		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/-1": nil,
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtLastSegmentOfMigrationPathIsOutOfBounds([]string{"-1"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete property of element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := NewWrappedValueList(NewObjectFromMap(ValMap{"b": Int(0)}, ctx))
		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/0/b": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, list, val)
		expectedInner := NewObjectFromMap(ValMap{}, ctx)
		expectedInner.keys = []string{}
		expectedInner.values = []Serializable{}
		assert.Equal(t, []Serializable{expectedInner}, list.GetOrBuildElements(ctx))
	})

	t.Run("replace list: / key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := NewWrappedValueList()
		replacement := NewWrappedValueList()

		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.NotSame(t, replacement, val)
	})

	t.Run("replace list: /x key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := NewWrappedValueList()
		replacement := NewWrappedValueList()

		val, err := list.Migrate(ctx, "/x", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/x": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.NotSame(t, replacement, val)
	})

	t.Run("replace element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewObjectFromMap(nil, ctx)

		list := NewWrappedValueList(NewObjectFromMap(ValMap{"b": Int(0)}, ctx))
		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/0": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Same(t, list, val) {
			return
		}
		if !assert.Equal(t, []Serializable{replacement}, list.GetOrBuildElements(ctx)) {
			return
		}

		assert.NotSame(t, replacement, list.At(ctx, 0))
	})

	t.Run("replace inexisting element (index >= len)", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewWrappedValueList()
		list := NewWrappedValueList()

		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/0": {InitialValue: replacement},
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtLastSegmentOfMigrationPathIsOutOfBounds([]string{"0"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("replace inexisting element (index < 0)", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewWrappedValueList()
		list := NewWrappedValueList()

		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/-1": {InitialValue: replacement},
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtLastSegmentOfMigrationPathIsOutOfBounds([]string{"-1"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("replace property of element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		list := NewWrappedValueList(NewObjectFromMap(ValMap{"b": Int(0)}, ctx))
		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/0/b": {InitialValue: Int(1)},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Same(t, list, val) {
			return
		}
		expectedInner := NewObjectFromMap(ValMap{"b": Int(1)}, ctx)
		assert.Equal(t, []Serializable{expectedInner}, list.GetOrBuildElements(ctx))
	})

	t.Run("replace property of immutable element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		list := NewWrappedValueList(NewRecordFromMap(ValMap{"b": Int(0)}))
		val, err := list.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/0/b": {InitialValue: Int(1)},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Same(t, list, val) {
			return
		}
		expectedInner := NewRecordFromMap(ValMap{"b": Int(1)})
		assert.Equal(t, []Serializable{expectedInner}, list.GetOrBuildElements(ctx))
	})

	t.Run("element inclusion should panic", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		list := NewWrappedValueList()

		assert.PanicsWithError(t, ErrUnreachable.Error(), func() {
			list.Migrate(ctx, "/", &InstanceMigrationArgs{
				NextPattern: nil,
				MigrationHandlers: MigrationOpHandlers{
					Inclusions: map[PathPattern]*MigrationOpHandler{
						"/0": {InitialValue: Int(1)},
					},
				},
			})
		})
	})
}

func TestTupleMigrate(t *testing.T) {

	t.Run("delete tuple: / key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		tuple := NewTuple(nil)
		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete tuple: /x key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		tuple := NewTuple(nil)
		val, err := tuple.Migrate(ctx, "/x", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/x": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		tuple := NewTuple([]Serializable{Int(0)})
		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/0": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.NotSame(t, tuple, val)
		assert.Equal(t, []Serializable{}, val.(*Tuple).GetOrBuildElements(ctx))
		//original tuple should not have changed
		assert.Equal(t, []Serializable{Int(0)}, tuple.GetOrBuildElements(ctx))
	})

	t.Run("delete inexisting element (index >= len)", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		tuple := NewTuple([]Serializable{Int(0)})
		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/1": nil,
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtLastSegmentOfMigrationPathIsOutOfBounds([]string{"1"})) {
			return
		}
		assert.Nil(t, val)
		//original tuple should not have changed
		assert.Equal(t, []Serializable{Int(0)}, tuple.GetOrBuildElements(ctx))
	})

	t.Run("delete inexisting element (index < 0)", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		tuple := NewTuple([]Serializable{Int(0)})
		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/-1": nil,
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtLastSegmentOfMigrationPathIsOutOfBounds([]string{"-1"})) {
			return
		}
		assert.Nil(t, val)
		//original tuple should not have changed
		assert.Equal(t, []Serializable{Int(0)}, tuple.GetOrBuildElements(ctx))
	})

	t.Run("delete property of element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		tuple := NewTuple([]Serializable{NewRecordFromMap(ValMap{"b": Int(0)})})
		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Deletions: map[PathPattern]*MigrationOpHandler{
					"/0/b": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.NotSame(t, tuple, val)
		expectedInner := NewRecordFromMap(ValMap{})
		assert.Equal(t, []Serializable{expectedInner}, val.(*Tuple).GetOrBuildElements(ctx))
		//original tuple should not have changed
		assert.Equal(t, []Serializable{NewRecordFromMap(ValMap{"b": Int(0)})}, tuple.GetOrBuildElements(ctx))
	})

	t.Run("replace tuple: / key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		tuple := NewTuple(nil)
		replacement := NewTuple(nil)

		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.Same(t, replacement, val)
	})

	t.Run("replace tuple: /x key", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		tuple := NewTuple(nil)
		replacement := NewTuple(nil)

		val, err := tuple.Migrate(ctx, "/x", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/x": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.Same(t, replacement, val)
	})

	t.Run("replace element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewRecordFromMap(nil)

		tuple := NewTuple([]Serializable{NewRecordFromMap(ValMap{"b": Int(0)})})
		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/0": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotSame(t, tuple, val) {
			return
		}
		if !assert.Equal(t, []Serializable{replacement}, val.(*Tuple).GetOrBuildElements(ctx)) {
			return
		}

		assert.Same(t, replacement, val.(*Tuple).At(ctx, 0))

		//original tuple should not have changed
		assert.Equal(t, []Serializable{NewRecordFromMap(ValMap{"b": Int(0)})}, tuple.GetOrBuildElements(ctx))
	})

	t.Run("replace inexisting element (index >= len)", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewWrappedValueList()
		tuple := NewTuple(nil)

		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/0": {InitialValue: replacement},
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtLastSegmentOfMigrationPathIsOutOfBounds([]string{"0"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("replace inexisting element (index < 0)", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		replacement := NewTuple(nil)
		tuple := NewTuple(nil)

		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/-1": {InitialValue: replacement},
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtLastSegmentOfMigrationPathIsOutOfBounds([]string{"-1"})) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("replace property of element", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		tuple := NewTuple([]Serializable{NewRecordFromMap(ValMap{"b": Int(0)})})
		val, err := tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: MigrationOpHandlers{
				Replacements: map[PathPattern]*MigrationOpHandler{
					"/0/b": {InitialValue: Int(1)},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotSame(t, tuple, val) {
			return
		}
		expectedInner := NewRecordFromMap(ValMap{"b": Int(1)})
		assert.Equal(t, []Serializable{expectedInner}, val.(*Tuple).GetOrBuildElements(ctx))
		//original tuple should not have changed
		assert.Equal(t, []Serializable{NewRecordFromMap(ValMap{"b": Int(0)})}, tuple.GetOrBuildElements(ctx))
	})

	t.Run("element inclusion should panic", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)

		tuple := NewTuple(nil)

		assert.PanicsWithError(t, ErrUnreachable.Error(), func() {
			tuple.Migrate(ctx, "/", &InstanceMigrationArgs{
				NextPattern: nil,
				MigrationHandlers: MigrationOpHandlers{
					Inclusions: map[PathPattern]*MigrationOpHandler{
						"/0": {InitialValue: Int(1)},
					},
				},
			})
		})
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

	ops, err = GetMigrationOperations(ctx, intIntList, serializableList, "/users?")
	assert.ErrorIs(t, err, ErrInvalidMigrationPseudoPath)
	assert.Nil(t, ops)

	ops, err = GetMigrationOperations(ctx, intIntList, serializableList, "/users/x?")
	assert.ErrorIs(t, err, ErrInvalidMigrationPseudoPath)
	assert.Nil(t, ops)

	ops, err = GetMigrationOperations(ctx, intIntList, serializableList, "/users/?")
	assert.ErrorIs(t, err, ErrInvalidMigrationPseudoPath)
	assert.Nil(t, ops)

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
