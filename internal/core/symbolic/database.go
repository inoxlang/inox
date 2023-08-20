package symbolic

import (
	"bufio"
	"fmt"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DB_MIGRATION__DELETIONS_PROP_NAME       = "deletions"
	DB_MIGRATION__INCLUSIONS_PROP_NAME      = "inclusions"
	DB_MIGRATION__REPLACEMENTS_PROP_NAME    = "replacements"
	DB_MIGRATION__INITIALIZATIONS_PROP_NAME = "initializations"
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

func (*DatabaseIL) WidestOfType() SymbolicValue {
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

func (db *DatabaseIL) UpdateSchema(ctx *Context, schema *ObjectPattern, additionalArgs ...*Object) {
	if !db.schema.IsConcretizable() {
		ctx.AddSymbolicGoFunctionError("previous schema is not concretizable, it should only contain values/patterns that can be known at check time")
		return
	}

	if !schema.IsConcretizable() {
		ctx.AddSymbolicGoFunctionError("new schema is not concretizable, it should only contain values/patterns that can be known at check time")
		return
	}

	currentConcreteSchema := db.schema.Concretize(ctx.startingConcreteContext)
	nextConcreteSchema := schema.Concretize(ctx.startingConcreteContext)

	if err := extData.CheckDatabaseSchema(nextConcreteSchema); err != nil {
		ctx.AddSymbolicGoFunctionError(err.Error())
		return
	}

	ops, err := extData.GetTopLevelEntitiesMigrationOperations(ctx.startingConcreteContext, currentConcreteSchema, nextConcreteSchema)
	if err != nil {
		ctx.AddSymbolicGoFunctionError(err.Error())
		return
	}

	if len(ops) == 0 {
		if len(additionalArgs) > 0 {
			expectedObject := &Object{
				entries: map[string]Serializable{},
				exact:   true,
			}

			ctx.SetSymbolicGoFunctionParameters(&[]SymbolicValue{ANY_OBJECT_PATTERN, expectedObject}, []string{"new-schema", "migrations"})
			return
		}
	} else {
		if len(additionalArgs) == 0 {
			ctx.AddSymbolicGoFunctionError("migration logic argument is required")
			return
		}
		replacements := utils.FilterSliceByType(ops, ReplacementMigrationOp{})
		deletions := utils.FilterSliceByType(ops, RemovalMigrationOp{})
		inclusions := utils.FilterSliceByType(ops, InclusionMigrationOp{})
		initializations := utils.FilterSliceByType(ops, NillableInitializationMigrationOp{})

		expectedObject := &Object{
			entries: map[string]Serializable{},
			exact:   true,
		}

		if len(replacements) > 0 {
			dict := &Dictionary{
				entries: map[string]Serializable{},
				keys:    map[string]Serializable{},
			}

			for _, op := range replacements {
				pathPattern := "%" + op.PseudoPath
				var entryValue Serializable = &InoxFunction{
					parameters:     []SymbolicValue{op.Current.SymbolicValue()},
					parameterNames: []string{"previous-value"},
					result:         op.Next.SymbolicValue(),
					visitCheckNode: isNodeAllowedInMigrationHandler,
				}

				capable, ok := op.Next.(MigrationInitialValueCapablePattern)
				if ok {
					acceptedInitialValue, ok := capable.MigrationInitialValue()
					if ok {
						entryValue = AsSerializable(joinValues([]SymbolicValue{entryValue, acceptedInitialValue})).(Serializable)
					}
				}

				dict.entries[pathPattern] = entryValue
			}

			expectedObject.entries[DB_MIGRATION__REPLACEMENTS_PROP_NAME] = dict
		}

		if len(deletions) > 0 {
			dict := &Dictionary{
				entries: map[string]Serializable{},
				keys:    map[string]Serializable{},
			}

			for _, op := range deletions {
				pathPattern := "%" + op.PseudoPath
				dict.entries[pathPattern] = AsSerializable(NewMultivalue(
					&InoxFunction{
						parameters:     []SymbolicValue{op.Value.SymbolicValue()},
						parameterNames: []string{"removed-value"},
						result:         Nil,
						visitCheckNode: isNodeAllowedInMigrationHandler,
					},
					Nil,
				)).(Serializable)

			}

			expectedObject.entries[DB_MIGRATION__DELETIONS_PROP_NAME] = dict
		}

		if len(inclusions) > 0 {
			dict := &Dictionary{
				entries: map[string]Serializable{},
				keys:    map[string]Serializable{},
			}

			for _, op := range inclusions {
				pathPattern := "%" + op.PseudoPath
				var entryValue Serializable = &InoxFunction{
					parameters:     []SymbolicValue{ANY},
					parameterNames: []string{"previous-value"},
					result:         op.Value.SymbolicValue(),
					visitCheckNode: isNodeAllowedInMigrationHandler,
				}

				capable, ok := op.Value.(MigrationInitialValueCapablePattern)
				if ok {
					acceptedInitialValue, ok := capable.MigrationInitialValue()
					if ok {
						entryValue = AsSerializable(joinValues([]SymbolicValue{entryValue, acceptedInitialValue})).(Serializable)
					}
				}

				dict.entries[pathPattern] = entryValue
			}

			expectedObject.entries[DB_MIGRATION__INCLUSIONS_PROP_NAME] = dict
		}

		if len(initializations) > 0 {
			dict := &Dictionary{
				entries: map[string]Serializable{},
				keys:    map[string]Serializable{},
			}

			for _, op := range initializations {
				pathPattern := "%" + op.PseudoPath
				value := op.Value.SymbolicValue()

				var entryValue Serializable = &InoxFunction{
					parameters:     []SymbolicValue{joinValues([]SymbolicValue{value, Nil})},
					parameterNames: []string{"previous-value"},
					result:         value,
					visitCheckNode: isNodeAllowedInMigrationHandler,
				}

				capable, ok := op.Value.(MigrationInitialValueCapablePattern)
				if ok {
					acceptedInitialValue, ok := capable.MigrationInitialValue()
					if ok {
						entryValue = AsSerializable(joinValues([]SymbolicValue{entryValue, acceptedInitialValue})).(Serializable)
					}
				}

				dict.entries[pathPattern] = entryValue
			}

			expectedObject.entries[DB_MIGRATION__INITIALIZATIONS_PROP_NAME] = dict
		}

		ctx.SetSymbolicGoFunctionParameters(&[]SymbolicValue{ANY_OBJECT_PATTERN, expectedObject}, []string{"new-schema", "migrations"})
		return
	}
}

func (db *DatabaseIL) Close(*Context) *Error {
	return nil
}

func (db *DatabaseIL) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%database")))
}
