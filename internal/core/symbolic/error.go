package symbolic

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/inoxconsts"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	CALLEE_HAS_NODE_BUT_NOT_DEFINED                                               = "callee is a node but has no defined type"
	CANNOT_CALL_GO_FUNC_NO_CONCRETE_VALUE                                         = "cannot call go function with no concrete value"
	SPREAD_ARGS_NOT_SUPPORTED_FOR_NON_VARIADIC_FUNCS                              = "spread arguments are not supported when calling non-variadic functions"
	STR_TEMPL_LITS_WITH_INTERP_SHOULD_BE_PRECEDED_BY_PATTERN_WICH_NAME_HAS_PREFIX = "string template literals with interpolations should be preceded by a pattern which name has a prefix"
	FUNCS_CALLED_RECU_SHOULD_HAVE_RET_TYPE                                        = "functions called recursively should have a return type"
	CANNOT_SPREAD_OBJ_PATTERN_THAT_MATCHES_ANY_OBJECT                             = "cannot spread an object pattern that matches any object"
	CANNOT_SPREAD_REC_PATTERN_THAT_MATCHES_ANY_RECORD                             = "cannot spread an record pattern that matches any record"
	CANNOT_SPREAD_OBJ_PATTERN_THAT_IS_INEXACT                                     = "cannot spread an object pattern that is inexact"
	SPREAD_ELEMENT_SHOULD_BE_A_LIST                                               = "spread element should be a list"
	SPREAD_ELEMENT_SHOULD_BE_A_TUPLE                                              = "spread element should be a tuple"

	CANNOT_ADD_NEW_PROPERTY_TO_AN_EXACT_OBJECT = "cannot add new property to an exact object"

	VALUES_INSIDE_PATTERNS_MUST_BE_SERIALIZABLE = "values inside patterns must be serializable"

	MISSING_RETURN_IN_FUNCTION                                                   = "missing return in function"
	MISSING_UNCONDITIONAL_RETURN_IN_FUNCTION                                     = "missing unconditional return in function"
	MISSING_RETURN_IN_FUNCTION_PATT                                              = "missing return in function pattern"
	INVALID_ASSIGN_INT_OPER_ASSIGN_LHS_NOT_INT                                   = "invalid assignment: left hand side is not an integer"
	INVALID_ASSIGN_INT_OPER_ASSIGN_RHS_NOT_INT                                   = "invalid assignment: right hand side is not an integer"
	INVALID_ASSIGN_NON_SERIALIZABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_SERIALIZABLE   = "invalid assignment: non-serializable values are not allowed as properties of serializable values"
	INVALID_ASSIGN_MUTABLE_NON_WATCHABLE_VALUE_NOT_ALLOWED_AS_PROPS_OF_WATCHABLE = "invalid assignment: mutable values that are not watchable are not allowed as properties of watchable values"
	PROP_SPREAD_IN_REC_NOT_SUPP_YET                                              = "property spread not supported in record yet"
	CONSTRAINTS_INIT_BLOCK_EXPLANATION                                           = "invalid statement or expression in constraints' initialization block"

	NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_SERIALIZABLE   = "non-serializable values are not allowed as initial values for properties of serializables"
	MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_INITIAL_VALUES_OF_WATCHABLE = "mutable values that are not watchable are not allowed as initial values for properties of watchables"
	NON_SERIALIZABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_SERIALIZABLE         = "non-serializable values are not allowed as elements of serializables"
	MUTABLE_NON_WATCHABLE_VALUES_NOT_ALLOWED_AS_ELEMENTS_OF_WATCHABLE       = "mutables values that are not watchable values are not allowed as elements of watchables"

	INDEX_IS_OUT_OF_BOUNDS                        = "index is out of bounds"
	START_INDEX_IS_OUT_OF_BOUNDS                  = "start index is out of bounds"
	END_INDEX_SHOULD_BE_LESS_OR_EQUAL_START_INDEX = "(exclusive) end index should be less or equal to start index"
	IMPOSSIBLE_TO_KNOW_UPDATED_ELEMENT            = "impossible to know updated element"
	IMPOSSIBLE_TO_KNOW_UPDATED_ELEMENTS           = "impossible to know updated elements"

	EXTRACTION_DOES_NOT_SUPPORT_DYNAMIC_VALUES = "extraction does not support dynamic values"

	UPPER_BOUND_OF_QTY_RANGE_LIT_SHOULD_OF_SAME_TYPE_AS_LOWER_BOUND = "the upper bound of a quantity range literal should be of the same type as the lower bound"

	INVALID_KEY_IN_COMPUTE_EXPRESSION_ONLY_SIMPLE_VALUE_ARE_SUPPORTED = "invalid key in compute expression: only simple values are supported"

	CANNOT_CREATE_OPTIONAL_PATTERN_WITH_PATT_MATCHING_NIL           = "cannot create optional pattern with pattern matching nil"
	KEY_VAR_SHOULD_BE_PROVIDED_ONLY_WHEN_ITERATING_OVER_AN_ITERABLE = "a key variable should be provided only when iterating over an iterable"

	ELEMS_OF_TUPLE_SHOUD_BE_IMMUTABLE                  = "elements of a tuple should be immutable"
	ELEM_PATTERNS_OF_TUPLE_SHOUD_MATCH_ONLY_IMMUTABLES = "element patterns of a tuple pattern should match only immutable values"
	UNSUPPORTED_PARAM_TYPE_FOR_RUNTIME_TYPECHECK       = "unsupported parameter type for runtime typecheck"

	CONCATENATION_SUPPORTED_TYPES_EXPLANATION = "only string, bytes & tuple concatenations are supported for now"
	SPREAD_ELEMENT_SHOULD_BE_ITERABLE         = "spread element in concenation should be iterable"

	NESTED_RECURSIVE_FUNCTION_DECLARATION = "nested recursive function declarations are not allowed"
	THIS_EXPR_STMT_SYNTAX_IS_NOT_ALLOWED  = "this expression/statement/syntax element is not allowed in this function"

	NAMESPACE_APPLIED_TO_XML_ELEMENT_SHOUD_BE_A_RECORD           = "namespace applied to xml element should be an Inox namespace such as html"
	MISSING_FACTORY_IN_NAMESPACE_APPLIED_TO_XML_ELEMENT          = "namespace applied to xml has not a " + FROM_XML_FACTORY_NAME + " property"
	FROM_XML_FACTORY_IS_NOT_A_GO_FUNCTION                        = "factory ." + FROM_XML_FACTORY_NAME + " is not a Go function"
	FROM_XML_FACTORY_SHOULD_NOT_BE_A_SHARED_FUNCTION             = "factory ." + FROM_XML_FACTORY_NAME + " should not be a shared function"
	FROM_XML_FACTORY_SHOULD_HAVE_AT_LEAST_ONE_NON_VARIADIC_PARAM = "factory ." + FROM_XML_FACTORY_NAME + " should have at least one non variadic parameter"

	//module import
	IMPORTED_MOD_PATH_MUST_END_WITH_IX = "imported module's path must end with '" + inoxconsts.INOXLANG_FILE_EXTENSION + "'"
	IMPORTED_MODULE_HAS_ERRORS         = "imported module has errors"

	INVALID_MUTATION                               = "invalid mutation"
	PATTERN_IS_NOT_CONVERTIBLE_TO_READONLY_VERSION = "pattern is not convertible to a readonly version"

	//permissions
	POSSIBLE_MISSING_PERM_TO_CREATE_A_LTHREAD = "missing permission to create a lthread"

	META_VAL_OF_LIFETIMEJOB_SHOULD_BE_IMMUTABLE                         = "meta value of lifetime job should be immutable"
	LIFETIME_JOBS_NOT_ALLOWED_IN_READONLY_OBJECTS                       = "lifetime jobs are not allowed in readonly objects"
	PROPERTY_VALUES_OF_READONLY_OBJECTS_SHOULD_BE_READONLY_OR_IMMUTABLE = "property values of readonly objects should be readonly or immutable"

	DOUBLE_COLON_EXPRS_ONLY_SUPPORT_OBJ_LHS_FOR_NOW = //
	"double-colon expressions only support object LHS for now"

	RHS_OF_DOUBLE_COLON_EXPRS_WITH_OBJ_LHS_SHOULD_BE_THE_NAME_OF_A_MUTABLE_NON_SHARABLE_VALUE_PROPERTY = //
	"the right hand side of double-colon expressions with object LHS should be the name of a property with a mutable, non-sharable value."

	USELESS_MUTATION_IN_CLONED_PROP_VALUE = "useless mutation in a cloned property's value"

	MISPLACED_DOUBLE_COLON_EXPR                               = "misplaced double-colon expression"
	MISPLACED_DOUBLE_COLON_EXPR_EXT_METHOD_CAN_ONLY_BE_CALLED = "misplaced double-colon expression: extension methods can only be called"

	OPERANDS_OF_BINARY_RANGE_EXPRS_SHOULD_BE_SERIALIZABLE = "operands of binary range expressions should be serializable"
	VARIABLE_DECL_ANNOTATION_MUST_BE_A_PATTERN            = "variable declaration annotation must be a pattern"

	//extend statement
	EXTENDED_PATTERN_MUST_BE_CONCRETIZABLE_AT_CHECK_TIME = "extended pattern must be concretizable at check time (example of non concretizable pattern: %{a: $runtime-value})"
	ONLY_SERIALIZABLE_VALUE_PATTERNS_ARE_ALLOWED         = "only patterns of serializable values are allowed"
	KEYS_OF_EXT_OBJ_MUST_BE_VALID_INOX_IDENTS            = "keys of the extension object must be valid Inox identifiers (e.g. total, first-name, total_count). Implicit and index-like keys are not allowed"
	META_PROPERTIES_NOT_ALLOWED_IN_EXTENSION_OBJECT      = "metaproperties are not allowed in the extension object"

	THIS_VAL_IS_AN_OPT_LIT_DID_YOU_FORGET_A_SPACE = "this value is an option literal, did you forget a space between '-' and the variable name ?"

	//test suites & cases
	META_VAL_OF_TEST_SUITE_SHOULD_EITHER_BE_A_STRING_OR_A_RECORD    = "the meta value of a test suite should either be a string or a record (e.g. #{name: \"my test suite\"})"
	META_VAL_OF_TEST_CASE_SHOULD_EITHER_BE_A_STRING_OR_A_RECORD    = "the meta value of a test case should either be a string or a record (e.g. #{name: \"my test suite\"})"
	PROGRAM_TESTING_ONLY_SUPPORTED_IN_PROJECTS                      = "program testing is only supported in projects"
	MAIN_DB_SCHEMA_CAN_ONLY_BE_SPECIFIED_WHEN_TESTING_A_PROGRAM     = "main database schema can only be specified when testing a program"
	MAIN_DB_MIGRATIONS_CAN_ONLY_BE_SPECIFIED_WHEN_TESTING_A_PROGRAM = "main database migeations can only be specified when testing a program"
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

func fmtCannotCall(v Value) string {
	return fmt.Sprintf("cannot call %s", Stringify(v))
}

func fmtInvalidBinaryOperator(operator parse.BinaryOperator) string {
	return "invalid binary operator " + operator.String()
}

func fmtOperandOfBoolNegateShouldBeBool(v Value) string {
	return fmt.Sprintf("operand of ! should should be a boolean but is a %s", Stringify(v))
}

func fmtOperandOfNumberNegateShouldBeIntOrFloat(v Value) string {
	return fmt.Sprintf("operand of '-' should should be an integer or float but is a %s", Stringify(v))
}

func fmtLeftOperandOfBinaryShouldBe(operator parse.BinaryOperator, expectedType string, actual string) string {
	return fmt.Sprintf("left operand of binary '%s' should be a(n) %s but is %s", operator.String(), expectedType, actual)
}

func fmtRightOperandOfBinaryShouldBe(operator parse.BinaryOperator, expectedType string, actual string) string {
	return fmt.Sprintf("right operand of binary '%s' should be a(n) %s but is %s", operator.String(), expectedType, actual)
}

func fmtRightOperandOfBinaryShouldBeLikeLeftOperand(operator parse.BinaryOperator, expectedType string, actual string) string {
	return fmt.Sprintf("right operand of binary '%s' should be a(n) %s like the left operand but is %s", operator.String(), expectedType, actual)
}

func fmtInvalidBinExprCannnotCheckNonObjectHasKey(v Value) string {
	return fmt.Sprintf("invalid binary expression: cannot check if non-object has a key: %s", Stringify(v))
}

func fmtValuesOfRecordShouldBeImmutablePropHasMutable(k string) string {
	return fmt.Sprintf("invalid value for key '%s', values of a record should be immutable", k)
}

func fmtEntriesOfRecordPatternShouldMatchOnlyImmutableValues(k string) string {
	return fmt.Sprintf("invalid value for key '%s', entry patterns of a record pattern should match only immutable values", k)
}

func fmtIfStmtTestNotBoolBut(test Value) string {
	return fmt.Sprintf("if statement test is not a boolean but a(n) %s", Stringify(test))
}

func fmtIfExprTestNotBoolBut(test Value) string {
	return fmt.Sprintf("if expression test is not a boolean but a %T", test)
}

func fmtNotAssignableToVarOftype(a Value, b Pattern) string {
	return fmt.Sprintf("a(n) %s is not assignable to a variable of type %s", Stringify(a), Stringify(b.SymbolicValue()))
}

func fmtVarOfTypeCannotBeNarrowedToAn(variable Value, val Value) string {
	return fmt.Sprintf("variable of type %s cannot be narrowed to a(n) %s", Stringify(variable), Stringify(val))
}

func fmtNotAssignableToPropOfType(a Value, b Value) string {
	examples := GetExamples(b, ExampleComputationContext{NonMatchingValue: a})
	examplesString := ""
	if len(examples) > 0 {
		examplesString = fmtExpectedValueExamples(examples)
	}

	return fmt.Sprintf("a(n) %s is not assignable to a property of type %s%s", Stringify(a), Stringify(b), examplesString)
}

func fmtNotAssignableToEntryOfExpectedValue(a Value, b Value) string {
	return fmt.Sprintf("a(n) %s is not assignable to an entry of expected value %s", Stringify(a), Stringify(b))
}

func fmtNotAssignableToElementOfValue(a Value, b Value) string {
	examples := GetExamples(b, ExampleComputationContext{NonMatchingValue: a})
	examplesString := ""
	if len(examples) > 0 {
		examplesString = fmtExpectedValueExamples(examples)
	}

	return fmt.Sprintf("a(n) %s is not assignable to an element of value %s%s", Stringify(a), Stringify(b), examplesString)
}

func fmtSeqOfXNotAssignableToSliceOfTheValue(a Value, b Value) string {
	return fmt.Sprintf("a sequence of %s is not assignable to a slice of value %s, try to have a less specific sequence on the left", Stringify(a), Stringify(b))
}

func fmtHasElementsOfType(val Sequence, typ Value) string {
	return fmt.Sprintf("%s has elements of type: %s", Stringify(val), Stringify(typ))
}

func fmtUnexpectedProperty(name string) string {
	return fmt.Sprintf("unexpected property '%s'", name)
}

func fmtUnexpectedPropertyDidYouMeanElse(name string, suggestion string) string {
	return fmt.Sprintf("unexpected property '%s', did you mean '%s' ?", name, suggestion)
}

func fmtUnexpectedElemInListAnnotated(e Value, elemType Pattern) string {
	expectedElem := elemType.SymbolicValue()
	examples := GetExamples(expectedElem, ExampleComputationContext{NonMatchingValue: e})
	examplesString := ""
	if len(examples) > 0 {
		examplesString = fmtExpectedValueExamples(examples)
	}

	return fmt.Sprintf("unexpected element of type %s in a list of %s (annotated)%s", Stringify(e), Stringify(expectedElem), examplesString)
}

func fmtUnexpectedElemInListofValues(e Value, elemType Value) string {
	examples := GetExamples(elemType, ExampleComputationContext{NonMatchingValue: e})
	examplesString := ""
	if len(examples) > 0 {
		examplesString = fmtExpectedValueExamples(examples)
	}

	return fmt.Sprintf("unexpected element of type %s in a list of %s%s", Stringify(e), Stringify(elemType), examplesString)
}

func fmtUnexpectedElemInTupleAnnotated(e Value, elemType Pattern) string {
	expectedElem := elemType.SymbolicValue()
	examples := GetExamples(expectedElem, ExampleComputationContext{NonMatchingValue: e})
	examplesString := ""
	if len(examples) > 0 {
		examplesString = fmtExpectedValueExamples(examples)
	}

	return fmt.Sprintf("unexpected element of type %s in a tuple of %s (annotated)%s", Stringify(e), Stringify(expectedElem), examplesString)
}

func FmtCannotAssignPropertyOf(v Value) string {
	return fmt.Sprintf("cannot assign property of a(n) %s", Stringify(v))
}

func fmtIndexIsNotAnIntButA(v Value) string {
	return fmt.Sprintf("index is not an integer but a(n) %s", Stringify(v))
}

func fmtStartIndexIsNotAnIntButA(v Value) string {
	return fmt.Sprintf("start index is not an integer but a(n) %s", Stringify(v))
}

func fmtEndIndexIsNotAnIntButA(v Value) string {
	return fmt.Sprintf("end index is not an integer but a(n) %s", Stringify(v))
}

func fmtMissingProperty(name string) string {
	return fmt.Sprintf("missing property '%s'", name)
}

func fmtInvalidNumberOfArgs(actual, expected int) string {
	return fmt.Sprintf("invalid number of arguments: %v, %v were expected", actual, expected)
}

func fmtTooManyArgs(actual, expected int) string {
	return fmt.Sprintf("too many arguments: %v, %v were expected", actual, expected)
}

func fmtNotEnoughArgs(actual, expected int) string {
	return fmt.Sprintf("not enough arguments: %v, %v were expected", actual, expected)
}

func fmtNotEnoughArgsAtLeastMandatoryMax(actual, mandatory int, max int) string {
	return fmt.Sprintf("not enough arguments: %v, at least %v were expected (max %v)", actual, mandatory, max)
}

func fmtInvalidNumberOfNonSpreadArgs(nonVariadicArgCount, nonVariadicParamCount int) string {
	return fmt.Sprintf("invalid number of non-spread arguments: %v, at least %v were expected", nonVariadicArgCount, nonVariadicParamCount)
}

func fmtInvalidNumberOfNonArgsAtLeastMandatoryMax(actual, mandatory int, max int) string {
	return fmt.Sprintf("invalid number of non-spread arguments: %v, at least %v were expected (max %v)", actual, mandatory, max)
}

func FmtInvalidArg(position int, actual, expected Value) string {
	return fmt.Sprintf("invalid value for argument at position %d: type is %s, but %s was expected", position, Stringify(actual), Stringify(expected))
}

func fmtInvalidReturnValue(actual, expected Value) string {
	return fmt.Sprintf("invalid return value: type is %v, but a value matching %v was expected", Stringify(actual), Stringify(expected))
}

func fmtSeqExpectedButIs(value Value) string {
	return fmt.Sprintf("a sequence was expected but value is a(n) %s", Stringify(value))
}

func fmtXisNotIterable(v Value) string {
	return fmt.Sprintf("a(n) %s is not iterable", Stringify(v))
}

func fmtXisNotWalkable(v Value) string {
	return fmt.Sprintf("a(n) %s is not walkable", Stringify(v))
}

func fmtXisNotIndexable(v Value) string {
	return fmt.Sprintf("a(n) %s is not indexable", Stringify(v))
}

func fmtXisNotASequence(v Value) string {
	return fmt.Sprintf("a(n) %s is not a sequence", Stringify(v))
}

func fmtXisNotAMutableSequence(v Value) string {
	return fmt.Sprintf("a(n) %s is not a mutable sequence", Stringify(v))
}

func fmtSequenceExpectedButIs(value Value) string {
	return fmt.Sprintf("a sequence was expected but value is a(n) %s", Stringify(value))
}

func fmtMutableSequenceExpectedButIs(value Value) string {
	return fmt.Sprintf("a mutable sequence was expected but value is a(n) %s", Stringify(value))
}

func fmtRHSSequenceShouldHaveLenOf(length int) string {
	return fmt.Sprintf("sequence on the right hand side should have a length of %d", length)
}

func fmtPatternIsNotDeclared(name string) string {
	return fmt.Sprintf("pattern %%%s is not declared", name)
}

func fmtPatternIsNotDeclaredYouProbablyMeant(name string, suggestion string) string {
	return fmt.Sprintf("pattern %%%s is not declared; you probably meant `%%%s`", name, suggestion)
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

func fmtAssertedValueShouldBeBoolNot(v Value) string {
	return fmt.Sprintf("asserted value should be a boolean not a %s", Stringify(v))
}

func fmtGroupPropertyNotLThreadGroup(v Value) string {
	return fmt.Sprintf("value of .group should be a lthread group, not a(n) %s", Stringify(v))
}

func fmtValueOfVarShouldBeAModuleNode(name string) string {
	return fmt.Sprintf("%s should be a module node", name)
}

func fmtSpreadArgumentShouldBeIterable(v Value) string {
	return fmt.Sprintf("a spread argument should be iterable but is a(n) %s", Stringify(v))
}

func fmtDidYouMeanDollarName(name string, doubleDollar bool) string {
	if doubleDollar {
		name = "$$" + name
	} else {
		name = "$" + name
	}

	return fmt.Sprintf(
		"did you mean `%s` ?"+
			" In a call with the CLI syntax, identifiers such as `a` are evaluated to identifier values (#a)."+
			" Local variables must be prefixed with a dollar: $mylocal. Global variables must be prefixed "+
			"with two dollars: $$myglobal.",
		name)
}

func fmtCannotInterpolatePatternNamespaceDoesNotExist(name string) string {
	return fmt.Sprintf("cannot interpolate: pattern namespace '%s' does not exist", name)
}

func fmtCannotInterpolateMemberOfPatternNamespaceDoesNotExist(name string, namespace string) string {
	return fmt.Sprintf("cannot interpolate: member .%s of pattern namespace '%s' does not exist", name, namespace)
}

func fmtInterpolationIsNotStringlikeOrIntBut(v Value) string {
	return fmt.Sprintf("result of interpolation expression should be a string/int but is a(n) %s", Stringify(v))
}

func fmtUntypedInterpolationIsNotStringlikeOrIntBut(v Value) string {
	return fmt.Sprintf("result of untyped interpolation expression should be a string/int but is a(n) %s", Stringify(v))
}

func fmtPropOfDoesNotExist(name string, v Value, suggestion string) string {
	if suggestion != "" {
		suggestion = " maybe you meant ." + suggestion
	}
	return fmt.Sprintf("property .%s does not exist in %s (%T)%s", name, Stringify(v), v, suggestion)
}

func fmtPropertyIsOptionalUseOptionalMembExpr(name string) string {
	return fmt.Sprintf("property .%s is optional, you should use an optional member expression: .?%s", name, name)
}

func fmtExtensionsDoNotProvideTheXProp(name string, suggestion string) string {
	if suggestion != "" {
		suggestion = " maybe you meant ." + suggestion
	}
	return fmt.Sprintf("extensions do not provide a(n) '%s' property%s", name, suggestion)
}

func fmtPatternSpreadInObjectPatternShouldBeAnObjectPatternNot(v Value) string {
	return fmt.Sprintf("a pattern that is a spread in an object pattern should be an object pattern not a(n) %s", Stringify(v))
}

func fmtPatternSpreadInRecordPatternShouldBeAnRecordPatternNot(v Value) string {
	return fmt.Sprintf("a pattern that is a spread in an record pattern should be an record pattern not a(n) %s", Stringify(v))
}

func fmtPropertyShouldNotBePresentInSeveralSpreadPatterns(name string) string {
	return fmt.Sprintf("property '%s' should not be present in several spread patterns", name)
}

func fmtCannotCreateHostAliasWithA(value Value) string {
	return fmt.Sprintf("cannot create a host alias with a value of type %s", Stringify(value))
}

func fmtPatternNamespaceShouldBeInitWithNot(v Value) string {
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

func fmtCannotGetDynamicMemberOfValueWithNoProps(v Value) string {
	return fmt.Sprintf("cannot get dynamic member of value with no properties: %s", Stringify(v))
}

func fmtValueHasNoProperties(value Value) string {
	return fmt.Sprintf("value has no properties: %s", Stringify(value))
}

func FormatErrPropertyDoesNotExist(name string, v Value) error {
	return fmt.Errorf("property .%s of value %#v does not exist", name, v)
}

func fmtSynchronizedValueShouldBeASharableValueOrImmutableNot(v Value) string {
	return fmt.Sprintf("synchronized value should be a sharable or immutable value not a(n) %s", Stringify(v))
}

func fmtXisNotAGroupMatchingPattern(v Value) string {
	return fmt.Sprintf("a(n) %s is not a group matching pattern", Stringify(v))
}

func fmtSubjectOfLifetimeJobShouldBeObjectPatternNot(v Value) string {
	return fmt.Sprintf("the subject pattern of a lifetime job should be an object pattern not an %s", Stringify(v))
}

func fmtSelfShouldMatchLifetimeJobSubjectPattern(p Pattern) string {
	return fmt.Sprintf("self should match subject pattern of lifetime job (%s) ", Stringify(p))
}

func fmtListShouldHaveLengthGreaterOrEqualTo(n int) string {
	return fmt.Sprintf("list should have a length greater or equal to %d", n)
}

func fmtComputedPropNameShouldBeAStringNotA(v Value) string {
	return fmt.Sprintf("computed property name should be a string, not a(n) %s", Stringify(v))
}

func fmtUnknownSectionInLThreadMetadata(name string) string {
	return fmt.Sprintf("unknown section '%s' in lthread metadata", name)
}

func fmtValueNotStringifiableToQueryParamValue(val Value) string {
	return fmt.Sprintf("value of type %s is not stringifiable to a query param value: only strings, integers & booleans are accepted", Stringify(val))
}

func fmtVal1Val2HaveNoOverlap(val1, val2 Value) string {
	return fmt.Sprintf("%s and %s have no overlap", Stringify(val1), Stringify(val2))
}

func fmtStringConcatInvalidElementOfType(v Value) string {
	return fmt.Sprintf("string concatenation: invalid element of type %s", Stringify(v))
}

func fmtDidYouForgetLeadingPercent(path string) string {
	return fmt.Sprintf("did you forget a leading `%%` symbol ? `%s` is a path, you probably meant the following path pattern: %%%s", path, path)
}

func fmtExtendedValueAlreadyHasAnXProperty(name string) string {
	return fmt.Sprintf("extended value already has a(n) %q property", name)
}

func FmtPropertyPatternError(name string, err error) error {
	return fmt.Errorf("property pattern .%s: %w", name, err)
}

func FmtPropertyError(name string, err error) error {
	return fmt.Errorf("property .%s: %w", name, err)
}

func FmtElementError(index int, err error) error {
	return fmt.Errorf("element at index %d: %w", index, err)
}

func FmtGeneralElementError(err error) error {
	return fmt.Errorf("general element: %w", err)
}

func fmtExpectedValueExamples(examples []MatchingValueExample) string {
	return "; expected value examples: \n" + strings.Join(utils.MapSlice(examples, func(e MatchingValueExample) string {
		return "• " + Stringify(e.Value)
	}), "\n")
}

func fmtUselessMutationInClonedPropValue(elementName string) string {
	return fmt.Sprintf("%s, you should use a double-colon expression (<object>::%s) to retrieve the actual property's value",
		USELESS_MUTATION_IN_CLONED_PROP_VALUE, elementName)
}

func fmtNotRegularFile(path string) string {
	return fmt.Sprintf("%q is not a regular file", path)
}
