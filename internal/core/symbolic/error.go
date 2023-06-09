package symbolic

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	parse "github.com/inoxlang/inox/internal/parse"
)

const (
	CALLEE_HAS_NODE_BUT_NOT_DEFINED                                               = "callee is a node but has not defined type"
	CANNOT_CALL_GO_FUNC_NO_CONCRETE_VALUE                                         = "cannot call go function with no concrete value"
	SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS                              = "spread arguments are not supported when calling non-variadic functions"
	STR_TEMPL_LITS_WITH_INTERP_SHOULD_BE_PRECEDED_BY_PATTERN_WICH_NAME_HAS_PREFIX = "string template literals with interpolations should be preceded by a pattern which name has a prefix"
	FUNCS_CALLED_RECU_SHOULD_HAVE_RET_TYPE                                        = "functions called recursively should have a return type"
	CANNOT_SPREAD_OBJ_PATTERN_THAT_MATCHES_ANY_OBJECT                             = "cannot spread an object pattern that matches any object"
	CANNOT_SPREAD_OBJ_PATTERN_THAT_IS_INEXACT                                     = "cannot spread an object pattern that is inexact"
	MISSING_RETURN_IN_FUNCTION                                                    = "missing return in function"
	MISSING_UNCONDITIONAL_RETURN_IN_FUNCTION                                      = "missing unconditional return in function"
	MISSING_RETURN_IN_FUNCTION_PATT                                               = "missing return in function pattern"
	INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT                                    = "invalid assignment: left hand side is not an integer"
	INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT                                    = "invalid assignment: right hand side is not an integer"
	INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE    = "invalid assignment: non-serializable values are not allowed as properties of serializable values"
	PROP_SPREAD_IN_REC_NOT_SUPP_YET                                               = "property spread not supported in record yet"
	CONSTRAINTS_INIT_BLOCK_EXPLANATION                                            = "invalid statement or expression in constraints' initialization block"

	NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE = "non-serializable values are not allowed as initial values for properties of serializables"
	NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE       = "non-serializable values are not allowed as elements of serializables"

	INVALID_KEY_IN_COMPUTE_EXPRESSION_ONLY_SIMPLE_VALUE_ARE_SUPPORTED = "invalid key in compute expression: only simple values are supported"

	CANNOT_CREATE_OPTIONAL_PATTERN_WITH_PATT_MATCHING_NIL           = "cannot create optional pattern with pattern matching nil"
	KEY_VAR_SHOULD_BE_PROVIDED_ONLY_WHEN_ITERATING_OVER_AN_ITERABLE = "a key variable should be provided only when iterating over an iterable"

	ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE            = "elements of a tuple should be immutable"
	UNSUPPORTED_PARAM_TYPE_FOR_RUNTIME_TYPECHECK = "unsupported parameter type for runtime typecheck"

	CONCATENATION_SUPPORTED_TYPES_EXPLANATION = "only string, bytes & tuple concatenations are supported for now"
	SPREAD_ELEMENT_IS_NOT_ITERABLE            = "spread element is not iterabe"

	NESTED_RECURSIVE_FUNCTION_DECLARATION = "nested recursive function declarations are not allowed"

	NAMESPACE_APPLIED_TO_XML_ELEMENT_SHOUD_BE_A_RECORD           = "namespace applied to xml element should be a record"
	MISSING_FACTORY_IN_NAMESPACE_APPLIED_TO_XML_ELEMENT          = "namespace applied to xml has not a " + FROM_XML_FACTORY_NAME + " property"
	FROM_XML_FACTORY_IS_NOT_A_GO_FUNCTION                        = "factory ." + FROM_XML_FACTORY_NAME + " is not a Go function"
	FROM_XML_FACTORY_SHOULD_NOT_BE_A_SHARED_FUNCTION             = "factory ." + FROM_XML_FACTORY_NAME + " should not be a shared function"
	FROM_XML_FACTORY_SHOULD_HAVE_AT_LEAST_ONE_NON_VARIADIC_PARAM = "factory ." + FROM_XML_FACTORY_NAME + " should have at least one non variadic parameter"

	IMPORTED_MOD_PATH_MUST_END_WITH_IX = "imported module's path must end with '.ix'"

	//permissions
	POSSIBLE_MISSING_PERM_TO_CREATE_A_COROUTINE = "missing permission to create a coroutine"
)

var (
	ErrNotImplementedYet = errors.New("not implemented yet")
	ErrUnreachable       = errors.New("unreachable")

	_ parse.LocatedError = SymbolicEvaluationError{}
)

type SymbolicEvaluationError struct {
	Message        string
	LocatedMessage string
	Location       parse.SourcePositionStack
}

func (err SymbolicEvaluationError) Error() string {
	return err.LocatedMessage
}

func (err SymbolicEvaluationError) MessageWithoutLocation() string {
	return err.Message
}

func (err SymbolicEvaluationError) LocationStack() parse.SourcePositionStack {
	return err.Location
}

func fmtCannotCallNode(node parse.Node) string {
	return fmt.Sprintf("cannot call node of type %T", node)
}

func fmtCannotCall(v SymbolicValue) string {
	return fmt.Sprintf("cannot call %s", Stringify(v))
}

func fmtInvalidBinaryOperator(operator parse.BinaryOperator) string {
	return "invalid binary operator " + operator.String()
}

func fmtOperandOfBoolNegateShouldBeBool(v SymbolicValue) string {
	return fmt.Sprintf("operand of ! should should be a boolean but is a %s", Stringify(v))
}

func fmtOperandOfNumberNegateShouldBeIntOrFloat(v SymbolicValue) string {
	return fmt.Sprintf("operand of '-' should should be an integer or float but is a %s", Stringify(v))
}

func fmtLeftOperandOfBinaryShouldBe(operator parse.BinaryOperator, expectedType string, actual string) string {
	return fmt.Sprintf("left operand of binary %s should be a(n) %s but is %s", operator.String(), expectedType, actual)
}

func fmtRightOperandOfBinaryShouldBe(operator parse.BinaryOperator, expectedType string, actual string) string {
	return fmt.Sprintf("right operand of binary %s should be a(n) %s but is %s", operator.String(), expectedType, actual)
}

func fmtInvalidBinExprCannnotCheckNonObjectHasKey(v SymbolicValue) string {
	return fmt.Sprintf("invalid binary expression: cannot check if non-object has a key: %T", v)
}

func fmtValuesOfRecordShouldBeImmutablePropHasMutable(k string) string {
	return fmt.Sprintf("invalid value for key '%s', values of a record should be immutable", k)
}

func fmtIfStmtTestNotBoolBut(test SymbolicValue) string {
	return fmt.Sprintf("if statement test is not a boolean but a %T", test)
}

func fmtIfExprTestNotBoolBut(test SymbolicValue) string {
	return fmt.Sprintf("if expression test is not a boolean but a %T", test)
}

func fmtNotAssignableToVarOftype(a SymbolicValue, b Pattern) string {
	return fmt.Sprintf("a(n) %s is not assignable to a variable of type %s", Stringify(a), Stringify(b.SymbolicValue()))
}

func fmtNotAssignableToPropOfType(a SymbolicValue, b Pattern) string {
	return fmt.Sprintf("a(n) %s is not assignable to a property of type %s", Stringify(a), Stringify(b.SymbolicValue()))
}

func fmtNotAssignableToPropOfValue(a SymbolicValue, b SymbolicValue) string {
	return fmt.Sprintf("a(n) %s is not assignable to a property of type %s", Stringify(a), Stringify(b))
}

func fmtUnexpectedElemInListAnnotated(e SymbolicValue, elemType Pattern) string {
	return fmt.Sprintf("unexpected element of type %s in a list of %s (annotation)", Stringify(e), Stringify(elemType.SymbolicValue()))
}

func fmtUnexpectedElemInTupleAnnotated(e SymbolicValue, elemType Pattern) string {
	return fmt.Sprintf("unexpected element of type %s in a tuple of %s (annotation)", Stringify(e), Stringify(elemType.SymbolicValue()))
}

func FmtCannotAssignPropertyOf(v SymbolicValue) string {
	return fmt.Sprintf("cannot assign property of a(n) %s", Stringify(v))
}

func fmtIndexIsNotAnIntButA(v SymbolicValue) string {
	return fmt.Sprintf("index is not an integer but a(n) %s", Stringify(v))
}

func fmtStartIndexIsNotAnIntButA(v SymbolicValue) string {
	return fmt.Sprintf("start index is not an integer but a(n) %s", Stringify(v))
}

func fmtEndIndexIsNotAnIntButA(v SymbolicValue) string {
	return fmt.Sprintf("end index is not an integer but a(n) %s", Stringify(v))
}

func fmtMissingProperty(name string) string {
	return fmt.Sprintf("missing property '%s'", name)
}

func fmtInvalidNumberOfArgs(actual, expected int) string {
	return fmt.Sprintf("invalid number of arguments : %v, %v was expected", actual, expected)
}

func fmtInvalidNumberOfNonSpreadArgs(nonVariadicArgCount, nonVariadicParamCount int) string {
	return fmt.Sprintf("invalid number of non-spread arguments : %v, at least %v were expected", nonVariadicArgCount, nonVariadicParamCount)
}

func FmtInvalidArg(position int, actual, expected SymbolicValue) string {
	return fmt.Sprintf("invalid value for argument at position %d: type is %s, but %s was expected", position, Stringify(actual), Stringify(expected))
}

func fmtInvalidReturnValue(actual, expected SymbolicValue) string {
	return fmt.Sprintf("invalid return value: type is %v, but a value matching %v was expected", Stringify(actual), Stringify(expected))
}

func fmtSeqExpectedButIs(value SymbolicValue) string {
	return fmt.Sprintf("a sequence was expected but value is a(n) %s", Stringify(value))
}

func fmtXisNotIterable(v SymbolicValue) string {
	return fmt.Sprintf("a(n) %s is not iterable", Stringify(v))
}

func fmtXisNotWalkable(v SymbolicValue) string {
	return fmt.Sprintf("a(n) %s is not walkable", Stringify(v))
}

func fmtXisNotIndexable(v SymbolicValue) string {
	return fmt.Sprintf("a(n) %s is not indexable", Stringify(v))
}

func fmtXisNotASequence(v SymbolicValue) string {
	return fmt.Sprintf("a(n) %s is not a sequence", Stringify(v))
}

func fmtXisNotAMutableSequence(v SymbolicValue) string {
	return fmt.Sprintf("a(n) %s is not a mutable sequence", v)
}

func fmtSequenceExpectedButIs(value SymbolicValue) string {
	return fmt.Sprintf("a sequence was expected but value is a(n) %s", Stringify(value))
}

func fmtMutableSequenceExpectedButIs(value SymbolicValue) string {
	return fmt.Sprintf("a mutable sequence was expected but value is a(n) %s", Stringify(value))
}

func fmtPatternIsNotDeclared(name string) string {
	return fmt.Sprintf("pattern %%%s is not declared", name)
}

func fmtPatternNamespaceIsNotDeclared(name string) string {
	return fmt.Sprintf("pattern namespace %%%s. is not declared", name)
}

func fmtVarIsNotDeclared(name string) string {
	return fmt.Sprintf("variable '%s' is not declared", name)
}

func fmtLocalVarIsNotDeclared(name string) string {
	return fmt.Sprintf("local variable '%s' is not declared", name)
}

func fmtGlobalVarIsNotDeclared(name string) string {
	return fmt.Sprintf("global variable '%s' is not declared", name)
}

func fmtAttempToAssignConstantGlobal(name string) string {
	return fmt.Sprintf("attempt to assign constant global '%s'", name)
}

func fmtAssertedValueShouldBeBoolNot(v SymbolicValue) string {
	return fmt.Sprintf("asserted value should be a boolean not a %s", Stringify(v))
}

func fmtGroupPropertyNotRoutineGroup(v SymbolicValue) string {
	return fmt.Sprintf("value of .group should be a routine group, not a(n) %s", Stringify(v))
}

func fmtValueOfVarShouldBeAModuleNode(name string) string {
	return fmt.Sprintf("%s should be a module node", name)
}

func fmtSpreadArgumentShouldBeIterable(v SymbolicValue) string {
	return fmt.Sprintf("a spread argument should be iterable but is a(n) %s", Stringify(v))
}

func fmtCannotInterpolatePatternNamespaceDoesNotExist(name string) string {
	return fmt.Sprintf("cannot interpolate: pattern namespace '%s' does not exist", name)
}

func fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist(name string, namespace string) string {
	return fmt.Sprintf("cannot interpolate: member .%s of pattern namespace '%s' does not exist", name, namespace)
}

func fmtInterpolationIsNotStringlikeOrIntBut(v SymbolicValue) string {
	return fmt.Sprintf("result of interpolation expression should be a string/int but is a(n) %s", Stringify(v))
}

func fmtUntypedInterpolationIsNotStringlikeOrIntBut(v SymbolicValue) string {
	return fmt.Sprintf("result of untyped interpolation expression should be a string/int but is a(n) %s", Stringify(v))
}

func fmtPropOfSymbolicDoesNotExist(name string, v SymbolicValue, suggestion string) string {
	if suggestion != "" {
		suggestion = " maybe you meant ." + suggestion
	}
	return fmt.Sprintf("property .%s does not exist in %s (%T)%s", name, Stringify(v), v, suggestion)
}

func fmtPropertyIsOptionalUseOptionalMembExpr(name string) string {
	return fmt.Sprintf("property .%s is optional, you should use an optional member expression: .?%s", name, name)
}

func fmtPatternSpreadInObjectPatternShouldBeAnObjectPatternNot(v SymbolicValue) string {
	return fmt.Sprintf("a pattern that is a spread in an object pattern should be an object pattern not a(n) %s", Stringify(v))
}

func fmtCannotCreateHostAliasWithA(value SymbolicValue) string {
	return fmt.Sprintf("cannot create a host alias with a value of type %s", Stringify(value))
}

func fmtPatternNamespaceShouldBeInitWithNot(v SymbolicValue) string {
	return fmt.Sprintf("a pattern namespace should be initialized with an object or a record not a(n) %s", Stringify(v))
}

func fmtMethodCyclesDetected(cycles [][]string) string {
	buf := bytes.Buffer{}

	for _, cycle := range cycles {
		buf.WriteString("method cycle detected between: ")
		buf.WriteString(strings.Join(cycle, ", "))
		buf.WriteRune('\n')
	}

	return buf.String()
}

func fmtCannotInitializedMetaProp(key string) string {
	return fmt.Sprintf("cannot initialize metaproperty '%s'", key)
}

func fmtCannotGetDynamicMemberOfValueWithNoProps(v SymbolicValue) string {
	return fmt.Sprintf("cannot get dynamic member of value with no properties: %s", Stringify(v))
}

func FormatErrPropertyDoesNotExist(name string, v SymbolicValue) error {
	return fmt.Errorf("property .%s of value %#v does not exist", name, v)
}

func fmtSynchronizedValueShouldBeASharableValueOrImmutableNot(v SymbolicValue) string {
	return fmt.Sprintf("synchronized value should be a sharable or immutable value not a(n) %s", Stringify(v))
}

func fmtXisNotAGroupMatchingPattern(v SymbolicValue) string {
	return fmt.Sprintf("a(n) %s is not a group matching pattern", v)
}

func fmtSubjectOfLifetimeJobShouldBeObjectPatternNot(v SymbolicValue) string {
	return fmt.Sprintf("the subject pattern of a lifetime job should be an object pattern not an %s", Stringify(v))
}

func fmtSelfShouldMatchLifetimeJobSubjectPattern(p Pattern) string {
	return fmt.Sprintf("self should match subject pattern of lifetime job (%s) ", Stringify(p))
}

func fmtListShouldHaveLengthGreaterOrEqualTo(n int) string {
	return fmt.Sprintf("list should have a length greater or equal to %d", n)
}

func fmtComputedPropNameShouldBeAStringNotA(v SymbolicValue) string {
	return fmt.Sprintf("computed property name should be a string, not a(n) %s", Stringify(v))
}

func fmtUnknownSectionInCoroutineMetadata(name string) string {
	return fmt.Sprintf("unknown section '%s' in coroutine metadata", name)
}

func fmtValueNotStringifiableToQueryParamValue(val SymbolicValue) string {
	return fmt.Sprintf("value of type %s is not stringifiable to a query param value: only strings, integers & booleans are accepted", Stringify(val))
}
