package symbolic

import parse "github.com/inoxlang/inox/internal/parse"

type MigrationOp interface {
	GetPseudoPath() string
}

type MigrationMixin struct {
	PseudoPath string
}

func (m MigrationMixin) GetPseudoPath() string {
	return m.PseudoPath
}

type ReplacementMigrationOp struct {
	Current, Next Pattern
	MigrationMixin
}

type RemovalMigrationOp struct {
	Value Pattern
	MigrationMixin
}

type NillableInitializationMigrationOp struct {
	Value Pattern
	MigrationMixin
}

type InclusionMigrationOp struct {
	Value    Pattern
	Optional bool
	MigrationMixin
}

type MigrationInitialValueCapablePattern interface {
	//MigrationInitialValue returns the initial value accepted by the pattern for initialization.
	MigrationInitialValue() (Serializable, bool)
}

func isNodeAllowedInMigrationHandler(visit visitArgs, globalsAtCreation map[string]SymbolicValue) (parse.TraversalAction, bool, error) {
	switch visit.node.(type) {
	case parse.SimpleValueLiteral, //includes IdentifierLiteral
		*parse.GlobalVariable, *parse.Variable,
		//basic data structures
		*parse.ObjectLiteral, *parse.ObjectProperty, *parse.PropertySpreadElement, *parse.RecordLiteral,
		*parse.ListLiteral, *parse.ElementSpreadElement, *parse.TupleLiteral:
	case *parse.ReturnStatement:
	case *parse.IfStatement:
	case *parse.IfExpression:
	case *parse.BinaryExpression:
		//TODO: prevent expensive operations
	case *parse.Assignment:
	default:
		return parse.Prune, false, nil
	}
	return parse.Continue, true, nil
}
