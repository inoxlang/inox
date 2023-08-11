package core

import (
	"bufio"
	"io"
	"reflect"
	"strconv"
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	internal "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestDatabaseIL(t *testing.T) {

	RegisterLoadInstanceFn(reflect.TypeOf(LOADABLE_TEST_VALUE_PATTERN), func(ctx *Context, args InstanceLoadArgs) (UrlHolder, error) {
		assert.Fail(t, "should never be called")
		return nil, nil
	})

	t.Run("", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
		}

		dbIL := WrapDatabase(ctx, DatabaseWrappingArgs{
			Inner:      db,
			OwnerState: ctx.state,
		})

		assert.Equal(t, map[string]Serializable{"a": &loadableTestValue{
			value: 1,
		}}, dbIL.topLevelEntities)
	})

	t.Run("if a schema update is expected top level entiries should not be loaded", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
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
				"a": LOADABLE_TEST_VALUE_PATTERN,
			}))
		})
	})

	t.Run("accessing the database while its schema is not yet updated should cause a panic", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		db := &dummyDatabase{
			resource: Host("ldb://main"),
			topLevelEntities: map[string]Serializable{"a": &loadableTestValue{
				value: 1,
			}},
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

	t.Run("accessing the database after its schema is updated should work", func(t *testing.T) {
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

		dbIL.UpdateSchema(ctx, NewInexactObjectPattern(map[string]Pattern{
			"a": LOADABLE_TEST_VALUE_PATTERN,
		}), NewObjectFromMap(ValMap{
			symbolic.DB_MIGRATION__INCLUSIONS_PROP_NAME: NewDictionary(ValMap{
				"%/a": handler,
			}),
		}, ctx))

		val := dbIL.Prop(ctx, "a")
		assert.Same(t, db.topLevelEntities["a"], val)
	})
}

var (
	_ UrlHolder              = (*loadableTestValue)(nil)
	_ Pattern                = (*loadableTestValuePattern)(nil)
	_ symbolic.Pattern       = (*symbolicLoadableTestValuePattern)(nil)
	_ symbolic.SymbolicValue = (*symbolicLoadableTestValue)(nil)

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

func (*loadableTestValue) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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

func (*loadableTestValuePattern) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
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
func (*symbolicLoadableTestValue) Concretize() any {
	return &loadableTestValue{}
}

func (*symbolicLoadableTestValue) IsMutable() bool {
	return false
}

func (*symbolicLoadableTestValue) PrettyPrint(w *bufio.Writer, config *internal.PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("symbolicLoadableTestValue")
}

func (*symbolicLoadableTestValue) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}

func (*symbolicLoadableTestValue) Test(v symbolic.SymbolicValue) bool {
	panic(ErrNotImplementedYet)
}

func (*symbolicLoadableTestValue) WidestOfType() symbolic.SymbolicValue {
	return &symbolicLoadableTestValue{}
}

type symbolicLoadableTestValuePattern struct {
	symbolic.NotCallablePatternMixin
	symbolic.SerializableMixin
}

func (*symbolicLoadableTestValuePattern) IsConcretizable() bool {
	return true
}
func (*symbolicLoadableTestValuePattern) Concretize() any {
	return &loadableTestValuePattern{}
}

func (*symbolicLoadableTestValuePattern) HasUnderylingPattern() bool {
	return true
}

func (*symbolicLoadableTestValuePattern) IsMutable() bool {
	return false
}

func (*symbolicLoadableTestValuePattern) IteratorElementKey() symbolic.SymbolicValue {
	return symbolic.ANY_INT
}

func (*symbolicLoadableTestValuePattern) IteratorElementValue() symbolic.SymbolicValue {
	return symbolic.ANY
}

func (*symbolicLoadableTestValuePattern) PrettyPrint(w *bufio.Writer, config *internal.PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("symbolicLoadableTestValuePattern")
}

func (*symbolicLoadableTestValuePattern) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}

func (*symbolicLoadableTestValuePattern) SymbolicValue() symbolic.SymbolicValue {
	panic(ErrNotImplementedYet)
}

func (*symbolicLoadableTestValuePattern) Test(v symbolic.SymbolicValue) bool {
	panic(ErrNotImplementedYet)
}

func (*symbolicLoadableTestValuePattern) TestValue(v symbolic.SymbolicValue) bool {
	panic(ErrNotImplementedYet)
}

func (*symbolicLoadableTestValuePattern) WidestOfType() symbolic.SymbolicValue {
	return &symbolicLoadableTestValuePattern{}
}
