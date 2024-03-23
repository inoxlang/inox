package symbolic

import (
	"reflect"

	"github.com/inoxlang/inox/internal/parse"
)

var (
	CTX_PTR_TYPE                         = reflect.TypeOf((*Context)(nil))
	ERROR_TYPE                           = reflect.TypeOf((*Error)(nil))
	SYMBOLIC_VALUE_INTERFACE_TYPE        = reflect.TypeOf((*Value)(nil)).Elem()
	SERIALIZABLE_INTERFACE_TYPE          = reflect.TypeOf((*Serializable)(nil)).Elem()
	ITERABLE_INTERFACE_TYPE              = reflect.TypeOf((*Iterable)(nil)).Elem()
	SERIALIZABLE_ITERABLE_INTERFACE_TYPE = reflect.TypeOf((*SerializableIterable)(nil)).Elem()
	INDEXABLE_INTERFACE_TYPE             = reflect.TypeOf((*Indexable)(nil)).Elem()
	SEQUENCE_INTERFACE_TYPE              = reflect.TypeOf((*Sequence)(nil)).Elem()
	MUTABLE_SEQUENCE_INTERFACE_TYPE      = reflect.TypeOf((*MutableSequence)(nil)).Elem()
	INTEGRAL_INTERFACE_TYPE              = reflect.TypeOf((*Integral)(nil)).Elem()
	WRITABLE_INTERFACE_TYPE              = reflect.TypeOf((*Writable)(nil)).Elem()
	STRLIKE_INTERFACE_TYPE               = reflect.TypeOf((*StringLike)(nil)).Elem()
	BYTESLIKE_INTERFACE_TYPE             = reflect.TypeOf((*BytesLike)(nil)).Elem()

	IPROPS_INTERFACE_TYPE              = reflect.TypeOf((*IProps)(nil)).Elem()
	PROTOCOL_CLIENT_INTERFACE_TYPE     = reflect.TypeOf((*ProtocolClient)(nil)).Elem()
	READABLE_INTERFACE_TYPE            = reflect.TypeOf((*Readable)(nil)).Elem()
	PATTERN_INTERFACE_TYPE             = reflect.TypeOf((*Pattern)(nil)).Elem()
	RESOURCE_NAME_INTERFACE_TYPE       = reflect.TypeOf((*ResourceName)(nil)).Elem()
	VALUE_RECEIVER_INTERFACE_TYPE      = reflect.TypeOf((*MessageReceiver)(nil)).Elem()
	STREAMABLE_INTERFACE_TYPE          = reflect.TypeOf((*StreamSource)(nil)).Elem()
	WATCHABLE_INTERFACE_TYPE           = reflect.TypeOf((*Watchable)(nil)).Elem()
	STR_PATTERN_ELEMENT_INTERFACE_TYPE = reflect.TypeOf((*StringPattern)(nil)).Elem()
	FORMAT_INTERFACE_TYPE              = reflect.TypeOf((*Format)(nil)).Elem()
	IN_MEM_SNAPSHOTABLE                = reflect.TypeOf((*InMemorySnapshotable)(nil)).Elem()
	VALUEPATH_INTERFACE_TYPE           = reflect.TypeOf((*ValuePath)(nil)).Elem()

	OPTIONAL_PARAM_TYPE = reflect.TypeOf((*optionalParam)(nil)).Elem()

	ANY_READABLE = &AnyReadable{}
	ANY_READER   = &Reader{}

	SUPPORTED_PARSING_ERRORS = []parse.ParsingErrorKind{
		parse.UnterminatedMemberExpr, parse.UnterminatedDoubleColonExpr,
		parse.UnterminatedExtendStmt,
		parse.UnterminatedStructDefinition,
		parse.MissingBlock, parse.MissingFnBody,
		parse.MissingEqualsSignInDeclaration,
		parse.MissingObjectPropertyValue,
		parse.MissingObjectPatternProperty,
		parse.ExtractionExpressionExpected,
	}
)
