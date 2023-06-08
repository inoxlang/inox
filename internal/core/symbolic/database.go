package internal

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	DATABASE_PROPNAMES = []string{"update_schema", "close"}

	ANY_DATABASE = NewDatabaseIL(NewAnyObjectPattern())
)

// A DatabaseIL represents a symbolic DatabaseIL.
type DatabaseIL struct {
	UnassignablePropsMixin
	schema *ObjectPattern
}

func NewDatabaseIL(schema *ObjectPattern) *DatabaseIL {
	return &DatabaseIL{
		schema: schema,
	}
}

func (db *DatabaseIL) Test(v SymbolicValue) bool {
	switch other := v.(type) {
	case *DatabaseIL:
		return db.schema.Test(other.schema)
	default:
		return false
	}
}

func (r *DatabaseIL) WidestOfType() SymbolicValue {
	return ANY_DATABASE
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

func (db *DatabaseIL) Prop(name string) SymbolicValue {
	switch name {
	}
	method, ok := db.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, db))
	}
	return method
}

func (*DatabaseIL) PropertyNames() []string {
	return DATABASE_PROPNAMES
}

func (DatabaseIL *DatabaseIL) UpdateSchema(ctx *Context, schema *ObjectPattern) *Error {
	return nil
}

func (DatabaseIL *DatabaseIL) Close(*Context) *Error {
	return nil
}

func (r *DatabaseIL) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *DatabaseIL) IsWidenable() bool {
	return false
}

func (r *DatabaseIL) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%database")))
	return
}
