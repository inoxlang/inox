package core

import (
	"bufio"
	"io"
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/prettyprint"
	internal "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestDatabaseIL(t *testing.T) {
	runtime.GC()
	startMemStats := new(runtime.MemStats)
	runtime.ReadMemStats(startMemStats)

	defer func() {
		utils.AssertNoGoroutineLeak(t, runtime.NumGoroutine())
	}()

	t.Run("the name of the top level entities should be a valid identifier", func(t *testing.T) {
		resetLoadInstanceFnRegistry()
		defer resetLoadInstanceFnRegistry()

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
			Name:                 "main",
		}))

		migrationHandlerReturnedVal := &loadableTestValue{value: 1}

		symbolicFn := symbolic.NewInoxFunction(nil, nil, &symbolicLoadableTestValue{})
		handler := &InoxFunction{
			Node: &parse.FunctionExpression{
				IsBodyExpression: true,
				Body: &parse.IdentifierLiteral{
					Name: "val",
				},
			},
			treeWalkCapturedLocals: map[string]Value{
				"val": migrationHandlerReturnedVal,
			},
			symbolicValue: symbolicFn,
			staticData:    &FunctionStaticData{},
		}

		func() {
			defer func() {
				v := recover()
				if !assert.NotNil(t, v) {
					return
				}
				assert.ErrorIs(t, v.(error), ErrTopLevelEntityNamesShouldBeValidInoxIdentifiers)
			}()

			dbIL.UpdateSchema(ctx, NewInexactObjectPattern(map[string]Pattern{
				"a-": LOADABLE_TEST_VALUE_PATTERN,
			}), NewObjectFromMap(ValMap{
				symbolic.DB_MIGRATION__INCLUSIONS_PROP_NAME: NewDictionary(ValMap{
					"%/a-": handler,
				}),
			}, ctx))
		}()
	})

	t.Run("top-level entity patterns should have a loading function", func(t *testing.T) {
		resetLoadInstanceFnRegistry()
		defer resetLoadInstanceFnRegistry()

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
			Name:                 "main",
		}))

		migrationHandlerReturnedVal := &loadableTestValue{value: 1}

		symbolicFn := symbolic.NewInoxFunction(nil, nil, &symbolicLoadableTestValue{})
		handler := &InoxFunction{
			Node: &parse.FunctionExpression{
				IsBodyExpression: true,
				Body: &parse.IdentifierLiteral{
					Name: "val",
				},
			},
			treeWalkCapturedLocals: map[string]Value{
				"val": migrationHandlerReturnedVal,
			},
			symbolicValue: symbolicFn,
			staticData:    &FunctionStaticData{},
		}

		func() {
			defer func() {
				v := recover()
				if !assert.NotNil(t, v) {
					return
				}
				assert.ErrorContains(t, v.(error), "invalid pattern for top level entity .a")
			}()

			dbIL.UpdateSchema(ctx, NewInexactObjectPattern(map[string]Pattern{
				"a": LOADABLE_TEST_VALUE_PATTERN,
			}), NewObjectFromMap(ValMap{
				symbolic.DB_MIGRATION__INCLUSIONS_PROP_NAME: NewDictionary(ValMap{
					"%/a": handler,
				}),
			}, ctx))
		}()
	})

	t.Run("base case", func(t *testing.T) {
		resetLoadInstanceFnRegistry()
		defer resetLoadInstanceFnRegistry()

		RegisterLoadInstanceFn(reflect.TypeOf(LOADABLE_TEST_VALUE_PATTERN), func(ctx *Context, args InstanceLoadArgs) (UrlHolder, error) {
			assert.Fail(t, "should never be called")
			return nil, nil
		})

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                        db,
			OwnerState:                   ctx.state,
			ForceLoadBeforeOwnerStateSet: true,
		}))

		assert.Equal(t, map[string]Serializable{"a": &loadableTestValue{
			value: 1,
		}}, dbIL.topLevelEntities)
	})

	t.Run("if the current schema is not equal to the expected schema an error should be returned", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		expectedSchema := NewInexactObjectPattern(map[string]Pattern{
			"a": LOADABLE_TEST_VALUE_PATTERN,
			"b": LOADABLE_TEST_VALUE_PATTERN,
		})

		dbIL, err := WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:          db,
			OwnerState:     ctx.state,
			Name:           "main",
			ExpectedSchema: expectedSchema,
		})

		if !assert.Nil(t, dbIL) {
			return
		}
		assert.ErrorIs(t, err, ErrCurrentSchemaNotEqualToExpectedSchema)
	})

	t.Run("if the current schema is not equal to the expected schema in dev mode the expected schema should be used", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		expectedSchema := NewInexactObjectPattern(map[string]Pattern{
			"a": LOADABLE_TEST_VALUE_PATTERN,
			"b": LOADABLE_TEST_VALUE_PATTERN,
		})

		dbIL, err := WrapDatabase(ctx, DatabaseWrappingArgs{
			DevMode:        true,
			Inner:          db,
			OwnerState:     ctx.state,
			Name:           "main",
			ExpectedSchema: expectedSchema,
		})

		assert.ErrorIs(t, err, ErrCurrentSchemaNotEqualToExpectedSchema)
		if !assert.NotNil(t, dbIL) {
			return
		}

		assert.Same(t, expectedSchema, dbIL.newSchema)
		assert.Same(t, expectedSchema, dbIL.Prop(ctx, "schema"))
	})

	t.Run("if a schema update is expected top level entities should not be loaded", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
			Name:                 "main",
		}))

		assert.Nil(t, dbIL.topLevelEntities)
	})

	t.Run("if a schema update is expected top level entities should not be loaded after call to SetOwnerStateOnceAndLoadIfNecessary", func(t *testing.T) {
		resetLoadInstanceFnRegistry()
		defer resetLoadInstanceFnRegistry()

		RegisterLoadInstanceFn(reflect.TypeOf(LOADABLE_TEST_VALUE_PATTERN), func(ctx *Context, args InstanceLoadArgs) (UrlHolder, error) {
			assert.Fail(t, "should never be called")
			return nil, nil
		})

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			ExpectedSchemaUpdate: true,
			Name:                 "main",
		}))

		dbIL.SetOwnerStateOnceAndLoadIfNecessary(ctx, ctx.state)

		assert.Nil(t, dbIL.topLevelEntities)
	})

	t.Run("if dev mode is enabled top level entities should not be loaded after call to SetOwnerStateOnceAndLoadIfNecessary", func(t *testing.T) {
		resetLoadInstanceFnRegistry()
		defer resetLoadInstanceFnRegistry()

		RegisterLoadInstanceFn(reflect.TypeOf(LOADABLE_TEST_VALUE_PATTERN), func(ctx *Context, args InstanceLoadArgs) (UrlHolder, error) {
			assert.Fail(t, "should never be called")
			return nil, nil
		})

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			ExpectedSchemaUpdate: true,
			Name:                 "main",
			DevMode:              true,
		}))

		assert.Nil(t, dbIL.topLevelEntities)

		dbIL.SetOwnerStateOnceAndLoadIfNecessary(ctx, ctx.state)

		assert.Nil(t, dbIL.topLevelEntities)
	})

	t.Run("top level entities should not be loaded after call to SetOwnerStateOnceAndLoadIfNecessary if no settings are set", func(t *testing.T) {
		resetLoadInstanceFnRegistry()
		defer resetLoadInstanceFnRegistry()

		RegisterLoadInstanceFn(reflect.TypeOf(LOADABLE_TEST_VALUE_PATTERN), func(ctx *Context, args InstanceLoadArgs) (UrlHolder, error) {
			assert.Fail(t, "should never be called")
			return nil, nil
		})

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner: db,
			Name:  "main",
		}))

		assert.Nil(t, dbIL.topLevelEntities)

		dbIL.SetOwnerStateOnceAndLoadIfNecessary(ctx, ctx.state)

		assert.NotNil(t, dbIL.topLevelEntities)
	})

	t.Run("only the owner state should be able to update the schema", func(t *testing.T) {
		ctx1 := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx1.CancelGracefully()

		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := utils.Must(WrapDatabase(ctx1, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx1.state,
			ExpectedSchemaUpdate: true,
			Name:                 "main",
		}))

		ctx2 := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx2.CancelGracefully()

		assert.PanicsWithValue(t, ErrDatabaseSchemaOnlyUpdatableByOwnerState, func() {
			dbIL.UpdateSchema(ctx2, NewInexactObjectPattern(map[string]Pattern{}))
		})
	})

	t.Run("updating the schema while it not expected should cause a panic", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: false,
		}))

		assert.PanicsWithValue(t, ErrNoDatabaseSchemaUpdateExpected, func() {
			dbIL.UpdateSchema(ctx, NewInexactObjectPattern(map[string]Pattern{}))
		})
	})

	t.Run("updating the schema twice should cause a panic", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
			Name:                 "main",
		}))

		dbIL.UpdateSchema(ctx, NewInexactObjectPattern(map[string]Pattern{}))

		assert.PanicsWithValue(t, ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed, func() {
			dbIL.UpdateSchema(ctx, NewInexactObjectPattern(map[string]Pattern{
				"a": LOADABLE_TEST_VALUE_PATTERN,
			}))
		})
	})

	t.Run("accessing the database while its schema is not yet updated should cause a panic", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
			Name:                 "main",
		}))

		assert.PanicsWithValue(t, ErrInvalidAccessSchemaNotUpdatedYet, func() {
			dbIL.Prop(ctx, "a")
		})
	})

	t.Run("accessing the database after its schema is updated should work", func(t *testing.T) {
		resetLoadInstanceFnRegistry()
		defer resetLoadInstanceFnRegistry()

		RegisterLoadInstanceFn(reflect.TypeOf(LOADABLE_TEST_VALUE_PATTERN), func(ctx *Context, args InstanceLoadArgs) (UrlHolder, error) {
			assert.Fail(t, "should never be called")
			return nil, nil
		})

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
			Name:                 "main",
		}))

		migrationHandlerReturnedVal := &loadableTestValue{value: 1}

		symbolicFn := symbolic.NewInoxFunction(nil, nil, &symbolicLoadableTestValue{})
		handler := &InoxFunction{
			Node: &parse.FunctionExpression{
				IsBodyExpression: true,
				Body: &parse.IdentifierLiteral{
					Name: "val",
				},
			},
			treeWalkCapturedLocals: map[string]Value{
				"val": migrationHandlerReturnedVal,
			},
			symbolicValue: symbolicFn,
			staticData:    &FunctionStaticData{},
		}

		newSchema := NewInexactObjectPattern(map[string]Pattern{
			"a": LOADABLE_TEST_VALUE_PATTERN,
		})
		dbIL.UpdateSchema(ctx, newSchema, NewObjectFromMap(ValMap{
			symbolic.DB_MIGRATION__INCLUSIONS_PROP_NAME: NewDictionary(ValMap{
				"%/a": handler,
			}),
		}, ctx))

		val := dbIL.Prop(ctx, "a")
		assert.Same(t, db.topLevelEntities["a"], val)

		assert.Same(t, newSchema, dbIL.Prop(ctx, "schema"))
	})

	t.Run("updating the database to a schema not equal to the expected schema should cause an error", func(t *testing.T) {
		resetLoadInstanceFnRegistry()
		defer resetLoadInstanceFnRegistry()

		RegisterLoadInstanceFn(reflect.TypeOf(LOADABLE_TEST_VALUE_PATTERN), func(ctx *Context, args InstanceLoadArgs) (UrlHolder, error) {
			assert.Fail(t, "should never be called")
			return nil, nil
		})

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				DatabasePermission{
					Kind_:  permkind.Read,
					Entity: Host("ldb://main"),
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		newSchema := NewInexactObjectPattern(map[string]Pattern{
			"a": LOADABLE_TEST_VALUE_PATTERN,
		})

		expectedSchema := NewInexactObjectPattern(map[string]Pattern{
			"a": LOADABLE_TEST_VALUE_PATTERN,
			"b": LOADABLE_TEST_VALUE_PATTERN,
		})

		db := &dummyDatabase{
			resource:         Host("ldb://main"),
			topLevelEntities: map[string]Serializable{},
		}

		dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
			Name:                 "main",
			Inner:                db,
			OwnerState:           ctx.state,
			ExpectedSchemaUpdate: true,
			ExpectedSchema:       expectedSchema,
		}))

		migrationHandlerReturnedVal := &loadableTestValue{value: 1}

		symbolicFn := symbolic.NewInoxFunction(nil, nil, &symbolicLoadableTestValue{})
		handler := &InoxFunction{
			Node: &parse.FunctionExpression{
				IsBodyExpression: true,
				Body: &parse.IdentifierLiteral{
					Name: "val",
				},
			},
			treeWalkCapturedLocals: map[string]Value{
				"val": migrationHandlerReturnedVal,
			},
			symbolicValue: symbolicFn,
			staticData:    &FunctionStaticData{},
		}

		assert.PanicsWithError(t, ErrNewSchemaNotEqualToExpectedSchema.Error(), func() {
			dbIL.UpdateSchema(ctx, newSchema, NewObjectFromMap(ValMap{
				symbolic.DB_MIGRATION__INCLUSIONS_PROP_NAME: NewDictionary(ValMap{
					"%/a": handler,
				}),
			}, ctx))
		})
	})

	t.Run("gracefully cancelling the context should close the database", func(t *testing.T) {
		t.Run("owner state set during WrapDatabase call", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{
				Permissions: []Permission{
					DatabasePermission{
						Kind_:  permkind.Read,
						Entity: Host("ldb://main"),
					},
				},
			}, nil)
			defer ctx.CancelGracefully()

			db := &dummyDatabase{
				resource:         Host("ldb://main"),
				topLevelEntities: map[string]Serializable{},
			}

			utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
				Inner:                db,
				OwnerState:           ctx.state,
				ExpectedSchemaUpdate: true,
				Name:                 "main",
			}))

			assert.False(t, db.closed.Load())
			ctx.CancelGracefully()
			assert.True(t, db.closed.Load())
		})

		t.Run("owner state set during WrapDatabase call", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{
				Permissions: []Permission{
					DatabasePermission{
						Kind_:  permkind.Read,
						Entity: Host("ldb://main"),
					},
				},
			}, nil)
			defer ctx.CancelGracefully()

			db := &dummyDatabase{
				resource:         Host("ldb://main"),
				topLevelEntities: map[string]Serializable{},
			}

			dbIL := utils.Must(WrapDatabase(ctx, DatabaseWrappingArgs{
				Inner: db,
				Name:  "main",
			}))

			dbIL.SetOwnerStateOnceAndLoadIfNecessary(ctx, ctx.GetClosestState())

			assert.False(t, db.closed.Load())
			ctx.CancelGracefully()
			assert.True(t, db.closed.Load())
		})
	})

}

var (
	_ UrlHolder        = (*loadableTestValue)(nil)
	_ Pattern          = (*loadableTestValuePattern)(nil)
	_ symbolic.Pattern = (*symbolicLoadableTestValuePattern)(nil)
	_ symbolic.Value   = (*symbolicLoadableTestValue)(nil)

	LOADABLE_TEST_VALUE_PATTERN = &loadableTestValuePattern{}
)

type loadableTestValue struct {
	value int32
	url   URL
}

func (*loadableTestValue) SetURLOnce(ctx *Context, u URL) error {
	panic(ErrNotImplemented)
}

func (v *loadableTestValue) URL() (URL, bool) {
	panic(ErrNotImplemented)
}

func (*loadableTestValue) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	panic(ErrNotImplemented)
}

func (*loadableTestValue) IsMutable() bool {
	return true
}

func (*loadableTestValue) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	panic(ErrNotImplemented)
}

func (*loadableTestValue) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	panic(ErrNotImplemented)
}

func (v *loadableTestValue) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	w.WriteInt(int(v.value))
	return nil
}

func (v *loadableTestValue) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	w.Write(utils.StringAsBytes(strconv.FormatInt(int64(v.value), 10)))
	return nil
}

type loadableTestValuePattern struct {
	NotCallablePatternMixin
}

func (*loadableTestValuePattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	panic(ErrNotImplemented)
}

func (*loadableTestValuePattern) IsMutable() bool {
	return false
}

func (*loadableTestValuePattern) Iterator(*Context, IteratorConfiguration) Iterator {
	panic(ErrNotImplemented)
}

func (*loadableTestValuePattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	panic(ErrNotImplemented)
}

func (*loadableTestValuePattern) Random(ctx *Context, options ...Option) Value {
	panic(ErrNotImplemented)
}

func (*loadableTestValuePattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (*loadableTestValuePattern) Test(ctx *Context, val Value) bool {
	_, ok := val.(*loadableTestValue)
	return ok
}

func (*loadableTestValuePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolicLoadableTestValuePattern{}, nil
}

func (*loadableTestValuePattern) WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error {
	panic(ErrNotImplemented)
}

func (*loadableTestValuePattern) WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error {
	panic(ErrNotImplemented)
}

type symbolicLoadableTestValue struct {
}

func (*symbolicLoadableTestValue) IsConcretizable() bool {
	return true
}
func (*symbolicLoadableTestValue) Concretize(ctx symbolic.ConcreteContext) any {
	return &loadableTestValue{}
}

func (*symbolicLoadableTestValue) IsMutable() bool {
	return false
}

func (*symbolicLoadableTestValue) PrettyPrint(w prettyprint.PrettyPrintWriter, config *internal.PrettyPrintConfig) {
	w.WriteString("symbolicLoadableTestValue")
}

func (*symbolicLoadableTestValue) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}

func (*symbolicLoadableTestValue) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	panic(ErrNotImplementedYet)
}

func (*symbolicLoadableTestValue) WidestOfType() symbolic.Value {
	return &symbolicLoadableTestValue{}
}

type symbolicLoadableTestValuePattern struct {
	symbolic.NotCallablePatternMixin
	symbolic.SerializableMixin
}

func (*symbolicLoadableTestValuePattern) IsConcretizable() bool {
	return true
}
func (*symbolicLoadableTestValuePattern) Concretize(ctx symbolic.ConcreteContext) any {
	return &loadableTestValuePattern{}
}

func (*symbolicLoadableTestValuePattern) HasUnderlyingPattern() bool {
	return true
}

func (*symbolicLoadableTestValuePattern) IsMutable() bool {
	return false
}

func (*symbolicLoadableTestValuePattern) IteratorElementKey() symbolic.Value {
	return symbolic.ANY_INT
}

func (*symbolicLoadableTestValuePattern) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (*symbolicLoadableTestValuePattern) PrettyPrint(w prettyprint.PrettyPrintWriter, config *internal.PrettyPrintConfig) {
	w.WriteString("symbolicLoadableTestValuePattern")
}

func (*symbolicLoadableTestValuePattern) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}

func (*symbolicLoadableTestValuePattern) SymbolicValue() symbolic.Value {
	panic(ErrNotImplementedYet)
}

func (*symbolicLoadableTestValuePattern) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	panic(ErrNotImplementedYet)
}

func (*symbolicLoadableTestValuePattern) TestValue(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	panic(ErrNotImplementedYet)
}

func (*symbolicLoadableTestValuePattern) WidestOfType() symbolic.Value {
	return &symbolicLoadableTestValuePattern{}
}
