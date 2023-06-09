package symbolic

import (
	"bufio"
	"fmt"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	DATABASE_PROPNAMES = []string{"update_schema", "close", "schema"}

	ANY_DATABASE = NewDatabaseIL(NewAnyObjectPattern())
)

// A DatabaseIL represents a symbolic DatabaseIL.
type DatabaseIL struct {
	UnassignablePropsMixin
	schema        *ObjectPattern
	propertyNames []string
}

func NewDatabaseIL(schema *ObjectPattern) *DatabaseIL {
	propertyNames := utils.CopySlice(DATABASE_PROPNAMES)
	for propName := range schema.entries {
		if utils.SliceContains(DATABASE_PROPNAMES, propName) {
			panic(fmt.Errorf("name collision with inital property name '%s'", propName))
		}
		propertyNames = append(propertyNames, propName)
	}

	return &DatabaseIL{
		schema:        schema,
		propertyNames: propertyNames,
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
	case "schema":
		return db.schema
	}

	entry, ok := db.schema.entries[name]
	if ok {
		return entry.SymbolicValue()
	}

	method, ok := db.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, db))
	}
	return method
}

func (db *DatabaseIL) PropertyNames() []string {
	return db.propertyNames
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
