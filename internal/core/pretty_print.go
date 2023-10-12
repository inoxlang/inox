package core

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/muesli/termenv"
)

const (
	PRETTY_PRINT_BUFF_WRITER_SIZE = 100
	MAX_VALUE_PRINT_DEPTH         = 10
)

var (
	ANSI_RESET_SEQUENCE = []byte(termenv.CSI + termenv.ResetSeq + "m")

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
	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, PRETTY_PRINT_BUFF_WRITER_SIZE)

	err := PrettyPrint(v, w, &PrettyPrintConfig{
		PrettyPrintConfig: pprint.PrettyPrintConfig{
			MaxDepth: 7,
			Colorize: false,
			Compact:  true,
		},
		Context: ctx,
	}, 0, 0)

	if err != nil {
		panic(fmt.Errorf("failed to stringify value of type %T: %w", v, err))
	}

	w.Flush()
	return buff.String()
}

func PrettyPrint(v Value, w io.Writer, config *PrettyPrintConfig, depth, parentIndentCount int) (err error) {
	buffered, ok := w.(*bufio.Writer)
	if !ok {
		buffered = bufio.NewWriterSize(w, PRETTY_PRINT_BUFF_WRITER_SIZE)
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
	Span          parse.NodeSpan
	ColorSequence []byte
}

func GetNodeColorizations(chunk *parse.Chunk, lightMode bool) []ColorizationInfo {
	var colorizations []ColorizationInfo

	var colors = pprint.DEFAULT_DARKMODE_PRINT_COLORS
	if lightMode {
		colors = pprint.DEFAULT_LIGHTMODE_PRINT_COLORS
	}

	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, _ bool) (parse.TraversalAction, error) {
		switch n := node.(type) {
		//literals
		case *parse.IdentifierLiteral:
			var colorSeq []byte
			if openingElem, ok := parent.(*parse.XMLOpeningElement); ok && openingElem.Name == n {
				colorSeq = colors.XmlTagName
			} else if _, ok := parent.(*parse.XMLClosingElement); ok {
				colorSeq = colors.XmlTagName
			} else {
				colorSeq = colors.IdentifierLiteral
			}

			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colorSeq,
			})

		case *parse.Variable, *parse.GlobalVariable, *parse.AtHostLiteral, *parse.NamedPathSegment:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.IdentifierLiteral,
			})

		case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceIdentifierLiteral:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PatternIdentifier,
			})
		case *parse.StringTemplateInterpolation:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Tokens[0].Span,
				ColorSequence: colors.PatternIdentifier,
			})
		case *parse.QuotedStringLiteral, *parse.UnquotedStringLiteral, *parse.MultilineStringLiteral, *parse.FlagLiteral,
			*parse.RuneLiteral, *parse.StringTemplateSlice:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.StringLiteral,
			})
		case *parse.ByteSliceLiteral:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.StringLiteral,
			})
		case *parse.AbsolutePathPatternLiteral, *parse.RelativePathPatternLiteral, *parse.URLPatternLiteral, *parse.HostPatternLiteral, *parse.RegularExpressionLiteral:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PatternLiteral,
			})
		case *parse.PathSlice:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PathLiteral,
			})
		case *parse.PathPatternSlice:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PatternLiteral,
			})
		case *parse.HostExpression:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Tokens[0].Span,
				ColorSequence: colors.StringLiteral,
			})
		case *parse.URLLiteral, *parse.SchemeLiteral, *parse.HostLiteral, *parse.EmailAddressLiteral, *parse.AbsolutePathLiteral,
			*parse.RelativePathLiteral, *parse.URLQueryParameterValueSlice:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.PathLiteral,
			})
		case *parse.IntLiteral, *parse.FloatLiteral, *parse.QuantityLiteral, *parse.PortLiteral:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.NumberLiteral,
			})
		case *parse.BooleanLiteral, *parse.NilLiteral, *parse.UnambiguousIdentifierLiteral, *parse.PropertyNameLiteral,
			*parse.SelfExpression:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.Constant,
			})
		case *parse.InvalidURLPattern, *parse.InvalidURL, *parse.InvalidAliasRelatedNode, *parse.InvalidComplexStringPatternElement,
			*parse.UnknownNode:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.InvalidNode,
			})
		case *parse.ComplexStringPatternPiece:
			for _, token := range n.Tokens {
				switch token.Type {
				case parse.PERCENT_STR:
					colorizations = append(colorizations, ColorizationInfo{
						Span:          token.Span,
						ColorSequence: colors.PatternIdentifier,
					})
				}
			}
		case *parse.IfStatement, *parse.IfExpression, *parse.SwitchStatement, *parse.MatchStatement, *parse.DefaultCase,
			*parse.ForStatement, *parse.WalkStatement, *parse.ReturnStatement, *parse.BreakStatement, *parse.ContinueStatement,
			*parse.PruneStatement, *parse.YieldStatement, *parse.AssertionStatement, *parse.ComputeExpression:
			for _, tok := range n.Base().Tokens {
				switch tok.Type {
				case parse.OPENING_PARENTHESIS, parse.CLOSING_PARENTHESIS,
					parse.OPENING_CURLY_BRACKET, parse.CLOSING_CURLY_BRACKET:
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          tok.Span,
					ColorSequence: colors.ControlKeyword,
				})
			}
		case *parse.BinaryExpression:
			for _, token := range n.Tokens {
				if token.Type == parse.OPENING_PARENTHESIS || token.Type == parse.CLOSING_PARENTHESIS {
					continue
				}

				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.SpawnExpression:
			for _, token := range n.Tokens {
				if token.Type != parse.GO_KEYWORD && token.Type != parse.DO_KEYWORD {
					continue
				}

				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.ControlKeyword,
				})
			}
		case *parse.MappingExpression, *parse.UDataLiteral:
			for _, tok := range n.Base().Tokens {
				colorizations = append(colorizations, ColorizationInfo{
					Span:          tok.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.MultiAssignment, *parse.GlobalConstantDeclarations, *parse.LocalVariableDeclarations, *parse.ImportStatement,
			*parse.InclusionImportStatement,
			*parse.PermissionDroppingStatement:
			for _, tok := range n.Base().Tokens {
				colorizations = append(colorizations, ColorizationInfo{
					Span:          tok.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.OptionExpression:
			if n.Value == nil {
				break
			}

			if _, isMissingExpr := n.Value.(*parse.MissingExpression); isMissingExpr {
				break
			}

			colorizations = append(colorizations, ColorizationInfo{
				Span:          parse.NodeSpan{Start: n.Span.Start, End: n.Value.Base().Span.Start - 1},
				ColorSequence: colors.StringLiteral,
			})
		case *parse.OptionPatternLiteral:
			if n.Value == nil {
				break
			}

			if _, isMissingExpr := n.Value.(*parse.MissingExpression); isMissingExpr {
				break
			}

			colorizations = append(colorizations, ColorizationInfo{
				Span:          parse.NodeSpan{Start: n.Span.Start, End: n.Value.Base().Span.Start - 1},
				ColorSequence: colors.StringLiteral,
			})
		case *parse.CssTypeSelector:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.CssTypeSelector,
			})
		case *parse.CssIdSelector, *parse.CssClassSelector, *parse.CssPseudoClassSelector, *parse.CssPseudoElementSelector:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.OtherKeyword,
			})
		case *parse.FunctionDeclaration, *parse.FunctionExpression:
			if len(n.Base().Tokens) == 0 {
				break
			}
			for _, token := range n.Base().Tokens {
				if token.Type != parse.FN_KEYWORD {
					continue
				}

				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.FunctionPatternExpression:
			if len(n.Base().Tokens) == 0 {
				break
			}
			for _, token := range n.Tokens {
				if token.Type != parse.PERCENT_FN {
					continue
				}

				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.PatternIdentifier,
				})
			}

		case *parse.PatternGroupName:
			colorizations = append(colorizations, ColorizationInfo{
				Span:          n.Base().Span,
				ColorSequence: colors.Constant,
			})
		case *parse.ConcatenationExpression:
			if len(n.Tokens) == 0 {
				break
			}

			for _, token := range n.Tokens {
				if token.Type != parse.CONCAT_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.TestSuiteExpression, *parse.TestCaseExpression:
			if len(n.Base().Tokens) == 0 {
				break
			}

			for _, token := range n.Base().Tokens {
				if token.Type != parse.TESTSUITE_KEYWORD && token.Type != parse.TESTCASE_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.LifetimejobExpression:
			if len(n.Base().Tokens) == 0 {
				break
			}

			for _, token := range n.Base().Tokens {
				if token.Type != parse.LIFETIMEJOB_KEYWORD && token.Type != parse.FOR_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.SendValueExpression:
			if len(n.Base().Tokens) == 0 {
				break
			}

			for _, token := range n.Base().Tokens {
				if token.Type != parse.SENDVAL_KEYWORD && token.Type != parse.TO_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.SynchronizedBlockStatement:
			for _, token := range n.Base().Tokens {
				if token.Type != parse.SYNCHRONIZED_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.XMLOpeningElement, *parse.XMLClosingElement:
			for _, token := range n.Base().Tokens {
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.DiscreteColor,
				})
			}
		case *parse.ExtendStatement:
			for _, token := range n.Base().Tokens {
				if token.Type == parse.EXTEND_KEYWORD {
					colorizations = append(colorizations, ColorizationInfo{
						Span:          token.Span,
						ColorSequence: colors.OtherKeyword,
					})
				}
			}
		case *parse.PatternDefinition:
			for _, token := range n.Base().Tokens {
				if token.Type == parse.PATTERN_KEYWORD {
					colorizations = append(colorizations, ColorizationInfo{
						Span:          token.Span,
						ColorSequence: colors.OtherKeyword,
					})
				}
			}
		case *parse.PatternNamespaceDefinition:
			for _, token := range n.Base().Tokens {
				if token.Type == parse.PNAMESPACE_KEYWORD {
					colorizations = append(colorizations, ColorizationInfo{
						Span:          token.Span,
						ColorSequence: colors.OtherKeyword,
					})
				}
			}
		}
		return parse.Continue, nil
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

func PrintColorizedChunk(w io.Writer, chunk *parse.Chunk, code []rune, lightMode bool, fgColorSequence []byte) {
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

func GetColorizedChunk(chunk *parse.Chunk, code []rune, lightMode bool, fgColorSequence []byte) string {
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

func (s Str) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
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

	utils.PanicIfErr(w.WriteByte(']'))
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

func (list *IntList) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
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

func (s *Struct) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("struct ")))
	utils.Must(w.Write(utils.StringAsBytes(s.structType.name)))
	utils.PanicIfErr(w.WriteByte('{'))

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	keys := s.structType.keys

	for i, k := range keys {

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
		v := s.values[i]
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(keys)-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}
	}

	if !config.Compact && len(keys) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
}

func (slice *RuneSlice) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, slice)
}

func (slice *ByteSlice) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	var bytes []byte

	if depth > config.MaxDepth && len(slice.Bytes) > 2 {
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
	InspectPrint(w, v)
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
	utils.PanicIfErr(w.WriteByte('='))
	utils.Must(w.Write(ANSI_RESET_SEQUENCE))

	if depth > config.MaxDepth {
		utils.Must(w.Write(utils.StringAsBytes("(...)")))
	} else {
		opt.Value.PrettyPrint(w, config, depth+1, 0)
	}
}

func (pth Path) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	utils.PanicIfErr(pth.WriteRepresentation(config.Context, w, &ReprConfig{AllVisible: true}, 0))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (patt PathPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PatternLiteral))
	}

	utils.PanicIfErr(patt.WriteRepresentation(config.Context, w, &ReprConfig{AllVisible: true}, 0))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (u URL) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	utils.PanicIfErr(u.WriteRepresentation(config.Context, w, &ReprConfig{AllVisible: true}, 0))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (scheme Scheme) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	utils.PanicIfErr(scheme.WriteRepresentation(config.Context, w, &ReprConfig{AllVisible: true}, 0))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (host Host) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	utils.PanicIfErr(host.WriteRepresentation(config.Context, w, &ReprConfig{AllVisible: true}, 0))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (patt HostPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PatternLiteral))
	}

	utils.PanicIfErr(patt.WriteRepresentation(config.Context, w, &ReprConfig{AllVisible: true}, 0))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (addr EmailAddress) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PathLiteral))
	}

	utils.PanicIfErr(addr.WriteRepresentation(config.Context, w, &ReprConfig{AllVisible: false}, 0))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (patt URLPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.PatternLiteral))
	}

	utils.PanicIfErr(patt.WriteRepresentation(config.Context, w, &ReprConfig{AllVisible: true}, 0))

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

func (p PropertyName) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.Constant))
	}

	utils.PanicIfErr(w.WriteByte('.'))
	utils.Must(w.Write(utils.StringAsBytes(p)))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
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

func (rate ByteRate) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(rate.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
}

func (rate SimpleRate) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.Must(rate.write(w))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}
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

func (d Date) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(d.write(w))
}

func (m FileMode) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	s := os.FileMode(m).String()

	utils.Must(w.Write(utils.StringAsBytes(s)))
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
	reprConfig := &ReprConfig{}

	if config.Colorize {
		utils.Must(w.Write(config.Colors.NumberLiteral))
	}

	utils.PanicIfErr(r.WriteRepresentation(config.Context, w, reprConfig, 0))

	if config.Colorize {
		utils.Must(w.Write(ANSI_RESET_SEQUENCE))
	}

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
	InspectPrint(w, pattern)
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
	if depth > config.MaxDepth && len(patt.entryPatterns) > 0 {
		utils.Must(w.Write(utils.StringAsBytes("%{(...)}")))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	utils.Must(w.Write([]byte{'%', '{'}))

	var keys []string
	for k := range patt.entryPatterns {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, k := range keys {

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
		v := patt.entryPatterns[k]
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(keys)-1

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

	if !config.Compact && len(keys) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.Must(w.Write(bytes.Repeat(config.Indent, depth)))
	if err := w.WriteByte('}'); err != nil {
		panic(err)
	}
}

func (patt *RecordPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if depth > config.MaxDepth && len(patt.entryPatterns) > 0 {
		utils.Must(w.Write(utils.StringAsBytes("record(%{(...)})")))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	utils.Must(w.Write(utils.StringAsBytes("record(%{")))

	var keys []string
	for k := range patt.entryPatterns {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, k := range keys {

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
		v := patt.entryPatterns[k]
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(keys)-1

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

	if !config.Compact && len(keys) > 0 {
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

func (b *Bytecode) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, b)
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

func (it *UdataWalker) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *ValueListIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, it)
}

func (it *IntListIterator) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
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
	utils.Must(w.Write([]byte(t.Name())))
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

func (u *UData) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {

	if config.Colorize {
		for _, b := range [][]byte{config.Colors.OtherKeyword, utils.StringAsBytes("udata"), ANSI_RESET_SEQUENCE} {
			utils.Must(w.Write(b))
		}
	}

	if depth > config.MaxDepth {
		utils.Must(w.Write([]byte{'(', '.', '.', '.', ')', '}'}))
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	utils.PanicIfErr(w.WriteByte(' '))

	if u.Root != nil {
		u.Root.PrettyPrint(w, config, depth+1, indentCount)
		utils.Must(w.Write([]byte{' ', '{'}))
	} else {
		utils.PanicIfErr(w.WriteByte('{'))
	}

	for _, entry := range u.HiearchyEntries {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

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

func (e UDataHiearchyEntry) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	e.Value.PrettyPrint(w, config, depth+1, indentCount)

	if len(e.Children) > 0 {
		utils.Must(w.Write([]byte{' ', '{'}))

		for _, entry := range e.Children {

			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))
			}

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

	Str(c.GetOrBuildString()).PrettyPrint(w, config, depth, parentIndentCount)
}

func (c *BytesConcatenation) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	//TODO: improve implementation

	c.GetOrBuildBytes().PrettyPrint(w, config, depth, parentIndentCount)
}

func (s *TestSuite) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, s)
}

func (c *TestCase) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, c)
}

func (r *TestCaseResult) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	if r.Success {
		w.Write(utils.StringAsBytes(r.Message))
	} else {
		w.Write(utils.StringAsBytes(r.Message))
	}
}

func (d *DynamicValue) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write([]byte{'d', 'y', 'n', '('}))

	d.Resolve(config.Context).PrettyPrint(w, config, depth, 0)

	utils.Must(w.Write([]byte{')'}))

}

func (e *Event) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, e)
}

func (s *ExecutedStep) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, s)
}

func (j *LifetimeJob) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, j)
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

func (c Color) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	InspectPrint(w, c)
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

func (s *XMLElement) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, s)
}

func (db *DatabaseIL) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	PrintType(w, db)
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

func (p *StructPattern) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("struct-type {")))

	//TODO
	utils.Must(w.Write(utils.StringAsBytes("...")))

	utils.PanicIfErr(w.WriteByte('}'))
}

func InspectPrint[T any](w *bufio.Writer, v T) {
	utils.Must(fmt.Fprintf(w, "%#v", v))
}

func PrintType[T any](w *bufio.Writer, v T) {
	utils.Must(fmt.Fprintf(w, "%T(...)", v))
}
