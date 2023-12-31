package symbolic

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/inoxlang/inox/internal/utils/intconv"
	"github.com/inoxlang/inox/internal/utils/pathutils"
	"golang.org/x/exp/slices"
)

const (
	DB_MIGRATION__DELETIONS_PROP_NAME       = "deletions"
	DB_MIGRATION__INCLUSIONS_PROP_NAME      = "inclusions"
	DB_MIGRATION__REPLACEMENTS_PROP_NAME    = "replacements"
	DB_MIGRATION__INITIALIZATIONS_PROP_NAME = "initializations"
)

var (
	DATABASE_PROPNAMES = []string{"update_schema", "close", "schema"}

	ANY_DATABASE = NewDatabaseIL(DatabaseILParams{Schema: NewAnyObjectPattern(), SchemaUpdateExpected: false})
)

// A DatabaseIL represents a symbolic DatabaseIL.
type DatabaseIL struct {
	UnassignablePropsMixin
	schema               *ObjectPattern
	schemaUpdateExpected bool                    //not used for comparison
	topLevelEntities     map[string]Serializable //created from the schema
	propertyNames        []string
	url                  *URL //can be nil

	//dummy state
	//We do not set it with the converted owner state of the concrete DatabaseIL in order to avoid issues (duplicate symbolic states, ...),
	//however that could cause other issues because .ownerState has no information about the concrete owner state.
	ownerState *State
}

type DatabaseILParams struct {
	Schema               *ObjectPattern
	SchemaUpdateExpected bool
	BaseURL              *URL //optional, should not be set if Host is set
}

func NewDatabaseIL(args DatabaseILParams) *DatabaseIL {
	propertyNames := slices.Clone(DATABASE_PROPNAMES)
	for propName := range args.Schema.entries {
		if utils.SliceContains(DATABASE_PROPNAMES, propName) {
			panic(fmt.Errorf("name collision with inital property name '%s'", propName))
		}
		propertyNames = append(propertyNames, propName)
	}

	db := &DatabaseIL{
		schemaUpdateExpected: args.SchemaUpdateExpected,
		schema:               args.Schema,
		topLevelEntities:     args.Schema.SymbolicValue().(*Object).SerializableEntryMap(),
		propertyNames:        propertyNames,

		url: args.BaseURL,
	}

	chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
		NameString: "pseudo-database-state-module",
		CodeString: "manifest {}",
	}))
	db.ownerState = newSymbolicState(NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil), chunk)

	return db
}

func (db *DatabaseIL) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch other := v.(type) {
	case *DatabaseIL:
		return db.schema.Test(other.schema, state)
	default:
		return false
	}
}

func (*DatabaseIL) WidestOfType() Value {
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

func (db *DatabaseIL) Prop(name string) Value {
	switch name {
	case "schema":
		return db.schema
	}

	entry, ok := db.schema.entries[name]
	if ok {
		topLevelEntity := entry.SymbolicValue().(PotentiallySharable).Share(db.ownerState)

		//set the URL if possible
		if urlHolder, ok := topLevelEntity.(UrlHolder); ok && db.url != nil {
			return urlHolder.WithURL(db.url.WithAdditionalPathSegment(name))
		}
		return topLevelEntity
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

func (db *DatabaseIL) getValueAt(pathOrPattern string) (Serializable, error) {

	if err := ValidatePathOfValueInDatabase(pathOrPattern); err != nil {
		return nil, err
	}

	i := 0
	var result Serializable
	currentPath := "/"

	err := pathutils.ForEachAbsolutePathSegment(pathOrPattern, func(segment string, _, _ int) error {
		nextPath := currentPath
		if currentPath == "/" {
			nextPath += segment
		} else {
			nextPath += "/" + segment
		}

		if i == 0 {
			entity, ok := db.topLevelEntities[segment]
			if !ok {
				return fmt.Errorf("top level entity %q does not exist", segment)
			}
			result = entity
			i++
		} else if collection, ok := result.(Collection); ok {
			result = collection.IteratorElementValue().(Serializable)
		} else {
			indexable, ok := result.(Indexable)
			if ok && '0' <= segment[0] && segment[0] <= '9' {
				index, err := strconv.ParseInt(segment, 10, 32)
				if err != nil {
					goto not_an_index
				}
				intIndex := intconv.MustI64ToI(index)

				if indexable.HasKnownLen() {
					if intIndex >= indexable.KnownLen() || intIndex < 0 {
						return fmt.Errorf("there is no element at %s: "+INDEX_IS_OUT_OF_RANGE, nextPath)
					}

					elem, ok := indexable.elementAt(intIndex).(Serializable)
					if !ok {
						return fmt.Errorf("element at %s is not serializable", nextPath)
					}
					result = elem
				} else {
					elem, ok := indexable.elementAt(intIndex).(Serializable)
					if !ok {
						return fmt.Errorf("elements of %s are not serializable", currentPath)
					}
					result = elem
				}
				return nil
			}

		not_an_index:
			iprops, ok := result.(IProps)
			if !ok {
				return errors.New(fmtValueAtXHasNoProperties(currentPath))
			}
			names := iprops.PropertyNames()
			if !slices.Contains(names, segment) {
				return errors.New(fmtValueAtXDoesNotHavePropX(currentPath, segment))
			}
			result, ok = iprops.Prop(segment).(Serializable)
			if !ok {
				return errors.New(fmtValueAtXIsNotSerializable(nextPath))
			}
			switch result.(type) {
			case *InoxFunction:
				return errors.New(fmtRetrievalOfMethodAtXIsNotAllowed(nextPath))
			}
		}

		currentPath = nextPath
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (db *DatabaseIL) UpdateSchema(ctx *Context, schema *ObjectPattern, additionalArgs ...*Object) {

	if !db.schemaUpdateExpected {
		ctx.AddSymbolicGoFunctionError("no schema update is expected for this database: did you forget to set the expected-schema-update property in the database description ?")
		return
	}

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

	if deeplyMatch(db.schema, schema) {
		ctx.AddSymbolicGoFunctionWarning(CURRENT_DATABASE_SCHEMA_SAME_AS_PASSED)
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

			ctx.SetSymbolicGoFunctionParameters(&[]Value{ANY_OBJECT_PATTERN, expectedObject}, []string{"new-schema", "migrations"})
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
					parameters:     []Value{op.Current.SymbolicValue()},
					parameterNames: []string{"previous-value"},
					result:         op.Next.SymbolicValue(),
					visitCheckNode: isNodeAllowedInMigrationHandler,
				}

				capable, ok := op.Next.(MigrationInitialValueCapablePattern)
				if ok {
					acceptedInitialValue, ok := capable.MigrationInitialValue()
					if ok {
						entryValue = AsSerializableChecked(joinValues([]Value{acceptedInitialValue, entryValue}))
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
						parameters:     []Value{op.Value.SymbolicValue()},
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
					parameters:     []Value{ANY},
					parameterNames: []string{"previous-value"},
					result:         op.Value.SymbolicValue(),
					visitCheckNode: isNodeAllowedInMigrationHandler,
				}

				capable, ok := op.Value.(MigrationInitialValueCapablePattern)
				if ok {
					acceptedInitialValue, ok := capable.MigrationInitialValue()
					if ok {
						entryValue = AsSerializableChecked(joinValues([]Value{acceptedInitialValue, entryValue}))
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
					parameters:     []Value{joinValues([]Value{value, Nil})},
					parameterNames: []string{"previous-value"},
					result:         value,
					visitCheckNode: isNodeAllowedInMigrationHandler,
				}

				capable, ok := op.Value.(MigrationInitialValueCapablePattern)
				if ok {
					acceptedInitialValue, ok := capable.MigrationInitialValue()
					if ok {
						entryValue = AsSerializableChecked(joinValues([]Value{acceptedInitialValue, entryValue}))
					}
				}

				dict.entries[pathPattern] = entryValue
			}

			expectedObject.entries[DB_MIGRATION__INITIALIZATIONS_PROP_NAME] = dict
		}

		ctx.SetSymbolicGoFunctionParameters(&[]Value{ANY_OBJECT_PATTERN, expectedObject}, []string{"new-schema", "migrations"})
		return
	}
}

func (db *DatabaseIL) Close(*Context) *Error {
	return nil
}

func (db *DatabaseIL) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("database ")
	db.schema.PrettyPrint(w, config)
}

func getValueAtURL(u *URL, state *State) (Serializable, error) {
	if !u.hasValue && (u.pattern == nil || !u.pattern.hasValue) {
		return nil, errors.New("URL is not specific enough")
	}

	urlOrPattern := u.value

	if !u.hasValue {
		urlOrPattern = u.pattern.value
	}

	if !strings.HasPrefix(urlOrPattern, "ldb://") {
		return nil, errors.New("only URLs with the scheme ldb:// are supported for now")
	}

	varInfo, ok := state.getGlobal(globalnames.DATABASES)
	if !ok {
		return nil, fmt.Errorf("there is no %q global variable", globalnames.DATABASES)
	}

	dbs, ok := varInfo.value.(*Namespace)
	if !ok {
		return nil, fmt.Errorf("global variable %q should be a namespace", globalnames.DATABASES)
	}

	hostEnd := len(urlOrPattern)
	pathStart := -1
	pathEnd := -1

loop:
	for i := strings.Index(urlOrPattern, "://") + 3; i < len(urlOrPattern); i++ {
		switch urlOrPattern[i] {
		case '/':
			if pathStart < 0 {
				pathStart = i
				hostEnd = i
			}
		case '?', '#':
			if pathStart > 0 {
				pathEnd = i
			} else {
				hostEnd = i
			}
			break loop
		}
	}

	if pathStart > 0 && pathEnd < 0 {
		pathEnd = len(urlOrPattern)
	}

	host := urlOrPattern[:hostEnd]
	parsed, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	hostname := parsed.Hostname()

	entryValue, ok := dbs.entries[hostname]
	if !ok {
		return nil, fmt.Errorf("the %q database does not exist", hostname)
	}

	db, ok := entryValue.(*DatabaseIL)
	if !ok {
		return nil, fmt.Errorf("%s.%s is not a database", globalnames.DATABASES, hostname)
	}

	path := urlOrPattern[pathStart:pathEnd]
	return db.getValueAt(path)
}

// Validates the path (or path pattern) of a value/entity located in a database.
func ValidatePathOfValueInDatabase[T ~string](pathOrPattern T) error {
	if pathOrPattern == "" {
		return errors.New("unexpected empty path")
	}

	if pathOrPattern[0] != '/' {
		return fmt.Errorf("unexpected relative path %q", pathOrPattern)
	}

	if pathOrPattern == "/" {
		return fmt.Errorf(ROOT_PATH_NOT_ALLOWED_REFERS_TO_DB)
	}

	if strings.Contains(string(pathOrPattern), "//") {
		return fmt.Errorf("unexpected empty segment(s) in path %q", pathOrPattern)
	}

	if pathutils.ContainsRelativePathSegments(pathOrPattern) {
		return fmt.Errorf("unexpected relative path segment(s) in path %q", pathOrPattern)
	}

	if pathOrPattern != "/" && pathOrPattern[len(pathOrPattern)-1] == '/' {
		return errors.New(PATH_OF_URL_SHOULD_NOT_HAVE_A_TRAILING_SLASH)
	}

	return nil
}
