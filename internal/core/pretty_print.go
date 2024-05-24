package core

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"golang.org/x/exp/maps"

	"github.com/muesli/termenv"
)

const (
	prettyprint_BUFF_WRITER_SIZE = 100
	MAX_VALUE_PRINT_DEPTH        = 10
)

var (
	ANSI_RESET_SEQUENCE             = []byte(termenv.CSI + termenv.ResetSeq + "m")
	ANSI_RESET_SEQUENCE_STRING      = string(ANSI_RESET_SEQUENCE)
	DEFAULT_DARKMODE_DISCRETE_COLOR = pprint.DEFAULT_DARKMODE_PRINT_COLORS.DiscreteColor
	DEFAULT_LIGHMODE_DISCRETE_COLOR = pprint.DEFAULT_LIGHTMODE_PRINT_COLORS.DiscreteColor

	COMMA                               = []byte{','}
	LF_CR                               = []byte{'\n', '\r'}
	DASH_DASH                           = []byte{'-', '-'}
	SHARP_OPENING_PAREN                 = []byte{'#', '('}
	COLON_SPACE                         = []byte{':', ' '}
	COMMA_SPACE                         = []byte{',', ' '}
	CLOSING_BRACKET_CLOSING_PAREN       = []byte{']', ')'}
	CLOSING_CURLY_BRACKET_CLOSING_PAREN = []byte{'}', ')'}
	THREE_DOTS                          = []byte{'.', '.', '.'}
	DOT_OPENING_CURLY_BRACKET           = []byte{'.', '{'}
	DOT_DOT                             = []byte{'.', '.'}
	SLASH_SECOND_BYTES                  = []byte{'/', 's'}
)

type PrettyPrintColors struct {
	ControlKeyword, OtherKeyword, PatternLiteral, StringLiteral, PathLiteral, IdentifierLiteral,
	NumberLiteral, Constant, PatternIdentifier, CssTypeSelector, CssOtherSelector, InvalidNode, Index []byte
}

type PrettyPrintConfig struct {
	pprint.PrettyPrintConfig
	Context *Context
}

func (config *PrettyPrintConfig) WithContext(ctx *Context) *PrettyPrintConfig {
	newConfig := *config
	newConfig.Context = ctx
	return &newConfig
}

func GetFullColorSequence(color termenv.Color, bg bool) []byte {
	return pprint.GetFullColorSequence(color, bg)
}

// Stringify calls PrettyPrint on the passed value
func Stringify(v Value, ctx *Context) string {
	return StringifyWithConfig(v, &PrettyPrintConfig{
		PrettyPrintConfig: pprint.PrettyPrintConfig{
			MaxDepth: 7,
			Colorize: false,
			Compact:  true,
		},
		Context: ctx,
	})
}

// Stringify calls PrettyPrint on the passed value
func StringifyWithConfig(v Value, config *PrettyPrintConfig) string {
	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, prettyprint_BUFF_WRITER_SIZE)

	err := PrettyPrint(v, w, config, 0, 0)

	if err != nil {
		panic(fmt.Errorf("failed to stringify value of type %T: %w", v, err))
	}

	w.Flush()
	return buff.String()
}

func PrettyPrint(v Value, w io.Writer, config *PrettyPrintConfig, depth, parentIndentCount int) (err error) {
	buffered, ok := w.(*bufio.Writer)
	if !ok {
		buffered = bufio.NewWriterSize(w, prettyprint_BUFF_WRITER_SIZE)
	}

	defer func() {
		e := recover()
		switch v := e.(type) {
		case error:
			err = v
		default:
			err = fmt.Errorf("panic: %#v", e)
		case nil:
		}
	}()

	v.PrettyPrint(buffered, config, depth, parentIndentCount)
	return buffered.Flush()
}

type ColorizationInfo struct {
	Span          sourcecode.NodeSpan
	ColorSequence []byte
}

func GetNodeColorizations(chunk *ast.Chunk, lightMode bool) []ColorizationInfo {
	var colorizations []ColorizationInfo

	var colors = pprint.DEFAULT_DARKMODE_PRINT_COLORS
	if lightMode {
		colors = pprint.DEFAULT_LIGHTMODE_PRINT_COLORS
	}

	tokens := ast.GetTokens(chunk, chunk, false)
	for _, token := range tokens {
		switch token.Type {
		case ast.PERCENT_STR:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          token.Span,
				ColorSequence: colors.PatternIdentifier,
			})
			//control keywords
		case ast.BREAK_KEYWORD, ast.CONTINUE_KEYWORD, ast.PRUNE_KEYWORD, ast.COYIELD_KEYWORD, ast.YIELD_KEYWORD, ast.RETURN_KEYWORD,
			ast.DEFAULTCASE_KEYWORD, ast.SWITCH_KEYWORD, ast.MATCH_KEYWORD, ast.ASSERT_KEYWORD,
			ast.GO_KEYWORD, ast.DO_KEYWORD, ast.TESTSUITE_KEYWORD, ast.TESTCASE_KEYWORD, ast.COMP_KEYWORD,
			ast.FOR_KEYWORD, ast.IN_KEYWORD, ast.IF_KEYWORD, ast.ELSE_KEYWORD,
			ast.PREINIT_KEYWORD, ast.ON_KEYWORD, ast.WALK_KEYWORD,
			ast.DROP_PERMS_KEYWORD, ast.IMPORT_KEYWORD:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          token.Span,
				ColorSequence: colors.ControlKeyword,
			})
		//other keywords
		case ast.AS_KEYWORD, ast.IS_KEYWORD, ast.AND_KEYWORD, ast.OR_KEYWORD, ast.MAPPING_KEYWORD, ast.TREEDATA_KEYWORD,
			ast.FN_KEYWORD, ast.CONST_KEYWORD, ast.VAR_KEYWORD, ast.GLOBALVAR_KEYWORD, ast.ASSIGN_KEYWORD, ast.CONCAT_KEYWORD,
			ast.SENDVAL_KEYWORD, ast.SYNCHRONIZED_KEYWORD, ast.EXTEND_KEYWORD, ast.PATTERN_KEYWORD,
			ast.PNAMESPACE_KEYWORD, ast.STRUCT_KEYWORD, ast.NEW_KEYWORD, ast.SELF_KEYWORD, ast.URLOF_KEYWORD,
			ast.KEYOF_KEYWORD, ast.NOT_IN_KEYWORD, ast.NOT_MATCH_KEYWORD, ast.READONLY_KEYWORD, ast.TO_KEYWORD,
			ast.OTHERPROPS_KEYWORD, ast.INCLUDABLE_FILE_KEYWORD, ast.MANIFEST_KEYWORD:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          token.Span,
				ColorSequence: colors.OtherKeyword,
			})
		//
		case ast.PERCENT_FN:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          token.Span,
				ColorSequence: colors.PatternIdentifier,
			})
		}
	}

	ast.Walk(chunk, func(node, parent, scopeNode ast.Node, ancestorChain []ast.Node, _ bool) (ast.TraversalAction, error) {
		switch n := node.(type) {
		//literals
		case *ast.IdentifierLiteral:
			var colorSeq []byte
			if openingElem, ok := parent.(*ast.MarkupOpeningTag); ok && openingElem.Name == n {
				colorSeq = colors.MarkupTagName
			} else if _, ok := parent.(*ast.MarkupClosingTag); ok {
				colorSeq = colors.MarkupTagName
			} else {
				colorSeq = colors.IdentifierLiteral
			}

			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colorSeq,
			})

		case *ast.Variable, *ast.NamedPathSegment:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.IdentifierLiteral,
			})

		case *ast.PatternIdentifierLiteral, *ast.PatternNamespaceIdentifierLiteral:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PatternIdentifier,
			})
		case *ast.StringTemplateInterpolation:
			// colorizations = append(colorizations, ColorizationInfo{
			// 	Span:          n.Tokens[0].Span,
			// 	ColorSequence: colors.PatternIdentifier,
			// })
		case *ast.DoubleQuotedStringLiteral, *ast.UnquotedStringLiteral, *ast.MultilineStringLiteral, *ast.FlagLiteral,
			*ast.RuneLiteral, *ast.StringTemplateSlice:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.StringLiteral,
			})
		case *ast.ByteSliceLiteral:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.StringLiteral,
			})
		case *ast.AbsolutePathPatternLiteral, *ast.RelativePathPatternLiteral, *ast.URLPatternLiteral, *ast.HostPatternLiteral, *ast.RegularExpressionLiteral:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PatternLiteral,
			})
		case *ast.PathSlice:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PathLiteral,
			})
		case *ast.PathPatternSlice:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PatternLiteral,
			})
		case *ast.HostExpression:
			// colorizations = append(colorizations, ColorizationInfo{
			// 	Span:          n.Tokens[0].Span,
			// 	ColorSequence: colors.StringLiteral,
			// })
		case *ast.URLLiteral, *ast.SchemeLiteral, *ast.HostLiteral, *ast.AbsolutePathLiteral,
			*ast.RelativePathLiteral, *ast.URLQueryParameterValueSlice:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PathLiteral,
			})
		case *ast.IntLiteral, *ast.FloatLiteral, *ast.QuantityLiteral, *ast.PortLiteral,
			*ast.YearLiteral, *ast.DateLiteral, *ast.DateTimeLiteral, *ast.RateLiteral:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.NumberLiteral,
			})
		case *ast.BooleanLiteral, *ast.NilLiteral, *ast.UnambiguousIdentifierLiteral, *ast.PropertyNameLiteral,
			*ast.SelfExpression:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.Constant,
			})
		case *ast.InvalidURLPattern, *ast.InvalidURL, *ast.InvalidAliasRelatedNode, *ast.InvalidComplexStringPatternElement,
			*ast.UnknownNode:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.InvalidNode,
			})
		case *ast.OptionExpression:
			if n.Value == nil {
				break
			}

			if _, isMissingExpr := n.Value.(*ast.MissingExpression); isMissingExpr {
				break
			}

			colorizations = append(colorizations, ColorizationInfo{
				Span:          sourcecode.NodeSpan{Start: n.Span.Start, End: n.Value.Base().Span.Start - 1},
				ColorSequence: colors.StringLiteral,
			})
		case *ast.OptionPatternLiteral:
			if n.Value == nil {
				break
			}

			if _, isMissingExpr := n.Value.(*ast.MissingExpression); isMissingExpr {
				break
			}

			colorizations = append(colorizations, ColorizationInfo{
				Span:          sourcecode.NodeSpan{Start: n.Span.Start, End: n.Value.Base().Span.Start - 1},
				ColorSequence: colors.StringLiteral,
			})
		case *ast.CssTypeSelector:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.CssTypeSelector,
			})
		case *ast.CssIdSelector, *ast.CssClassSelector, *ast.CssPseudoClassSelector, *ast.CssPseudoElementSelector:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.OtherKeyword,
			})
		case *ast.PatternGroupName:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.Constant,
			})
		case *ast.MarkupOpeningTag, *ast.MarkupClosingTag:
			// for _, token := range n.Base().Tokens {
			// 	colorizations = append(colorizations, ColorizationInfo{
			// 		Span:          token.Span,
			// 		ColorSequence: colors.DiscreteColor,
			// 	})
			// }
		}
		return ast.ContinueTraversal, nil
	}, nil)

	sort.Slice(colorizations, func(i, j int) bool {
		return colorizations[i].Span.Start < colorizations[j].Span.Start
	})

	var uniqueColorizations []ColorizationInfo

	for i := 0; i < len(colorizations); i++ {
		if i != 0 && colorizations[i].Span == colorizations[i-1].Span {
			continue
		}
		uniqueColorizations = append(uniqueColorizations, colorizations[i])
	}

	return uniqueColorizations
}

func PrintColorizedChunk(w io.Writer, chunk *ast.Chunk, code []rune, lightMode bool, fgColorSequence []byte) {
	prevColorizationEndIndex := int(0)

	// TODO: reduce memory allocations

	colorizations := GetNodeColorizations(chunk, lightMode)

	for _, colorization := range colorizations {
		w.Write(fgColorSequence)
		w.Write([]byte(string(code[prevColorizationEndIndex:colorization.Span.Start])))

		w.Write(colorization.ColorSequence)
		w.Write([]byte(string(code[colorization.Span.Start:min(len(code), int(colorization.Span.End))])))
		w.Write(ANSI_RESET_SEQUENCE)

		prevColorizationEndIndex = int(colorization.Span.End)
	}

	if prevColorizationEndIndex < len(code) {
		w.Write([]byte(string(code[prevColorizationEndIndex:])))
	}
}

func GetColorizedChunk(chunk *ast.Chunk, code []rune, lightMode bool, fgColorSequence []byte) string {
	buf := bytes.NewBuffer(nil)
	PrintColorizedChunk(buf, chunk, code, lightMode, fgColorSequence)
	return buf.String()
}

func (n AstNode) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, n)
}

func (t Token) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, t)
}

func (Nil NilT) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		for _, b := range [][]byte{config.Colors.Constant, {'n', 'i', 'l'}, ANSI_RESET_SEQUENCE} {
			utils.Must(w.Write(b))
		}
	} else {
		utils.Must(w.Write([]byte{'n', 'i', 'l'}))
	}
}

func (err Error) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "(%T)%s", err.goError, err.goError.Error()))
}

func (boolean Bool) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	var b = []byte{'f', 'a', 'l', 's', 'e'}
	if boolean {
		b = []byte{'t', 'r', 'u', 'e'}
	}

	if config.Colorize {
		for _, b := range [][]byte{config.Colors.Constant, b, ANSI_RESET_SEQUENCE} {
			utils.Must(w.Write(b))
		}
	} else {
		utils.Must(w.Write(b))
	}
}

func (r Rune) reprBytes() []byte {
	return []byte(commonfmt.FmtRune(rune(r)))
}

func (r Rune) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.StringLiteral))
	}

	utils.Must(w.Write(r.reprBytes()))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (b Byte) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, b)
}

func (i Int) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	s := strconv.FormatInt(int64(i), 10)
	utils.Must(w.Write(utils.StringAsBytes(s)))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (f Float) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	s := strconv.FormatFloat(float64(f), 'f', -1, 64)
	utils.Must(w.Write(utils.StringAsBytes(s)))

	if !strings.Contains(s, ".") {
		utils.Must(w.Write(utils.StringAsBytes(".0")))
	}

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (s String) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if depth == 0 && config.PrintDecodedTopLevelStrings {
		utils.Must(w.Write([]byte(utils.StripANSISequences(string(s)))))
		return
	}

	if config.Colorize {
		utils.Must(w.Write(config.Colors.StringLiteral))
	}

	jsonStr := utils.Must(utils.MarshalJsonNoHTMLEspace(string(s)))

	if depth > config.MaxDepth && len(jsonStr) > 3 {
		//TODO: fix cut
		utils.Must(w.Write(jsonStr[:3]))
		utils.Must(w.Write(utils.StringAsBytes("...\"")))
	} else {
		utils.Must(w.Write(jsonStr))
	}

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (obj *Object) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if depth > config.MaxDepth && len(obj.keys) > 0 {
		utils.Must(w.Write(utils.StringAsBytes("{(...)}")))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	utils.Must(w.Write(utils.StringAsBytes("{")))

	for i, k := range obj.keys {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

		if config.Colorize {
			utils.Must(w.Write(config.Colors.IdentifierLiteral))

		}

		utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(k))))

		if config.Colorize {
			utils.Must(w.Write(ANSI_RESET_SEQUENCE))
		}

		//colon
		utils.Must(w.Write(COLON_SPACE))

		//value
		v := obj.values[i]
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(obj.keys)-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}
	}

	if !config.Compact && len(obj.keys) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
}

func (rec Record) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if depth > config.MaxDepth && len(rec.keys) > 0 {
		utils.Must(w.Write(utils.StringAsBytes("#{(...)}")))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	utils.Must(w.Write(utils.StringAsBytes("#{")))

	for i, k := range rec.keys {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

		if config.Colorize {
			utils.Must(w.Write(config.Colors.IdentifierLiteral))

		}

		utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(k))))

		if config.Colorize {
			utils.Must(w.Write(ANSI_RESET_SEQUENCE))

		}

		//colon
		utils.Must(w.Write(COLON_SPACE))

		//value
		v := rec.values[i]
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(rec.keys)-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}
	}

	if !config.Compact && len(rec.keys) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
}

func (dict *Dictionary) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	//TODO: prevent modification of the dictionary while this function is running

	if depth > config.MaxDepth && len(dict.entries) > 0 {
		utils.Must(w.Write(utils.StringAsBytes(":{(...)}")))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	utils.Must(w.Write(utils.StringAsBytes(":{")))

	var keys []string
	for k := range dict.entries {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, k := range keys {
		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

		//key
		if config.Colorize {
			utils.Must(w.Write(config.Colors.StringLiteral))

		}
		utils.Must(w.Write(utils.StringAsBytes(k)))

		if config.Colorize {
			utils.Must(w.Write(ANSI_RESET_SEQUENCE))
		}

		//colon
		utils.Must(w.Write(COLON_SPACE))

		//value
		v := dict.entries[k]

		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(keys)-1

		if !isLastEntry {
			utils.Must(w.Write([]byte{',', ' '}))

		}

	}

	if !config.Compact && len(keys) > 0 {
		utils.Must(w.Write(LF_CR))
	}
	utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
	utils.PanicIfErr(w.WriteByte('}'))
}

func (list KeyList) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if depth > config.MaxDepth && len(list) > 0 {
		utils.Must(w.Write(utils.StringAsBytes(".{(...)]}")))
		return
	}

	utils.Must(w.Write(DOT_OPENING_CURLY_BRACKET))

	first := true

	for _, k := range list {
		if !first {
			utils.Must(w.Write(COMMA_SPACE))
		}
		first = false

		utils.Must(w.Write([]byte(k)))
	}

	utils.PanicIfErr(w.WriteByte('}'))
}

func PrettyPrintList(list underlyingList, w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	//TODO: prevent modification of the list while this function is running
	length := list.Len()

	if depth > config.MaxDepth && length > 0 {
		utils.Must(w.Write(utils.StringAsBytes("[(...)]")))
		return
	}

	utils.PanicIfErr(w.WriteByte('['))

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)
	printIndices := !config.Compact && length > 10

	for i := 0; i < length; i++ {
		v := list.At(config.Context, i)

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))

			//index
			if printIndices {
				if config.Colorize {
					utils.Must(w.Write(config.Colors.DiscreteColor))
				}
				if i < 10 {
					utils.PanicIfErr(w.WriteByte(' '))
				}
				utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(int64(i), 10))))
				utils.Must(w.Write(COLON_SPACE))
				if config.Colorize {
					utils.Must(w.Write(ANSI_RESET_SEQUENCE))
				}
			}
		}

		//element
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == length-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}

	}

	var end []byte
	if !config.Compact && length > 0 {
		end = append(end, '\n', '\r')
	}
	end = append(end, bytes.Repeat(config.Indent, depth)...)
	end = append(end, ']')

	utils.Must(w.Write(end))
}

func (list *List) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	list.underlyingList.PrettyPrint(w, config, depth, parentIndentCount)
}

func (list *ValueList) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrettyPrintList(list, w, config, depth, parentIndentCount)
}

func (list *NumberList[T]) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrettyPrintList(list, w, config, depth, parentIndentCount)
}

func (list *BoolList) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrettyPrintList(list, w, config, depth, parentIndentCount)
}

func (list *StringList) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrettyPrintList(list, w, config, depth, parentIndentCount)
}

func (tuple Tuple) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	lst := &List{underlyingList: &ValueList{elements: tuple.elements}}
	utils.Must(w.Write([]byte{'#'}))

	lst.PrettyPrint(w, config, depth, parentIndentCount)
}

func (p OrderedPair) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("ordered-pair")))
	lst := &List{underlyingList: &ValueList{elements: p[:]}}
	lst.PrettyPrint(w, config, depth, parentIndentCount)
}

func (a *Array) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	//TODO: prevent modification of the array while this function is running
	length := a.Len()

	if depth > config.MaxDepth && length > 0 {
		utils.Must(w.Write(utils.StringAsBytes("Array(...)")))
		return
	}

	utils.Must(w.Write(utils.StringAsBytes("Array(")))

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)
	printIndices := !config.Compact && length > 10

	for i := 0; i < length; i++ {
		v := (*a)[i]

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))

			//index
			if printIndices {
				if config.Colorize {
					utils.Must(w.Write(config.Colors.DiscreteColor))
				}
				if i < 10 {
					utils.PanicIfErr(w.WriteByte(' '))
				}
				utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(int64(i), 10))))
				utils.Must(w.Write(COLON_SPACE))
				if config.Colorize {
					utils.Must(w.Write(ANSI_RESET_SEQUENCE))
				}
			}
		}

		//element
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == length-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}

	}

	var end []byte
	if !config.Compact && length > 0 {
		end = append(end, '\n', '\r')
	}
	end = append(end, bytes.Repeat(config.Indent, depth)...)
	end = append(end, ')')

	utils.Must(w.Write(end))
}

func (args *ModuleArgs) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("module-arguments{")))

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	argNames := maps.Keys(args.values)

	sort.Strings(argNames)

	for i, argName := range argNames {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

		if config.Colorize {
			utils.Must(w.Write(config.Colors.IdentifierLiteral))

		}

		utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(argName))))

		if config.Colorize {
			utils.Must(w.Write(ANSI_RESET_SEQUENCE))
		}

		//colon
		utils.Must(w.Write(COLON_SPACE))

		//value
		v := args.values[argName]
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(argNames)-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}
	}

	if !config.Compact && len(argNames) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
}

func (slice *RuneSlice) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, slice)
}

func (slice *ByteSlice) write(w io.Writer) (int, error) {
	totalN, err := w.Write([]byte{'0', 'x', '['})
	if err != nil {
		return totalN, err
	}

	n, err := hex.NewEncoder(w).Write(slice.bytes)
	totalN += n
	if err != nil {
		return totalN, err
	}

	n, err = w.Write([]byte{']'})
	totalN += n
	return totalN, err
}

func (slice *ByteSlice) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	var bytes []byte

	if depth > config.MaxDepth && len(slice.bytes) > 2 {
		//TODO: fix cut
		bytes = []byte("0x[...]")
		if !config.Colorize {
			utils.Must(w.Write(bytes))
			return
		}
	}

	if config.Colorize {
		for _, b := range [][]byte{config.Colors.StringLiteral, bytes, ANSI_RESET_SEQUENCE} {
			if b == nil {
				utils.Must(slice.write(w))
			} else {
				utils.Must(w.Write(b))
			}
		}

	} else {
		utils.Must(slice.write(w))
	}
}

func (v *GoFunction) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.WriteString(commonfmt.FormatGoFunctionSignature(v.fn)))
}

func (opt Option) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.StringLiteral))
	}

	if len(opt.Name) <= 1 {
		utils.PanicIfErr(w.WriteByte('-'))
	} else {
		utils.Must(w.Write(DASH_DASH))
	}

	utils.Must(w.Write(utils.StringAsBytes(opt.Name)))

	if boolean, ok := opt.Value.(Bool); ok && bool(boolean) {
		return
	}

	utils.PanicIfErr(w.WriteByte('='))
	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}

	if depth > config.MaxDepth {
		utils.Must(w.Write(utils.StringAsBytes("(...)")))
	} else {
		opt.Value.PrettyPrint(w, config, depth, parentIndentCount)
	}
}

func (pth Path) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	utils.Must(parse.PrintPath(w, pth))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (patt PathPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PatternLiteral))
	}

	utils.Must(parse.PrintPathPattern(w, patt))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (u URL) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	_, err := w.Write(utils.StringAsBytes(u))
	utils.PanicIfErr(err)

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (scheme Scheme) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	_, err := w.Write(utils.StringAsBytes(scheme + "://"))
	utils.PanicIfErr(err)

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (host Host) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	_, err := w.Write(utils.StringAsBytes(host))
	utils.PanicIfErr(err)

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (patt HostPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PatternLiteral))
	}

	var b = []byte{'%'}
	b = append(b, patt...)

	_, err := w.Write(b)
	utils.PanicIfErr(err)

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (addr EmailAddress) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	if len(addr) < 5 {
		jsonStr, err := utils.MarshalJsonNoHTMLEspace(string(addr))
		utils.PanicIfErr(err)

		_, err = w.Write(utils.StringAsBytes(globalnames.EMAIL_ADDRESS_FN))
		utils.PanicIfErr(err)

		_, err = w.Write(jsonStr)
		utils.PanicIfErr(err)
	}

	addrS := string(addr)
	atDomainIndex := strings.LastIndexByte(addrS, '@')
	if atDomainIndex < 0 {
		panic(ErrInvalidEmailAdddres)
	}

	name := addrS[:atDomainIndex]
	atDomain := addrS[atDomainIndex:]

	finalString := name[0:1] + strings.Repeat("*", len(name)-1) + atDomain

	jsonStr, err := utils.MarshalJsonNoHTMLEspace(string(finalString))
	utils.PanicIfErr(err)

	_, err = w.Write(utils.StringAsBytes(globalnames.EMAIL_ADDRESS_FN))
	utils.PanicIfErr(err)

	_, err = w.Write(jsonStr)
	utils.PanicIfErr(err)

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (patt URLPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PatternLiteral))
	}

	var b = []byte{'%'}
	b = append(b, patt...)

	_, err := w.Write(b)
	utils.PanicIfErr(err)

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (i Identifier) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.Constant))
	}

	utils.Must(w.Write(SHARP_OPENING_PAREN))
	utils.Must(w.Write(utils.StringAsBytes(i)))
	utils.PanicIfErr(w.WriteByte((')')))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (n PropertyName) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.Constant))
	}

	utils.PanicIfErr(w.WriteByte('.'))
	utils.Must(w.Write(utils.StringAsBytes(n)))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (p *LongValuePath) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	for _, segment := range *p {
		segment.PrettyPrint(w, config, depth+1, 0)
	}
}

func (str CheckedString) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PatternIdentifier))
	}

	utils.PanicIfErr(w.WriteByte('%'))
	utils.Must(w.Write(utils.StringAsBytes(str.matchingPatternName)))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
		utils.Must(w.Write(config.Colors.StringLiteral))
	}

	utils.PanicIfErr(w.WriteByte('`'))

	jsonStr, _ := utils.MarshalJsonNoHTMLEspace(str.str)
	utils.Must(w.Write(jsonStr[1 : len(jsonStr)-1]))
	utils.PanicIfErr(w.WriteByte('`'))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (count ByteCount) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(count.Write(w, -1))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (count LineCount) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(count.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (count RuneCount) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(count.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (rate ByteRate) write(w io.Writer) (int, error) {
	if rate < 0 {
		return 0, ErrNegByteRate
	}
	totalN := 0
	if n, err := ByteCount(rate).Write(w, -1); err != nil {
		return n, err
	} else {
		totalN = n
	}
	n, err := w.Write(SLASH_SECOND_BYTES)
	totalN += n
	return totalN, err
}

func (rate ByteRate) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(rate.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (f Frequency) write(w io.Writer) (int, error) {
	var format = "%gx/s"

	if f < 0 {
		return 0, ErrNegFrequency
	}

	switch {
	case f >= 1e9:
		format = "%gGx/s"
		f /= 1e9
		if utils.IsWholeInt64(f) && f >= 1000 {
			format = "%g.0Gx/s"
		}
	case f >= 1e6:
		format = "%gMx/s"
		f /= 1e6
		if utils.IsWholeInt64(f) && f >= 1000 {
			format = "%g.0Mx/s"
		}
	case f >= 1e3:
		format = "%gkx/s"
		f /= 1e3
		if utils.IsWholeInt64(f) && f >= 1000 {
			format = "%g.0kx/s"
		}
	case f >= 0:
		if f < 1.0 {
			//Add '.0' before exponent if present.
			//We do this because 'e' would be parsed as an unit.
			var buf [20]byte
			res := strconv.AppendFloat(buf[:], float64(f), 'g', -1, 64)

			if !bytes.ContainsAny(res, ".") {
				exponentIndex := bytes.IndexAny(res, "e")

				//Shift the exponent part to the right.
				res = res[:len(res)+2]
				copy(res[exponentIndex+2:], res[exponentIndex:])

				res[exponentIndex] = '.'
				res[exponentIndex+1] = '0'
			}
			res = append(res, "x/s"...)
			return w.Write(res)
		}
	default:
		return 0, ErrNoRepresentation
	}

	return fmt.Fprintf(w, format, f)
}

func (f Frequency) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(f.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (d Duration) write(w io.Writer) (int, error) {
	return w.Write(utils.StringAsBytes(commonfmt.FmtInoxDuration(time.Duration(d))))
}

func (d Duration) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(d.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (y Year) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(y.write(w))
}

func (d Date) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(d.write(w))
}

func (d DateTime) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(d.write(w))
}

func (m FileMode) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	s := os.FileMode(m).String()

	utils.Must(w.Write(utils.StringAsBytes(s)))
}

func (r RuneRange) write(w io.Writer) (int, error) {
	b := []byte{'\''}
	b = append(b, string(r.Start)...)
	b = append(b, '\'', '.', '.', '\'')
	b = append(b, string(r.End)...)
	b = append(b, '\'')

	return w.Write(b)
}

func (r RuneRange) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(r.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (r QuantityRange) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	if r.start != nil {
		r.start.PrettyPrint(w, config, depth+1, 0)
	}

	_, err := w.Write(DOT_DOT)
	if err != nil {
		utils.PanicIfErr(err)
	}

	if r.end != nil {
		r.end.PrettyPrint(w, config, depth+1, 0)
		utils.PanicIfErr(err)
	}

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}

}

func (r IntRange) write(w io.Writer) (int, error) {
	b := make([]byte, 0, 10)
	if !r.unknownStart {
		b = append(b, strconv.FormatInt(r.start, 10)...)
	}
	b = append(b, '.', '.')
	b = append(b, strconv.FormatInt(r.end, 10)...)

	return w.Write(b)
}

func (r IntRange) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(r.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func fmtFloat(f float64) string {
	return strconv.FormatFloat(float64(f), 'f', -1, 64)
}

func (r FloatRange) write(w io.Writer) (int, error) {
	b := make([]byte, 0, 10)
	if !r.unknownStart {
		repr := fmtFloat(r.start)
		b = append(b, repr...)

		hasPoint := false
		for _, r := range repr {
			if r == '.' {
				hasPoint = true
			}
		}

		if !hasPoint {
			b = append(b, '.', '0')
		}
	}
	b = append(b, '.', '.')

	repr := fmtFloat(r.end)
	b = append(b, repr...)

	hasPoint := false
	for _, r := range repr {
		if r == '.' {
			hasPoint = true
		}
	}

	if !hasPoint {
		b = append(b, '.', '0')
	}

	return w.Write(b)
}

func (r FloatRange) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(r.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

//patterns

func (pattern ExactValuePattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("%(")
	pattern.value.PrettyPrint(w, config, depth, parentIndentCount)
	w.WriteString(")")
}

func (pattern ExactStringPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, pattern)
}

func (pattern TypePattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PatternIdentifier))
	}

	utils.PanicIfErr(w.WriteByte('%'))
	utils.Must(w.Write(utils.StringAsBytes(pattern.Name)))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (pattern *DifferencePattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, pattern)
}

func (pattern *OptionalPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, pattern)
}

func (pattern *FunctionPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, pattern)
}

func (patt *RegexPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *UnionPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *IntersectionPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *LengthCheckingStringPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *SequenceStringPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *UnionStringPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *RuneRangeStringPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *IntRangePattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *FloatRangePattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *DynamicStringPatternElement) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *RepeatedPatternElement) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *NamedSegmentPathPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt ObjectPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if depth > config.MaxDepth && len(patt.entries) > 0 {
		utils.Must(w.Write(utils.StringAsBytes("%{(...)}")))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	utils.Must(w.Write([]byte{'%', '{'}))

	for i, entry := range patt.entries {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

		if config.Colorize {
			utils.Must(w.Write(config.Colors.IdentifierLiteral))
		}

		utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(entry.Name))))

		if config.Colorize {
			utils.Must(w.Write(ANSI_RESET_SEQUENCE))
		}

		//colon
		utils.Must(w.Write(COLON_SPACE))

		//write entry pattern
		entry.Pattern.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(patt.entries)-1

		if !isLastEntry || patt.inexact {
			utils.Must(w.Write(COMMA_SPACE))
		}
	}

	// if patt.inexact {
	// 	if !config.Compact {
	// 		utils.Must(w.Write(LF_CR))
	// 		utils.Must(w.Write(indent))
	// 	}

	// 	utils.Must(w.Write(THREE_DOTS))
	// }

	if !config.Compact && len(patt.entries) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
	if err := w.WriteByte('}'); err != nil {
		panic(err)
	}
}

func (patt *RecordPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if depth > config.MaxDepth && len(patt.entries) > 0 {
		utils.Must(w.Write(utils.StringAsBytes("record(%{(...)})")))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	utils.Must(w.Write(utils.StringAsBytes("record(%{")))

	for i, entry := range patt.entries {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

		if config.Colorize {
			utils.Must(w.Write(config.Colors.IdentifierLiteral))
		}

		utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(entry.Name))))

		if config.Colorize {
			utils.Must(w.Write(ANSI_RESET_SEQUENCE))
		}

		//colon
		utils.Must(w.Write(COLON_SPACE))

		//write entry pattern
		entry.Pattern.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(patt.entries)-1

		if !isLastEntry || patt.inexact {
			utils.Must(w.Write(COMMA_SPACE))

		}
	}

	if patt.inexact {
		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

		utils.Must(w.Write(THREE_DOTS))
	}

	if !config.Compact && len(patt.entries) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
	utils.Must(w.Write(CLOSING_CURLY_BRACKET_CLOSING_PAREN))
}

func prettyPrintListPattern(
	w *bufio.Writer, tuplePattern bool,
	generalElementPattern Pattern, elementPatterns []Pattern,
	config *PrettyPrintConfig, depth int, parentIndentCount int,

) {

	if generalElementPattern != nil {
		b := utils.StringAsBytes("%[]")

		if tuplePattern {
			b = utils.StringAsBytes("%tuple(")
		}

		utils.Must(w.Write(b))

		generalElementPattern.PrettyPrint(w, config, depth, parentIndentCount)

		if tuplePattern {
			utils.Must(w.Write(utils.StringAsBytes(")")))
		}
		return
	}

	if depth > config.MaxDepth && len(elementPatterns) > 0 {
		b := utils.StringAsBytes("%[(...)]")
		if tuplePattern {
			b = utils.StringAsBytes("%tuple(...)")
		}

		utils.Must(w.Write(b))
		return
	}

	start := utils.StringAsBytes("%[")
	if tuplePattern {
		start = utils.StringAsBytes("%tuple(%[")
	}
	utils.Must(w.Write(start))

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)
	printIndices := !config.Compact && len(elementPatterns) > 10

	for i, v := range elementPatterns {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))

			//index
			if printIndices {
				if config.Colorize {
					utils.Must(w.Write(config.Colors.DiscreteColor))
				}
				if i < 10 {
					utils.PanicIfErr(w.WriteByte(' '))
				}
				utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(int64(i), 10))))
				utils.Must(w.Write(config.Colors.DiscreteColor))
				utils.Must(w.Write(COLON_SPACE))

				if config.Colorize {
					utils.Must(w.Write(ANSI_RESET_SEQUENCE))
				}
			}
		}

		//element
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(elementPatterns)-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}

	}

	if !config.Compact && len(elementPatterns) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
	if tuplePattern {
		utils.Must(w.Write(CLOSING_BRACKET_CLOSING_PAREN))
	} else {
		utils.PanicIfErr(w.WriteByte(']'))
	}
}

func (i FileInfo) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {

	if i.IsDir() {
		if config.Colorize {
			utils.Must(w.Write(config.Colors.Folder))
		}
		utils.Must(w.Write(utils.StringAsBytes(i.BaseName_)))
		utils.PanicIfErr(w.WriteByte('/'))
		if config.Colorize {
			utils.Must(w.Write(ANSI_RESET_SEQUENCE))
		}
	} else {

		if i.Mode_.Executable() && config.Colorize {
			utils.Must(w.Write(config.Colors.Executable))
		}
		utils.Must(w.Write(utils.StringAsBytes(i.BaseName_)))

		if config.Colorize {
			utils.Must(w.Write(config.Colors.DiscreteColor))
		}
		utils.PanicIfErr(w.WriteByte(' '))
		utils.Must(i.Size_.Write(w, 1))

	}

	if config.Colorize {
		utils.Must(w.Write(config.Colors.DiscreteColor))
	}

	utils.PanicIfErr(w.WriteByte(' '))
	utils.Must(w.Write(utils.StringAsBytes(os.FileMode(i.Mode_).String())))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (patt ListPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	prettyPrintListPattern(w, false, patt.generalElementPattern, patt.elementPatterns, config, depth, parentIndentCount)
}

func (patt TuplePattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	prettyPrintListPattern(w, true, patt.generalElementPattern, patt.elementPatterns, config, depth, parentIndentCount)
}

func (patt OptionPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *EventPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *MutationPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *ParserBasedPseudoPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *IntRangeStringPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *FloatRangeStringPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (patt *PathStringPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, patt)
}

func (reader *Reader) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, reader)
}

func (writer *Writer) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, writer)
}

func (mt Mimetype) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, mt)
}

func (r *LThread) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, r)
}

func (g *LThreadGroup) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, g)
}

func (g *InoxFunction) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, g)
}

func (it *KeyFilteredIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *ValueFilteredIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *KeyValueFilteredIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *ArrayIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *indexableIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *immutableSliceIterator[T]) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it IntRangeIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it FloatRangeIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it RuneRangeIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it QuantityRangeIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *PatternIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it indexedEntryIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *IpropsIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *EventSourceIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *DirWalker) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *TreedataWalker) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *ValueListIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *NumberListIterator[T]) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *BitSetIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *StrListIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *TupleIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (t Type) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	pkg := t.PkgPath()
	if pkg != "" {
		lastSlashIndex := strings.LastIndex(pkg, "/")
		utils.Must(w.WriteString(pkg[lastSlashIndex+1:]))
		utils.PanicIfErr(w.WriteByte('.'))
	}

	utils.Must(w.WriteString(t.Name()))
}

func (tx *Transaction) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, tx)
}

func (r *RandomnessSource) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, r)
}

func (m *Mapping) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, m)
}

func (ns *PatternNamespace) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, ns)
}

func (port Port) repr(quote bool) []byte {
	b := make([]byte, 0, 10)
	if quote {
		b = append(b, '"')
	}
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(port.Number), 10)
	if port.Scheme != NO_SCHEME_SCHEME_NAME && port.Scheme != "" {
		b = append(b, '/')
		b = append(b, port.Scheme...)
	}
	if quote {
		b = append(b, '"')
	}

	return b
}

func (port Port) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	repr := port.repr(false)
	if config.Colorize {
		for _, b := range [][]byte{config.Colors.NumberLiteral, repr, ANSI_RESET_SEQUENCE} {
			utils.Must(w.Write(b))
		}
	} else {
		utils.Must(w.Write(repr))
	}

}

func (u *Treedata) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {

	if config.Colorize {
		for _, b := range [][]byte{config.Colors.OtherKeyword, utils.StringAsBytes("treedata"), ANSI_RESET_SEQUENCE} {
			utils.Must(w.Write(b))
		}
	} else {
		utils.Must(w.WriteString("treedata"))
	}

	if depth > config.MaxDepth {
		utils.Must(w.WriteString("(...)"))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	w.WriteByte(' ')

	if u.Root != nil {
		u.Root.PrettyPrint(w, config, depth+1, indentCount)
		utils.Must(w.WriteString(" {"))
	} else {
		utils.PanicIfErr(w.WriteByte('{'))
	}

	first := true

	for _, entry := range u.HiearchyEntries {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		} else if !first {
			w.WriteString(", ")
		}

		first = false

		if config.Colorize {
			utils.Must(w.Write(config.Colors.IdentifierLiteral))
		}

		entry.PrettyPrint(w, config, depth+1, indentCount)
	}

	if !config.Compact && len(u.HiearchyEntries) > 0 {
		utils.Must(w.Write(LF_CR))
	}
	utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
	utils.PanicIfErr(w.WriteByte('}'))
}

func (e TreedataHiearchyEntry) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	e.Value.PrettyPrint(w, config, depth+1, indentCount)

	if len(e.Children) > 0 {
		utils.Must(w.Write([]byte{' ', '{'}))

		first := true
		for _, entry := range e.Children {
			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))
			} else if !first {
				w.WriteString(", ")
			}
			first = false

			if config.Colorize {
				utils.Must(w.Write(config.Colors.IdentifierLiteral))
			}

			entry.PrettyPrint(w, config, depth+1, indentCount)
		}

		if !config.Compact && len(e.Children) > 0 {
			utils.Must(w.Write(LF_CR))
		}

		utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
		utils.PanicIfErr(w.WriteByte('}'))
	}

}

func (c *StringConcatenation) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	//TODO: improve implementation

	String(c.GetOrBuildString()).PrettyPrint(w, config, depth, parentIndentCount)
}

func (c *BytesConcatenation) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	//TODO: improve implementation

	c.GetOrBuildBytes().PrettyPrint(w, config, depth, parentIndentCount)
}

// func (s *TestSuite) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
// 	InspectPrint(w, s)
// }

// func (c *TestCase) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
// 	InspectPrint(w, c)
// }

// func (r *TestCaseResult) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
// 	if r.Success {
// 		w.Write(utils.StringAsBytes(r.Message))
// 	} else {
// 		w.Write(utils.StringAsBytes(r.Message))
// 	}
// }

func (e *Event) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, e)
}

func (s *ExecutedStep) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, s)
}

func (watcher *GenericWatcher) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, watcher)
}

func (watcher *PeriodicWatcher) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, watcher)
}

func (m Mutation) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, m)
}

func (watcher *joinedWatchers) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, watcher)
}

func (watcher stoppedWatcher) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, watcher)
}

func (s *wrappedWatcherStream) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, s)
}

func (s *ElementsStream) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, s)
}

func (s *ReadableByteStream) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, s)
}

func (s *WritableByteStream) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, s)
}

func (s *ConfluenceStream) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, s)
}

func (r *RingBuffer) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, r)
}

func (c *DataChunk) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, c)
}

func (d *StaticCheckData) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, d)
}

func (d *SymbolicData) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, d)
}

func (m *Module) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, m)
}

func (s *GlobalState) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, s)
}

func (m Message) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, m)
}

func (s *Subscription) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, s)
}

func (p *Publication) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, p)
}

func (h *ValueHistory) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, h)
}

func (h *SynchronousMessageHandler) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, h)
}

func (g *SystemGraph) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, g)
}

func (n *SystemGraphNodes) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, n)
}

func (n *SystemGraphNode) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, n)
}

func (e SystemGraphEvent) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, e)
}

func (e SystemGraphEdge) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, e)
}

func (s *Secret) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, s)
}

func (s *SecretPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, s)
}

func (s *MarkupPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, s)
}

func (s *NonInterpretedMarkupElement) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, s)
}

func (api *ApiIL) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, api)
}

func (ns *Namespace) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if depth > config.MaxDepth && len(ns.names) > 0 {
		utils.Must(w.Write(utils.StringAsBytes("(..namespace..)")))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	utils.Must(w.Write(utils.StringAsBytes("namespace{")))

	for i, k := range ns.names {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

		if config.Colorize {
			utils.Must(w.Write(config.Colors.IdentifierLiteral))

		}

		utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(k))))

		if config.Colorize {
			utils.Must(w.Write(ANSI_RESET_SEQUENCE))

		}

		//colon
		utils.Must(w.Write(COLON_SPACE))

		//value
		v := ns.entries[k]
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(ns.names)-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}
	}

	if !config.Compact && len(ns.names) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
}

func (p *ModuleParamsPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("module-parameters{")))

	//TODO
	utils.Must(w.Write(utils.StringAsBytes("...")))

	utils.PanicIfErr(w.WriteByte('}'))
}

// func (t *CurrentTest) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
// 	InspectPrint(w, t)
// }

// func (p *TestedProgram) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
// 	InspectPrint(w, p)
// }

func (id ULID) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString(id.libValue().String())
}

func (id UUIDv4) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.WriteString(id.libValue().String()))
}

func (Struct) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	w.Write(utils.StringAsBytes("struct-pointer"))
}

func InspectPrint[T any](w *bufio.Writer, v T) {
	utils.Must(fmt.Fprintf(w, "%#v", v))
}

func PrintType[T any](w *bufio.Writer, v T) {
	utils.Must(fmt.Fprintf(w, "%T(...)", v))
}
