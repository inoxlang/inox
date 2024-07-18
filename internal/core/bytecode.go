package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
)

// A Bytecode contains the constants and a reference to a *CompiledFunction.
// The bytecode instructions are in the *CompiledFunction.
type Bytecode struct {
	module    *Module
	constants []Value
	main      *CompiledFunction
}

// Constants returns the constants used during bytecode interpretation, the slice should not be modified.
func (b *Bytecode) Constants() []Value {
	return b.constants
}

func (b *Bytecode) FormatInstructions(ctx *Context, leftPadding string) []string {
	return FormatInstructions(ctx, b.main.Instructions, 0, leftPadding, b.constants)
}

// FormatConstants returns a human readable representation of compiled constants.
func (b *Bytecode) FormatConstants(ctx *Context, leftPadding string) (output []string) {

	for cidx, cn := range b.constants {
		switch cn := cn.(type) {
		case *InoxFunction:
			output = append(output, fmt.Sprintf("%s[% 3d] (Compiled Function|%p)", leftPadding, cidx, &cn))
			for _, l := range FormatInstructions(ctx, cn.compiledFunction.Instructions, 0, leftPadding, nil) {
				output = append(output, fmt.Sprintf("%s     %s", leftPadding, l))
			}
		case *Bytecode:
			output = append(output, fmt.Sprintf("     %s", cn.Format(ctx, leftPadding+"    ")))
		default:
			repr := Stringify(cn, nil)
			output = append(output, fmt.Sprintf("%s[% 3d] %s", leftPadding, cidx, repr))
		}
	}
	return
}

// Fomat returnsa a human readable representations of the bytecode.
func (b *Bytecode) Format(ctx *Context, leftPadding string) string {
	s := fmt.Sprintf("compiled constants:\n%s", strings.Join(b.FormatConstants(ctx, leftPadding), "\n"))
	s += fmt.Sprintf("\ncompiled instructions:\n%s\n", strings.Join(b.FormatInstructions(ctx, leftPadding), "\n"))
	return s
}

// A CompiledFunction contains the bytecode instructions of a module or a compiled Inox function.
// The compilation of a module produces a *CompiledFunction that is the "main" function.
type CompiledFunction struct {
	ParamCount   int
	IsVariadic   bool
	LocalCount   int // includes parameters
	Instructions []byte
	SourceMap    map[int]instructionSourcePosition
	Bytecode     *Bytecode //bytecode containing the function

	SourceNodeSpan parse.NodeSpan
	IncludedChunk  *parse.ParsedChunkSource //set if the function is defined in an included chunk
}

// GetSourcePositionRange returns the position in source code of the instruction at the ip address,
// several subsequent instructions can have the same position.
func (fn *CompiledFunction) GetSourcePositionRange(ip int) sourcecode.PositionRange {
	info := fn.SourceMap[ip]
	if info.chunk == nil {
		return sourcecode.PositionRange{
			SourceName:  "??",
			StartLine:   1,
			StartColumn: 1,
			EndLine:     1,
			EndColumn:   2,
			Span:        parse.NodeSpan{Start: 0, End: 1},
		}
	}
	return info.chunk.GetSourcePosition(info.span)
}

type instructionSourcePosition struct {
	chunk *parse.ParsedChunkSource
	span  parse.NodeSpan
}

type InstructionCallbackFn = func(instr []byte, op Opcode, operands []int, constantIndexOperandIndex []int, constants []Value, i int) ([]byte, error)

// MapInstructions iterates instructions and calls callbackFn for each instruction.
func MapInstructions(b []byte, constants []Value, callbackFn InstructionCallbackFn) ([]byte, error) {
	i := 0

	var newInstructions []byte

	for i < len(b) {
		op := Opcode(b[i])
		numOperands := OpcodeOperands[b[i]]
		operands, read := ReadOperands(numOperands, b[i+1:])

		var referredConstants []Value
		var constantIndexOperandIndexes []int

		if len(constants) != 0 {
			for j, operand := range operands {
				if OpcodeConstantIndexes[b[i]][j] {
					if numOperands[j] != 2 {
						return nil, errors.New("index of constant should have a width of 2, opcode: " + OpcodeNames[op])
					}
					referredConstants = append(referredConstants, constants[operand])
					constantIndexOperandIndexes = append(constantIndexOperandIndexes, j)
				}
			}
		}

		instruction := b[i : i+1+read]
		if instr, err := callbackFn(instruction, op, operands, constantIndexOperandIndexes, referredConstants, i); err != nil {
			return nil, err
		} else {
			newInstructions = append(newInstructions, instr...)
		}

		i += 1 + read
	}

	return newInstructions, nil
}

// Opcode represents a single byte operation code.
type Opcode = byte

// opcodes
const (
	OpPushConstant Opcode = iota
	OpPop
	OpCopyTop
	OpSwap
	OpMoveThirdTop //move third element at the top of the stack & shift the two others
	OpPushTrue
	OpPushFalse
	OpEqual
	OpNotEqual
	OpIs
	OpIsNot
	OpMinus
	OpBooleanNot
	OpMatch
	OpGroupMatch
	OpIn
	OpSubstrOf
	OpKeyOf
	OpUrlOf
	OpDoSetDifference
	OpNilCoalesce
	OpJumpIfFalse
	OpAndJump // Logical AND jump
	OpOrJump  // Logical OR jump
	OpJump
	OpPopJumpIfTestDisabled
	OpPushNil
	OpCreateList
	OpCreateListDynLen
	OpCreateKeyList
	OpCreateTuple
	OpCreateObject
	OpCreateRecord
	OpCreateDict
	OpCreateMapping
	OpCreateTreedata
	OpCreateTreedataHiearchyEntry
	OpCreateOrderedPair
	OpCreateModuleArgs
	OpSpreadObject
	OpExtractProps
	OpSpreadList
	OpSpreadTuple
	OpAppend
	OpCreateListPattern
	OpCreateTuplePattern
	OpCreateObjectPattern
	OpCreateRecordPattern
	OpCreateOptionPattern
	OpCreateUnionPattern
	OpCreateStringUnionPattern
	OpCreateRepeatedPatternElement
	OpCreateSequenceStringPattern
	OpCreatePatternNamespace
	OpCreateOptionalPattern
	OpToPattern
	OpToBool
	OpCreateString
	OpCreateOption
	OpCreatePath
	OpCreatePathPattern
	OpCreateURL
	OpCreateHost
	OpCreateRuneRange
	OpCreateIntRange
	OpCreateFloatRange
	OpCreateUpperBoundRange
	OpCreateTestSuite
	OpCreateTestCase
	OpAddTestSuiteResult
	OpAddTestCaseResult //add test case result if in test suite
	OpCreateXMLelem
	OpCreateAddTypeExtension
	OpSpreadObjectPattern
	OpSpreadRecordPattern
	BindCapturedLocals
	OpCall
	OpReturn
	OpCallFromXMLFactory
	OpYield
	OpCoyield
	OpCallPattern
	OpDropPerms
	OpSpawnLThread
	OpImport
	OpGetGlobal
	OpSetGlobal
	OpGetLocal
	OpSetLocal
	OpGetSelf
	OpSetSelf
	OpResolveHost
	OpResolvePattern
	OpAddPattern
	OpResolvePatternNamespace
	OpAddPatternNamespace
	OpPatternNamespaceMemb
	OpSetMember
	OpSetIndex
	OpSetSlice
	OpSetBoolField
	OpSetIntField
	OpSetFloatField
	OpSetStructPtrField
	OpIterInit
	OpIterNext
	OpIterNextChunk
	OpIterKey
	OpIterValue
	OpIterPrune
	OpWalkerInit
	OpIntBin
	OpFloatBin
	OpNumBin
	OpPseudoArith
	OpLess
	OpLessEqual
	OpGreater
	OpGreaterEqual
	OptStrQueryParamVal
	OpStrConcat
	OpConcatStrLikes
	OpConcatBytesLikes
	OpConcatTuples
	OpRange
	OpMemb
	OpGetBoolField
	OpGetIntField
	OpGetFloatField
	OpGetStructPtrField
	OpObjPropNotStored
	OpExtensionMethod
	OpOptionalMemb
	OpComputedMemb
	OpAt
	OpSafeAt
	OpSlice
	OpAssert
	OpBlockLock
	OpBlockUnlock
	OpRuntimeTypecheck
	OpPushIncludedChunk
	OpPopIncludedChunk
	OpNoOp
	OpSuspendVM
)

// OpcodeNames contains the string representation of each opcode.
// TODO: improve names
var OpcodeNames = [...]string{
	OpPushConstant:                 "PUSH_CONST",
	OpPop:                          "POP",
	OpCopyTop:                      "COPY_TOP",
	OpSwap:                         "SWAP",
	OpMoveThirdTop:                 "SWAP_THIRD",
	OpPushTrue:                     "PUSH_TRUE",
	OpPushFalse:                    "PUSH_FALSE",
	OpEqual:                        "EQUAL",
	OpNotEqual:                     "NOT_EQUAL",
	OpIs:                           "IS",
	OpIsNot:                        "IS_NOT",
	OpMinus:                        "NEG",
	OpBooleanNot:                   "NOT",
	OpMatch:                        "MATCH",
	OpGroupMatch:                   "GRP_MATCH",
	OpIn:                           "IN",
	OpSubstrOf:                     "SUBSTR_OF",
	OpKeyOf:                        "KEY_OF",
	OpUrlOf:                        "URL_OF",
	OpDoSetDifference:              "DO_SET_DIFF",
	OpNilCoalesce:                  "NIL_COALESCE",
	OpJumpIfFalse:                  "JUMP_IFF",
	OpAndJump:                      "AND_JUMP",
	OpOrJump:                       "OR_JUMP",
	OpJump:                         "JUMP",
	OpPopJumpIfTestDisabled:        "PJUMP_IF_TDISABLED",
	OpPushNil:                      "PUSH_NIL",
	OpCreateList:                   "CRT_LST",
	OpCreateListDynLen:             "CRT_LST_DYN_LEN",
	OpCreateKeyList:                "CRT_KLST",
	OpCreateTuple:                  "CRT_TUPLE",
	OpCreateObject:                 "CRT_OBJ",
	OpCreateRecord:                 "CRT_REC",
	OpCreateDict:                   "CRT_DICT",
	OpCreateMapping:                "CRT_MPG",
	OpCreateTreedata:               "CRT_UDAT",
	OpCreateTreedataHiearchyEntry:  "CRT_UDHE",
	OpCreateOrderedPair:            "CRT_OPAIR",
	OpCreateModuleArgs:             "CRT_MOD_ARGS",
	OpSpreadObject:                 "SPREAD_OBJ",
	OpExtractProps:                 "EXTR_PROPS",
	OpSpreadList:                   "SPREAD_LST",
	OpSpreadTuple:                  "SPREAD_TPL",
	OpAppend:                       "APPEND",
	OpCreateListPattern:            "CRT_LSTP",
	OpCreateTuplePattern:           "CRT_TPLP",
	OpCreateObjectPattern:          "CRT_OBJP",
	OpCreateRecordPattern:          "CRT_RECP",
	OpCreateOptionPattern:          "CRT_OPTP",
	OpCreateUnionPattern:           "CRT_UP",
	OpCreateStringUnionPattern:     "CRT_SUP",
	OpCreateRepeatedPatternElement: "CRT_RPE",
	OpCreateSequenceStringPattern:  "CRT_SSP",
	OpCreatePatternNamespace:       "CRT_PNS",
	OpToPattern:                    "TO_PATT",
	OpCreateOptionalPattern:        "CRT_OPTP",
	OpToBool:                       "TO_BOOL",
	OpCreateString:                 "CRT_STR",
	OpCreateOption:                 "CRT_OPT",
	OpCreatePath:                   "CRT_PATH",
	OpCreatePathPattern:            "CRT_PATHP",
	OpCreateURL:                    "CRT_URL",
	OpCreateHost:                   "CRT_HST",
	OpCreateRuneRange:              "CRT_RUNERG",
	OpCreateIntRange:               "CRT_INTRG",
	OpCreateFloatRange:             "CRT_FLOATRG",
	OpCreateUpperBoundRange:        "CRT_UBRG",
	OpCreateTestSuite:              "CRT_TSTS",
	OpCreateTestCase:               "CRT_TSTC",
	OpAddTestSuiteResult:           "ADD_TST_RES",
	OpAddTestCaseResult:            "ADD_TSTC_RES",
	OpCreateXMLelem:                "CRT_XML_ELEM",
	OpCreateAddTypeExtension:       "CRT_ADD_TYPE_EXT",
	OpSpreadObjectPattern:          "SPRD_OBJP",
	OpSpreadRecordPattern:          "SPRD_RECP",
	BindCapturedLocals:             "BIND_LOCS",
	OpGetGlobal:                    "GET_GLOBAL",
	OpSetGlobal:                    "SET_GLOBAL",
	OpSetMember:                    "SET_MEMBER",
	OpSetIndex:                     "SET_INDEX",
	OpSetSlice:                     "SET_SLICE",
	OpSetBoolField:                 "SET_BOOL_FIELD",
	OpSetIntField:                  "SET_INT_FIELD",
	OpSetFloatField:                "SET_FLOAT_FIELD",
	OpSetStructPtrField:            "SET_STRUCT_PTR_FIELD",
	OpCall:                         "CALL",
	OpReturn:                       "RETURN",
	OpCallFromXMLFactory:           "CALL_FXML_FACTORY",
	OpYield:                        "YIELD",
	OpCoyield:                      "COYIELD",
	OpCallPattern:                  "CALL_PATT",
	OpDropPerms:                    "DROP_PERMS",
	OpSpawnLThread:                 "SPAWN_LTHREAD",
	OpImport:                       "IMPORT",
	OpGetLocal:                     "GET_LOCAL",
	OpSetLocal:                     "SET_LOCAL",
	OpGetSelf:                      "GET_SELF",
	OpSetSelf:                      "SET_SELF",
	OpResolveHost:                  "RSLV_HOST",
	OpResolvePattern:               "RSLV_PATT",
	OpAddPattern:                   "ADD_PATT",
	OpResolvePatternNamespace:      "RSLV_PNS",
	OpAddPatternNamespace:          "ADD_PATTNS",
	OpPatternNamespaceMemb:         "PNS_MEMB",
	OpIterInit:                     "ITER_INIT",
	OpIterNext:                     "ITER_NEXT",
	OpIterNextChunk:                "ITER_NEXT_CHUNK",
	OpIterKey:                      "ITER_KEY",
	OpIterValue:                    "ITER_VAL",
	OpIterPrune:                    "ITER_PRUNE",
	OpWalkerInit:                   "DWALK_INIT",
	OpIntBin:                       "INT_BIN",
	OpFloatBin:                     "FLOAT_BIN",
	OpNumBin:                       "NUM_BIN",
	OpPseudoArith:                  "PSEUDO_ARITH",
	OpLess:                         "OP_LESS",
	OpLessEqual:                    "OP_LESS_EQUAL",
	OpGreater:                      "OP_GREATER",
	OpGreaterEqual:                 "OP_GREATER_EQUAL",
	OpStrConcat:                    "STR_CONCAT",
	OptStrQueryParamVal:            "STRINGIFY_QPARAM",
	OpConcatStrLikes:               "CONCAT_STR_LIKES",
	OpConcatBytesLikes:             "CONCAT_BYTES",
	OpConcatTuples:                 "CONCAT_TUPLES",
	OpRange:                        "RANGE",
	OpMemb:                         "MEMB",
	OpGetBoolField:                 "GET_BOOL_FIELD",
	OpGetIntField:                  "GET_INT_FIELD",
	OpGetFloatField:                "GET_FLOAT_FIELD",
	OpGetStructPtrField:            "GET_STRUCT_PTR_FIELD",
	OpObjPropNotStored:             "OBJ_PROP_NOT_STORED",
	OpExtensionMethod:              "EXT_METHOD",
	OpOptionalMemb:                 "OPT_MEMB",
	OpComputedMemb:                 "COMPUTED_MEMB",
	OpAt:                           "AT",
	OpSafeAt:                       "SAFE_AT",
	OpSlice:                        "SLICE",
	OpAssert:                       "ASSERT",
	OpBlockLock:                    "BLOCK_LOCK",
	OpBlockUnlock:                  "BLOCK_LOCK",
	OpRuntimeTypecheck:             "TYPECHECK",
	OpPushIncludedChunk:            "PUSH_CHUNK",
	OpPopIncludedChunk:             "POP_CHUNK",
	OpNoOp:                         "NO_OP",
	OpSuspendVM:                    "SUSPEND",
}

// OpcodeOperands contains the number of operands of each opcode.
var OpcodeOperands = [...][]int{
	OpPushConstant:                 {2},
	OpPop:                          {},
	OpCopyTop:                      {},
	OpSwap:                         {},
	OpMoveThirdTop:                 {},
	OpPushTrue:                     {},
	OpPushFalse:                    {},
	OpEqual:                        {},
	OpNotEqual:                     {},
	OpIs:                           {},
	OpIsNot:                        {},
	OpMinus:                        {},
	OpBooleanNot:                   {},
	OpMatch:                        {},
	OpGroupMatch:                   {2},
	OpIn:                           {},
	OpSubstrOf:                     {},
	OpKeyOf:                        {},
	OpUrlOf:                        {},
	OpDoSetDifference:              {},
	OpNilCoalesce:                  {},
	OpJumpIfFalse:                  {2},
	OpAndJump:                      {2},
	OpOrJump:                       {2},
	OpJump:                         {2},
	OpPopJumpIfTestDisabled:        {2},
	OpPushNil:                      {},
	OpGetGlobal:                    {2},
	OpSetGlobal:                    {2},
	OpCreateList:                   {2},
	OpCreateListDynLen:             {},
	OpCreateKeyList:                {2},
	OpCreateTuple:                  {2},
	OpCreateObject:                 {2, 2},
	OpCreateRecord:                 {2},
	OpCreateDict:                   {2},
	OpCreateMapping:                {2},
	OpCreateTreedata:               {2},
	OpCreateTreedataHiearchyEntry:  {2},
	OpCreateOrderedPair:            {},
	OpCreateModuleArgs:             {2, 1},
	OpSpreadObject:                 {},
	OpExtractProps:                 {2},
	OpSpreadList:                   {},
	OpSpreadTuple:                  {},
	OpAppend:                       {2},
	OpCreateListPattern:            {2, 1},
	OpCreateTuplePattern:           {2, 1},
	OpCreateObjectPattern:          {2, 1},
	OpCreateRecordPattern:          {2, 1},
	OpCreateOptionPattern:          {2},
	OpCreateUnionPattern:           {2},
	OpCreateStringUnionPattern:     {2},
	OpCreateRepeatedPatternElement: {1, 1},
	OpCreateSequenceStringPattern:  {1, 2, 2},
	OpCreatePatternNamespace:       {},
	OpToPattern:                    {},
	OpCreateOptionalPattern:        {},
	OpToBool:                       {},
	OpCreateString:                 {1, 1, 2},
	OpCreateOption:                 {2},
	OpCreatePath:                   {1, 2},
	OpCreatePathPattern:            {1, 2},
	OpCreateURL:                    {2},
	OpCreateHost:                   {2},
	OpCreateRuneRange:              {},
	OpCreateIntRange:               {},
	OpCreateFloatRange:             {},
	OpCreateUpperBoundRange:        {},
	OpCreateTestSuite:              {2, 2},
	OpCreateTestCase:               {2, 2},
	OpAddTestSuiteResult:           {},
	OpAddTestCaseResult:            {},
	OpCreateXMLelem:                {2, 1, 2, 1},
	OpCreateAddTypeExtension:       {2},
	OpSpreadObjectPattern:          {},
	OpSpreadRecordPattern:          {},
	BindCapturedLocals:             {1},
	OpCall:                         {1, 1, 1},
	OpReturn:                       {1},
	OpYield:                        {1},
	OpCoyield:                      {1},
	OpCallPattern:                  {1},
	OpDropPerms:                    {},
	OpSpawnLThread:                 {1, 2, 2, 2},
	OpImport:                       {2},
	OpGetLocal:                     {1},
	OpSetLocal:                     {1},
	OpGetSelf:                      {},
	OpSetSelf:                      {},
	OpResolveHost:                  {2},
	OpResolvePattern:               {2},
	OpAddPattern:                   {2},
	OpResolvePatternNamespace:      {2},
	OpAddPatternNamespace:          {2},
	OpPatternNamespaceMemb:         {2, 2},
	OpSetMember:                    {2},
	OpSetIndex:                     {},
	OpSetSlice:                     {},
	OpSetBoolField:                 {2, 2},
	OpSetIntField:                  {2, 2},
	OpSetFloatField:                {2, 2},
	OpSetStructPtrField:            {2, 2},
	OpIterInit:                     {1},
	OpIterNext:                     {1},
	OpIterNextChunk:                {1},
	OpIterKey:                      {},
	OpIterValue:                    {1},
	OpIterPrune:                    {1},
	OpWalkerInit:                   {},
	OpIntBin:                       {1},
	OpFloatBin:                     {1},
	OpNumBin:                       {1},
	OpPseudoArith:                  {1},
	OpLess:                         {},
	OpLessEqual:                    {},
	OpGreater:                      {},
	OpGreaterEqual:                 {},
	OpStrConcat:                    {},
	OptStrQueryParamVal:            {},
	OpConcatStrLikes:               {2, 2},
	OpConcatBytesLikes:             {2, 2},
	OpConcatTuples:                 {2, 2},
	OpRange:                        {1},
	OpMemb:                         {2},
	OpGetBoolField:                 {2, 2},
	OpGetIntField:                  {2, 2},
	OpGetFloatField:                {2, 2},
	OpGetStructPtrField:            {2, 2},
	OpObjPropNotStored:             {2},
	OpExtensionMethod:              {2, 2},
	OpOptionalMemb:                 {2},
	OpComputedMemb:                 {},
	OpAt:                           {},
	OpSafeAt:                       {},
	OpSlice:                        {},
	OpAssert:                       {2},
	OpBlockLock:                    {1},
	OpBlockUnlock:                  {},
	OpRuntimeTypecheck:             {2},
	OpPushIncludedChunk:            {2},
	OpPopIncludedChunk:             {},
	OpNoOp:                         {},
	OpSuspendVM:                    {},
}

// OpcodeConstantIndexes stores for each opcode what arguments are indexes (positions) of constants.
var OpcodeConstantIndexes = [...][]bool{
	OpPushConstant:                 {true},
	OpPop:                          {},
	OpCopyTop:                      {},
	OpSwap:                         {},
	OpMoveThirdTop:                 {},
	OpPushTrue:                     {},
	OpPushFalse:                    {},
	OpEqual:                        {},
	OpNotEqual:                     {},
	OpIs:                           {},
	OpIsNot:                        {},
	OpMinus:                        {},
	OpBooleanNot:                   {},
	OpMatch:                        {},
	OpGroupMatch:                   {false},
	OpIn:                           {},
	OpSubstrOf:                     {},
	OpKeyOf:                        {},
	OpUrlOf:                        {},
	OpDoSetDifference:              {},
	OpNilCoalesce:                  {},
	OpJumpIfFalse:                  {false},
	OpAndJump:                      {false},
	OpOrJump:                       {false},
	OpJump:                         {false},
	OpPopJumpIfTestDisabled:        {false},
	OpPushNil:                      {},
	OpGetGlobal:                    {true},
	OpSetGlobal:                    {true},
	OpCreateList:                   {false},
	OpCreateListDynLen:             {},
	OpCreateKeyList:                {false},
	OpCreateTuple:                  {false},
	OpCreateObject:                 {false, true},
	OpCreateRecord:                 {false},
	OpCreateDict:                   {false},
	OpCreateMapping:                {true},
	OpCreateTreedata:               {false},
	OpCreateTreedataHiearchyEntry:  {false},
	OpCreateOrderedPair:            {},
	OpCreateModuleArgs:             {true, false},
	OpSpreadObject:                 {},
	OpExtractProps:                 {true},
	OpSpreadList:                   {},
	OpSpreadTuple:                  {},
	OpAppend:                       {false},
	OpCreateListPattern:            {false, false},
	OpCreateTuplePattern:           {false, false},
	OpCreateObjectPattern:          {false, false},
	OpCreateRecordPattern:          {false, false},
	OpCreateOptionPattern:          {true},
	OpCreateUnionPattern:           {false},
	OpCreateStringUnionPattern:     {false},
	OpCreateRepeatedPatternElement: {false, false},
	OpCreateSequenceStringPattern:  {false, true, true},
	OpCreatePatternNamespace:       {},
	OpToPattern:                    {},
	OpCreateOptionalPattern:        {},
	OpToBool:                       {},
	OpCreateString:                 {false, false, true},
	OpCreateOption:                 {true},
	OpCreatePath:                   {false, true},
	OpCreatePathPattern:            {false, true},
	OpCreateURL:                    {true},
	OpCreateHost:                   {true},
	OpCreateRuneRange:              {},
	OpCreateIntRange:               {},
	OpCreateFloatRange:             {},
	OpCreateUpperBoundRange:        {},
	OpCreateTestSuite:              {true, true},
	OpCreateTestCase:               {true, true},
	OpAddTestSuiteResult:           {},
	OpAddTestCaseResult:            {},
	OpCreateXMLelem:                {true, false, true, false},
	OpCreateAddTypeExtension:       {true},
	OpSpreadObjectPattern:          {},
	OpSpreadRecordPattern:          {},
	BindCapturedLocals:             {false},
	OpCall:                         {false, false, false},
	OpReturn:                       {false},
	OpCallFromXMLFactory:           {},
	OpYield:                        {false},
	OpCoyield:                      {false},
	OpCallPattern:                  {false},
	OpDropPerms:                    {},
	OpSpawnLThread:                 {false, true, true, true},
	OpImport:                       {true},
	OpGetLocal:                     {false},
	OpSetLocal:                     {false},
	OpGetSelf:                      {},
	OpSetSelf:                      {},
	OpResolveHost:                  {true},
	OpResolvePattern:               {true},
	OpAddPattern:                   {true},
	OpResolvePatternNamespace:      {true},
	OpAddPatternNamespace:          {true},
	OpPatternNamespaceMemb:         {true, true},
	OpSetMember:                    {true},
	OpSetIndex:                     {},
	OpSetSlice:                     {},
	OpSetBoolField:                 {false, false},
	OpSetIntField:                  {false, false},
	OpSetFloatField:                {false, false},
	OpSetStructPtrField:            {false, false},
	OpIterInit:                     {false},
	OpIterNext:                     {false},
	OpIterNextChunk:                {false},
	OpIterKey:                      {},
	OpIterValue:                    {false},
	OpIterPrune:                    {false},
	OpWalkerInit:                   {},
	OpIntBin:                       {false},
	OpFloatBin:                     {false},
	OpNumBin:                       {false},
	OpPseudoArith:                  {false},
	OpLess:                         {},
	OpLessEqual:                    {},
	OpGreater:                      {},
	OpGreaterEqual:                 {},
	OpStrConcat:                    {},
	OptStrQueryParamVal:            {},
	OpConcatStrLikes:               {false, true},
	OpConcatBytesLikes:             {false, true},
	OpConcatTuples:                 {false, true},
	OpRange:                        {false},
	OpMemb:                         {true},
	OpGetBoolField:                 {false, false},
	OpGetIntField:                  {false, false},
	OpGetFloatField:                {false, false},
	OpGetStructPtrField:            {false, false},
	OpObjPropNotStored:             {true},
	OpExtensionMethod:              {true, true},
	OpOptionalMemb:                 {true},
	OpComputedMemb:                 {},
	OpAt:                           {},
	OpSafeAt:                       {},
	OpSlice:                        {},
	OpAssert:                       {true},
	OpBlockLock:                    {false},
	OpBlockUnlock:                  {},
	OpRuntimeTypecheck:             {true},
	OpPushIncludedChunk:            {true},
	OpPopIncludedChunk:             {},
	OpNoOp:                         {},
	OpSuspendVM:                    {},
}

// ReadOperands reads the operands of an instruction in bytecode.
func ReadOperands(numOperands []int, instruction []byte) (operands []int, offset int) {
	for _, width := range numOperands {
		switch width {
		case 1:
			operands = append(operands, int(instruction[offset]))
		case 2:
			operands = append(operands, int(instruction[offset+1])|int(instruction[offset])<<8)
		}
		offset += width
	}
	return
}

// MakeInstruction returns a bytecode for an opcode and the operands.
func MakeInstruction(opcode Opcode, operands ...int) []byte {
	numOperands := OpcodeOperands[opcode]

	totalLen := 1
	for _, w := range numOperands {
		totalLen += w
	}

	instruction := make([]byte, totalLen)
	instruction[0] = opcode

	offset := 1
	for i, o := range operands {
		width := numOperands[i]
		switch width {
		case 1:
			instruction[offset] = byte(o)
		case 2:
			n := uint16(o)
			instruction[offset] = byte(n >> 8)
			instruction[offset+1] = byte(n)
		}
		offset += width
	}
	return instruction
}

// FormatInstructions returns string representation of bytecode instructions.
func FormatInstructions(ctx *Context, b []byte, posOffset int, leftPadding string, constants []Value) []string {

	var out []string

	fn := func(instr []byte, op Opcode, operands, constantIndexes []int, constants []Value, i int) ([]byte, error) {

		var consts []string

		for _, constant := range constants {
			consts = append(consts, Stringify(constant, nil))
		}

		switch len(operands) {
		case 0:
			out = append(out, fmt.Sprintf("%04d %-10s",
				posOffset+i, OpcodeNames[b[i]]))
		case 1:
			out = append(out, fmt.Sprintf("%04d %-10s %-5d %-5s %-5s %-5s",
				posOffset+i, OpcodeNames[b[i]], operands[0], "", "", ""))
		case 2:
			out = append(out, fmt.Sprintf("%04d %-10s %-5d %-5d %-5s %-5s",
				posOffset+i, OpcodeNames[b[i]],
				operands[0], operands[1], "", ""))
		case 3:
			out = append(out, fmt.Sprintf("%04d %-10s %-5d %-5d %-5d %-5s",
				posOffset+i, OpcodeNames[b[i]],
				operands[0], operands[1], operands[2], ""))
		case 4:
			out = append(out, fmt.Sprintf("%04d %-10s %-5d %-5d %-5d %-5d",
				posOffset+i, OpcodeNames[b[i]],
				operands[0], operands[1], operands[2], operands[3]))
		}

		s := leftPadding + out[len(out)-1]

		//add constants on the same line.
		if len(consts) >= 1 {
			s += " : " + strings.Join(consts, " ")
		}

		out[len(out)-1] = s
		i += len(instr)

		return nil, nil
	}

	_, err := MapInstructions(b, constants, fn)
	if err != nil {
		panic(err)
	}

	return out
}
