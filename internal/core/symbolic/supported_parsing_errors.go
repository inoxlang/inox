package symbolic

import "github.com/inoxlang/inox/internal/parse"

var (
	SUPPORTED_PARSING_ERRORS = []parse.ParsingErrorKind{
		parse.MissingExpr,
		parse.UnterminatedMemberExpr, parse.UnterminatedDoubleColonExpr,
		parse.UnterminatedOptionExpr,

		parse.UnterminatedExtendStmt,
		parse.UnterminatedStructDefinition,

		parse.UnterminatedSwitchStmt,
		parse.UnterminatedSwitchExpr,
		parse.UnterminatedMatchStmt,
		parse.UnterminatedMatchExpr,

		parse.MissingBlock, parse.MissingFnBody,
		parse.MissingEqualsSignInDeclaration,
		parse.MissingObjectPropertyValue,
		parse.MissingObjectPatternProperty,
		parse.ExtractionExpressionExpected,
	}
)
