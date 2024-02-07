package symbolic

import (
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/parse"
)

var (
	_ = []MigrationInitialValueCapablePattern{
		(*TypePattern)(nil),
		(*ObjectPattern)(nil), (*RecordPattern)(nil), (*ListPattern)(nil), (*TuplePattern)(nil),
		(*ExactValuePattern)(nil), (*ExactStringPattern)(nil),
	}
)

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

func isNodeAllowedInMigrationHandler(visit visitArgs, globalsAtCreation map[string]Value) (parse.TraversalAction, bool, error) {
	switch n := visit.node.(type) {
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
	case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceIdentifierLiteral, *parse.PatternNamespaceMemberExpression:
	case *parse.CallExpression:
		ident, ok := n.Callee.(*parse.IdentifierLiteral)

		if !ok {
			return parse.Prune, false, nil
		}

		switch ident.Name {
		//conversion functions
		case globalnames.TOINT_FN, globalnames.TOFLOAT_FN,
			globalnames.TOSTRING_FN, globalnames.TOSTR_FN, globalnames.TORUNE_FN,
			globalnames.TOBYTE_FN, globalnames.TOBYTECOUNT:
		//other functions
		case globalnames.RAND_FN:
		default:
			return parse.Prune, false, nil
		}
	default:
		return parse.Prune, false, nil
	}
	return parse.ContinueTraversal, true, nil
}
