package internal

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/muesli/termenv"
)

type PrettyPrintColors struct {
	ControlKeyword, OtherKeyword, PatternLiteral, StringLiteral, PathLiteral, IdentifierLiteral,
	NumberLiteral, Constant, PatternIdentifier, CssTypeSelector, CssOtherSelector, InvalidNode, Index []byte
}

type PrettyPrintConfig struct {
	MaxDepth                    int
	Colorize                    bool
	Colors                      *PrettyPrintColors
	Compact                     bool
	Indent                      []byte
	Context                     *Context
	PrintDecodedTopLevelStrings bool
}

func (config *PrettyPrintConfig) WithContext(ctx *Context) *PrettyPrintConfig {
	newConfig := *config
	newConfig.Context = ctx
	return &newConfig
}

func GetFullColorSequence(color termenv.Color, bg bool) []byte {
	var b = []byte(termenv.CSI)
	b = append(b, []byte(color.Sequence(bg))...)
	b = append(b, 'm')
	return b
}

const (
	MAX_VALUE_PRINT_DEPTH = 10
)

var (
	ANSI_RESET_SEQUENCE = []byte(termenv.CSI + termenv.ResetSeq + "m")

	DEFAULT_DARKMODE_PRINT_COLORS = PrettyPrintColors{
		ControlKeyword:    GetFullColorSequence(termenv.ANSIBrightMagenta, false),
		OtherKeyword:      GetFullColorSequence(termenv.ANSIBlue, false),
		PatternLiteral:    GetFullColorSequence(termenv.ANSIRed, false),
		StringLiteral:     GetFullColorSequence(termenv.ANSI256Color(209), false),
		PathLiteral:       GetFullColorSequence(termenv.ANSI256Color(209), false),
		IdentifierLiteral: GetFullColorSequence(termenv.ANSIBrightCyan, false),
		NumberLiteral:     GetFullColorSequence(termenv.ANSIBrightGreen, false),
		Constant:          GetFullColorSequence(termenv.ANSIBlue, false),
		PatternIdentifier: GetFullColorSequence(termenv.ANSIBrightGreen, false),
		CssTypeSelector:   GetFullColorSequence(termenv.ANSIBlack, false),
		CssOtherSelector:  GetFullColorSequence(termenv.ANSIYellow, false),
		InvalidNode:       GetFullColorSequence(termenv.ANSIBrightRed, false),
		Index:             GetFullColorSequence(termenv.ANSIBrightBlack, false),
	}

	DEFAULT_LIGHTMODE_PRINT_COLORS = PrettyPrintColors{
		ControlKeyword:    GetFullColorSequence(termenv.ANSI256Color(90), false),
		OtherKeyword:      GetFullColorSequence(termenv.ANSI256Color(26), false),
		PatternLiteral:    GetFullColorSequence(termenv.ANSI256Color(1), false),
		StringLiteral:     GetFullColorSequence(termenv.ANSI256Color(88), false),
		PathLiteral:       GetFullColorSequence(termenv.ANSI256Color(88), false),
		IdentifierLiteral: GetFullColorSequence(termenv.ANSI256Color(27), false),
		NumberLiteral:     GetFullColorSequence(termenv.ANSI256Color(22), false),
		Constant:          GetFullColorSequence(termenv.ANSI256Color(21), false),
		PatternIdentifier: GetFullColorSequence(termenv.ANSI256Color(22), false),
		CssTypeSelector:   GetFullColorSequence(termenv.ANSIBlack, false),
		CssOtherSelector:  GetFullColorSequence(termenv.ANSIYellow, false),
		InvalidNode:       GetFullColorSequence(termenv.ANSI256Color(160), false),
		Index:             GetFullColorSequence(termenv.ANSIBrightBlack, false),
	}

	QUOTED_BELL_RUNE   = []byte("'\\b'")
	QUOTED_FFEED_RUNE  = []byte("'\\f'")
	QUOTED_NL_RUNE     = []byte("'\\n'")
	QUOTED_CR_RUNE     = []byte("'\\r'")
	QUOTED_TAB_RUNE    = []byte("'\\t'")
	QUOTED_VTAB_RUNE   = []byte("'\\v'")
	QUOTED_SQUOTE_RUNE = []byte("'\\''")
	QUOTED_ASLASH_RUNE = []byte("'\\\\'")
)

// Stringify calls PrettyPrint on the passed value
func Stringify(v Value, ctx *Context) string {
	buff := bytes.Buffer{}
	_, err := v.PrettyPrint(&buff, &PrettyPrintConfig{
		MaxDepth: 7,
		Colorize: false,
		Compact:  true,
		Context:  ctx,
	}, 0, 0)

	if err != nil {
		panic(err)
	}

	return buff.String()
}

type ColorizationInfo struct {
	Span          parse.NodeSpan
	ColorSequence []byte
}

func GetNodeColorizations(chunk *parse.Chunk, lightMode bool) []ColorizationInfo {
	var colorizations []ColorizationInfo

	var colors = DEFAULT_DARKMODE_PRINT_COLORS
	if lightMode {
		colors = DEFAULT_LIGHTMODE_PRINT_COLORS
	}

	parse.Walk(chunk, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, _ bool) (parse.TraversalAction, error) {
		switch n := node.(type) {
		//literals
		case *parse.IdentifierLiteral, *parse.Variable, *parse.GlobalVariable, *parse.AtHostLiteral, *parse.NamedPathSegment:
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
				Span:          n.ValuelessTokens[0].Span,
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
				Span:          n.ValuelessTokens[0].Span,
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
			*parse.SelfExpression, *parse.SupersysExpression:
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
			// other nodes
		case *parse.IfStatement, *parse.IfExpression, *parse.SwitchStatement, *parse.MatchStatement, *parse.ForStatement, *parse.WalkStatement,
			*parse.ReturnStatement, *parse.BreakStatement, *parse.ContinueStatement, *parse.PruneStatement, *parse.YieldStatement,
			*parse.AssertionStatement, *parse.ComputeExpression:
			for _, tok := range n.Base().ValuelessTokens {
				if tok.Type == parse.OPENING_PARENTHESIS || tok.Type == parse.CLOSING_PARENTHESIS {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          tok.Span,
					ColorSequence: colors.ControlKeyword,
				})
			}
		case *parse.SpawnExpression:
			for _, token := range n.ValuelessTokens {
				if token.Type != parse.GO_KEYWORD && token.Type != parse.DO_KEYWORD {
					continue
				}

				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.ControlKeyword,
				})
			}
		case *parse.MappingExpression, *parse.UDataLiteral:
			for _, tok := range n.Base().ValuelessTokens {
				colorizations = append(colorizations, ColorizationInfo{
					Span:          tok.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.MultiAssignment, *parse.GlobalConstantDeclarations, *parse.LocalVariableDeclarations, *parse.ImportStatement,
			*parse.InclusionImportStatement,
			*parse.PermissionDroppingStatement:
			for _, tok := range n.Base().ValuelessTokens {
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
			if len(n.Base().ValuelessTokens) == 0 {
				break
			}
			for _, token := range n.Base().ValuelessTokens {
				if token.Type != parse.FN_KEYWORD {
					continue
				}

				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.FunctionPatternExpression:
			if len(n.Base().ValuelessTokens) == 0 {
				break
			}
			for _, token := range n.ValuelessTokens {
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
			if len(n.ValuelessTokens) == 0 {
				break
			}

			for _, token := range n.ValuelessTokens {
				if token.Type != parse.CONCAT_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.TestSuiteExpression, *parse.TestCaseExpression:
			if len(n.Base().ValuelessTokens) == 0 {
				break
			}

			for _, token := range n.Base().ValuelessTokens {
				if token.Type != parse.TESTSUITE_KEYWORD && token.Type != parse.TESTCASE_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.LifetimejobExpression:
			if len(n.Base().ValuelessTokens) == 0 {
				break
			}

			for _, token := range n.Base().ValuelessTokens {
				if token.Type != parse.LIFETIMEJOB_KEYWORD && token.Type != parse.FOR_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.SendValueExpression:
			if len(n.Base().ValuelessTokens) == 0 {
				break
			}

			for _, token := range n.Base().ValuelessTokens {
				if token.Type != parse.SENDVAL_KEYWORD && token.Type != parse.TO_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
			}
		case *parse.SynchronizedBlockStatement:
			for _, token := range n.Base().ValuelessTokens {
				if token.Type != parse.SYNCHRONIZED_KEYWORD {
					continue
				}
				colorizations = append(colorizations, ColorizationInfo{
					Span:          token.Span,
					ColorSequence: colors.OtherKeyword,
				})
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
		w.Write([]byte(string(code[colorization.Span.Start:utils.Min(len(code), int(colorization.Span.End))])))
		w.Write(ANSI_RESET_SEQUENCE)

		prevColorizationEndIndex = int(colorization.Span.End)
	}

	if prevColorizationEndIndex < len(code) {
		w.Write([]byte(string(code[prevColorizationEndIndex:])))
	}
}

func (n AstNode) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", n)
}

func (t Token) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", t)
}

func (Nil NilT) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.Constant, {'n', 'i', 'l'}, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	} else {
		return w.Write([]byte{'n', 'i', 'l'})
	}
}

func (err Error) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "(%T)%s", err.goError, err.goError.Error())
}

func (boolean Bool) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	var b = []byte{'f', 'a', 'l', 's', 'e'}
	if boolean {
		b = []byte{'t', 'r', 'u', 'e'}
	}

	if config.Colorize {
		var totalN int

		for _, b := range [][]byte{config.Colors.Constant, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (r Rune) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	totalN := 0
	var err error

	if config.Colorize {
		totalN, err = w.Write(config.Colors.StringLiteral)
		if err != nil {
			return totalN, err
		}
	}

	n, err := w.Write(r.reprBytes())

	totalN += n
	if err != nil {
		return totalN, err
	}

	if config.Colorize {
		n, err = w.Write(ANSI_RESET_SEQUENCE)
		totalN += n
	}

	return totalN, err
}

func (b Byte) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", b)
}

func (i Int) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	var b []byte
	b = strconv.AppendInt(b, int64(i), 10)

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.NumberLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (f Float) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	var b []byte
	b = strconv.AppendFloat(b, float64(f), 'f', -1, 64)

	if !bytes.Contains(b, []byte{'.'}) {
		b = append(b, '.', '0')
	}

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.NumberLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (s Str) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if depth == 0 && config.PrintDecodedTopLevelStrings {
		return w.Write([]byte(utils.StripANSISequences(string(s))))
	}

	jsonStr, err := MarshalJsonNoHTMLEspace(string(s))
	if err != nil {
		return 0, err
	}

	if depth > config.MaxDepth && len(jsonStr) > 3 {
		//TODO: fix cut
		jsonStr = jsonStr[:3]
		jsonStr = append(jsonStr, '.', '.', '.', '"')
	}

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.StringLiteral, jsonStr, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(jsonStr)
}

func (obj *Object) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	closestState := config.Context.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	//TODO: prevent modification of the object while this function is running

	if depth > config.MaxDepth && len(obj.keys) > 0 {
		return w.Write([]byte{'{', '(', '.', '.', '.', ')', '}'})
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	var n int
	totalN, err := w.Write([]byte{'{'})
	if err != nil {
		return totalN, err
	}

	for i, k := range obj.keys {

		if !config.Compact {
			b := []byte{'\n', '\r'}
			b = append(b, indent...)

			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		if config.Colorize {
			n, err = w.Write(config.Colors.IdentifierLiteral)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		n, err = w.Write(utils.Must(MarshalJsonNoHTMLEspace(k)))
		totalN += n
		if err != nil {
			return totalN, err
		}

		if config.Colorize {
			n, err = w.Write(ANSI_RESET_SEQUENCE)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		//colon
		n, err = w.Write([]byte{':', ' '})
		totalN += n
		if err != nil {
			return totalN, err
		}

		//value
		v := obj.values[i]
		n, err = v.PrettyPrint(w, config, depth+1, indentCount)
		totalN += n
		if err != nil {
			return totalN, err
		}

		//comma & indent
		isLastEntry := i == len(obj.keys)-1

		if !isLastEntry {
			n, err = w.Write([]byte{',', ' '})
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
	}

	var end []byte
	if !config.Compact && len(obj.keys) > 0 {
		end = append(end, '\n', '\r')
	}
	end = append(end, bytes.Repeat(config.Indent, depth)...)
	end = append(end, '}')

	n, err = w.Write(end)
	totalN += n
	return totalN, err
}

func (rec Record) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	obj := objFrom(rec.EntryMap())
	totalN, err := w.Write([]byte{'#'})
	if err != nil {
		return totalN, err
	}

	n, err := obj.PrettyPrint(w, config, depth, parentIndentCount)
	return totalN + n, err
}

func (dict *Dictionary) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	//TODO: prevent modification of the dictionary while this function is running

	if depth > config.MaxDepth && len(dict.Entries) > 0 {
		return w.Write([]byte{':', '{', '(', '.', '.', '.', ')', '}'})
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	var n int
	totalN, err := w.Write([]byte{':', '{'})
	if err != nil {
		return totalN, err
	}

	var keys []string
	for k := range dict.Entries {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, k := range keys {
		if !config.Compact {

			b := []byte{'\n', '\r'}
			b = append(b, indent...)

			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		var n int
		var err error

		//key
		if config.Colorize {
			n, err = w.Write(config.Colors.StringLiteral)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		n, err = w.Write([]byte(k))
		totalN += n
		if err != nil {
			return totalN, err
		}

		if config.Colorize {
			n, err = w.Write(ANSI_RESET_SEQUENCE)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		//colon
		n, err = w.Write([]byte{':', ' '})
		totalN += n
		if err != nil {
			return totalN, err
		}

		//value
		v := dict.Entries[k]

		n, err = v.PrettyPrint(w, config, depth+1, indentCount)
		totalN += n
		if err != nil {
			return totalN, err
		}

		//comma & indent
		isLastEntry := i == len(keys)-1

		if !isLastEntry {
			n, err = w.Write([]byte{',', ' '})
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

	}

	var end []byte

	if !config.Compact && len(keys) > 0 {
		end = append(end, '\n', '\r')
	}
	end = append(end, bytes.Repeat(config.Indent, depth)...)
	end = append(end, '}')

	n, err = w.Write(end)
	totalN += n
	return totalN, err
}

func (list KeyList) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	//TODO: prevent modification of the key list while this function is running

	if depth > config.MaxDepth && len(list) > 0 {
		return w.Write([]byte{'.', '{', '(', '.', '.', '.', ')', '}'})
	}

	totalN, err := w.Write([]byte{'.', '{'})
	if err != nil {
		return totalN, err
	}

	first := true

	for _, k := range list {
		if !first {
			n, err := w.Write([]byte{',', ' '})
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		first = false

		n, err := w.Write([]byte(k))
		totalN += n
		if err != nil {
			return totalN, err
		}
	}

	n, err := w.Write([]byte{'}'})
	totalN += n
	return totalN, err
}

func PrettyPrintList(list underylingList, w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	//TODO: prevent modification of the list while this function is running
	length := list.Len()

	if depth > config.MaxDepth && length > 0 {
		return w.Write([]byte{'[', '(', '.', '.', '.', ')', ']'})
	}

	var n int
	totalN, err := w.Write([]byte{'['})
	if err != nil {
		return totalN, err
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)
	printIndices := !config.Compact && length > 10

	for i := 0; i < length; i++ {
		v := list.At(config.Context, i)

		if !config.Compact {

			b := []byte{'\n', '\r'}
			b = append(b, indent...)

			//index
			if printIndices {
				if config.Colorize {
					b = append(b, config.Colors.Index...)
				}
				if i < 10 {
					b = append(b, ' ')
				}
				b = append(b, strconv.FormatInt(int64(i), 10)...)
				b = append(b, ':', ' ')
				if config.Colorize {
					b = append(b, ANSI_RESET_SEQUENCE...)
				}
			}

			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		//element
		n, err := v.PrettyPrint(w, config, depth+1, indentCount)
		totalN += n
		if err != nil {
			return totalN, err
		}

		//comma & indent
		isLastEntry := i == length-1

		if !isLastEntry {
			n, err = w.Write([]byte{',', ' '})
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

	}

	var end []byte
	if !config.Compact && length > 0 {
		end = append(end, '\n', '\r')
	}
	end = append(end, bytes.Repeat(config.Indent, depth)...)
	end = append(end, ']')

	n, err = w.Write(end)
	totalN += n
	return totalN, err
}

func (list *List) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return list.underylingList.PrettyPrint(w, config, depth, parentIndentCount)
}

func (list *ValueList) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return PrettyPrintList(list, w, config, depth, parentIndentCount)
}

func (list *IntList) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return PrettyPrintList(list, w, config, depth, parentIndentCount)
}

func (tuple Tuple) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	lst := &List{underylingList: &ValueList{elements: tuple.elements}}
	totalN, err := w.Write([]byte{'#'})
	if err != nil {
		return totalN, err
	}
	n, err := lst.PrettyPrint(w, config, depth, parentIndentCount)
	return totalN + n, err
}

func (slice *RuneSlice) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", slice)
}

func (slice *ByteSlice) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	var bytes []byte
	if depth > config.MaxDepth && len(slice.Bytes) > 2 {
		//TODO: fix cut
		bytes = []byte("0x[...]")
		if !config.Colorize {
			return w.Write(bytes)
		}
	}

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.StringLiteral, bytes, ANSI_RESET_SEQUENCE} {
			if b == nil {
				n, err := slice.write(w)
				totalN += n
				if err != nil {
					return totalN, err
				}
			} else {
				n, err := w.Write(b)
				totalN += n
				if err != nil {
					return totalN, err
				}
			}
		}
		return totalN, nil
	}

	return slice.write(w)
}

func (v *GoFunction) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", v)
}

func (opt Option) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	totalN := 0
	var n int
	var err error

	namePart := make([]byte, 3+len(opt.Name))

	if config.Colorize {
		namePart = append(namePart, config.Colors.StringLiteral...)
	}

	if len(opt.Name) <= 1 {
		namePart = append(namePart, '-')
	} else {
		namePart = append(namePart, '-', '-')
	}

	namePart = append(namePart, opt.Name...)
	namePart = append(namePart, '=')
	namePart = append(namePart, ANSI_RESET_SEQUENCE...)
	n, err = w.Write(namePart)
	totalN += n

	if err != nil {
		return totalN, err
	}

	if depth > config.MaxDepth {
		n, err = w.Write([]byte{'(', '.', '.', '.', ')'})
	} else {
		n, err = opt.Value.PrettyPrint(w, config, depth+1, 0)
	}

	totalN += n
	return totalN, err
}

func (pth Path) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte(GetRepresentation(pth, config.Context))

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.PathLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (patt PathPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte(GetRepresentation(patt, config.Context))

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.PatternLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (u URL) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte(GetRepresentation(u, config.Context))

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.PathLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (scheme Scheme) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte(GetRepresentation(scheme, config.Context))

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.PathLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (host Host) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte(GetRepresentation(host, config.Context))

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.PathLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (patt HostPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte(GetRepresentation(patt, config.Context))

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.PatternLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (addr EmailAddress) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte(GetRepresentation(addr, config.Context))

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.PathLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (patt URLPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte(GetRepresentation(patt, config.Context))

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.PatternLiteral, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (i Identifier) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte{'#', '('}
	b = append(b, i...)
	b = append(b, ')')

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.Constant, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (p PropertyName) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	b := []byte{'.'}
	b = append(b, p...)

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.Constant, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (str CheckedString) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	patt := []byte{'%'}
	patt = append(patt, str.matchingPatternName...)

	s := []byte{'`'}
	jsonStr, _ := MarshalJsonNoHTMLEspace(str.str)
	s = append(s, jsonStr[1:len(jsonStr)-1]...)
	s = append(s, '`')

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{
			config.Colors.PatternIdentifier, patt, ANSI_RESET_SEQUENCE, config.Colors.StringLiteral, s, ANSI_RESET_SEQUENCE,
		} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	totalN, err := w.Write(patt)
	if err != nil {
		return totalN, err
	}
	n, err := w.Write(s)
	totalN += n
	return totalN, err
}

func (count ByteCount) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if config.Colorize {
		var (
			totalN, n int
			err       error
		)
		for _, b := range [][]byte{config.Colors.NumberLiteral, nil, ANSI_RESET_SEQUENCE} {
			if b == nil {
				n, err = count.write(w)
			} else {
				n, err = w.Write(b)
			}
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}
	return count.write(w)
}

func (count LineCount) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if config.Colorize {
		var (
			totalN, n int
			err       error
		)
		for _, b := range [][]byte{config.Colors.NumberLiteral, nil, ANSI_RESET_SEQUENCE} {
			if b == nil {
				n, err = count.write(w)
			} else {
				n, err = w.Write(b)
			}
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return count.write(w)
}

func (count RuneCount) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if config.Colorize {
		var (
			totalN, n int
			err       error
		)
		for _, b := range [][]byte{config.Colors.NumberLiteral, nil, ANSI_RESET_SEQUENCE} {
			if b == nil {
				n, err = count.write(w)
			} else {
				n, err = w.Write(b)
			}
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return count.write(w)
}

func (rate ByteRate) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if config.Colorize {
		var (
			totalN, n int
			err       error
		)
		for _, b := range [][]byte{config.Colors.NumberLiteral, nil, ANSI_RESET_SEQUENCE} {
			if b == nil {
				n, err = rate.write(w)
			} else {
				n, err = w.Write(b)
			}
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return rate.write(w)
}

func (rate SimpleRate) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if config.Colorize {
		var (
			totalN, n int
			err       error
		)
		for _, b := range [][]byte{config.Colors.NumberLiteral, nil, ANSI_RESET_SEQUENCE} {
			if b == nil {
				n, err = rate.write(w)
			} else {
				n, err = w.Write(b)
			}
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return rate.write(w)
}

func (d Duration) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if config.Colorize {
		var (
			totalN, n int
			err       error
		)
		for _, b := range [][]byte{config.Colors.NumberLiteral, nil, ANSI_RESET_SEQUENCE} {
			if b == nil {
				n, err = d.write(w)
			} else {
				n, err = w.Write(b)
			}
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return d.write(w)
}

func (d Date) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return d.write(w)
}

func (m FileMode) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", m)
}

func (r RuneRange) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if config.Colorize {
		var (
			totalN, n int
			err       error
		)
		for _, b := range [][]byte{config.Colors.NumberLiteral, nil, ANSI_RESET_SEQUENCE} {
			if b == nil {
				n, err = r.write(w)
			} else {
				n, err = w.Write(b)
			}
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return r.write(w)
}

func (r QuantityRange) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	reprConfig := &ReprConfig{}

	if r.HasRepresentation(nil, reprConfig) {
		if config.Colorize {
			var (
				totalN, n int
				err       error
			)
			for _, b := range [][]byte{config.Colors.NumberLiteral, nil, ANSI_RESET_SEQUENCE} {
				if b == nil {
					buff := &bytes.Buffer{}
					if err := r.WriteRepresentation(config.Context, buff, nil, reprConfig); err != nil {
						return buff.Len(), err
					}
					n, err = w.Write(buff.Bytes())
				} else {
					n, err = w.Write(b)
				}
				totalN += n
				if err != nil {
					return totalN, err
				}
			}
			return totalN, nil
		}
	}

	return fmt.Fprintf(w, "%#v", r)
}

func (r IntRange) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	if config.Colorize {
		var (
			totalN, n int
			err       error
		)
		for _, b := range [][]byte{config.Colors.NumberLiteral, nil, ANSI_RESET_SEQUENCE} {
			if b == nil {
				n, err = r.write(w)
			} else {
				n, err = w.Write(b)
			}
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return r.write(w)
}

//patterns

func (pattern ExactValuePattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", pattern)
}

func (pattern TypePattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	var b = []byte{'%'}
	b = append(b, pattern.Name...)

	if config.Colorize {
		var totalN int
		for _, b := range [][]byte{config.Colors.PatternIdentifier, b, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(b)
}

func (pattern *DifferencePattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", pattern)
}

func (pattern *OptionalPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", pattern)
}

func (pattern *FunctionPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", pattern)
}

func (patt *RegexPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *UnionPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *IntersectionPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *SequenceStringPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *UnionStringPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *RuneRangeStringPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *IntRangePattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *DynamicStringPatternElement) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *RepeatedPatternElement) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *NamedSegmentPathPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt ObjectPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {

	if depth > config.MaxDepth && len(patt.entryPatterns) > 0 {
		return w.Write([]byte{'%', '{', '(', '.', '.', '.', ')', '}'})
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	var n int
	totalN, err := w.Write([]byte{'%', '{'})
	if err != nil {
		return totalN, err
	}

	var keys []string
	for k := range patt.entryPatterns {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, k := range keys {

		if !config.Compact {
			b := []byte{'\n', '\r'}
			b = append(b, indent...)

			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		if config.Colorize {
			n, err = w.Write(config.Colors.IdentifierLiteral)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		n, err = w.Write(utils.Must(MarshalJsonNoHTMLEspace(k)))
		totalN += n
		if err != nil {
			return totalN, err
		}

		if config.Colorize {
			n, err = w.Write(ANSI_RESET_SEQUENCE)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		//colon
		n, err = w.Write([]byte{':', ' '})
		totalN += n
		if err != nil {
			return totalN, err
		}

		//value
		v := patt.entryPatterns[k]
		n, err = v.PrettyPrint(w, config, depth+1, indentCount)
		totalN += n
		if err != nil {
			return totalN, err
		}

		//comma & indent
		isLastEntry := i == len(keys)-1

		if !isLastEntry || patt.inexact {
			n, err = w.Write([]byte{',', ' '})
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
	}

	if patt.inexact {
		if !config.Compact {
			b := []byte{'\n', '\r'}
			b = append(b, indent...)

			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		n, err = w.Write([]byte{'.', '.', '.'})
		totalN += n
		if err != nil {
			return totalN, err
		}
	}

	var end []byte
	if !config.Compact && len(keys) > 0 {
		end = append(end, '\n', '\r')
	}
	end = append(end, bytes.Repeat(config.Indent, depth)...)
	end = append(end, '}')

	n, err = w.Write(end)
	totalN += n
	return totalN, err
}

func (patt *RecordPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {

	if depth > config.MaxDepth && len(patt.entryPatterns) > 0 {
		return w.Write([]byte{'r', 'e', 'c', 'o', 'r', 'd', '(', '%', '{', '(', '.', '.', '.', ')', '}'})
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	var n int
	totalN, err := w.Write([]byte{'r', 'e', 'c', 'o', 'r', 'd', '(', '%', '{'})
	if err != nil {
		return totalN, err
	}

	var keys []string
	for k := range patt.entryPatterns {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, k := range keys {

		if !config.Compact {
			b := []byte{'\n', '\r'}
			b = append(b, indent...)

			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		if config.Colorize {
			n, err = w.Write(config.Colors.IdentifierLiteral)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		n, err = w.Write(utils.Must(MarshalJsonNoHTMLEspace(k)))
		totalN += n
		if err != nil {
			return totalN, err
		}

		if config.Colorize {
			n, err = w.Write(ANSI_RESET_SEQUENCE)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		//colon
		n, err = w.Write([]byte{':', ' '})
		totalN += n
		if err != nil {
			return totalN, err
		}

		//value
		v := patt.entryPatterns[k]
		n, err = v.PrettyPrint(w, config, depth+1, indentCount)
		totalN += n
		if err != nil {
			return totalN, err
		}

		//comma & indent
		isLastEntry := i == len(keys)-1

		if !isLastEntry || patt.inexact {
			n, err = w.Write([]byte{',', ' '})
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
	}

	if patt.inexact {
		if !config.Compact {
			b := []byte{'\n', '\r'}
			b = append(b, indent...)

			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		n, err = w.Write([]byte{'.', '.', '.'})
		totalN += n
		if err != nil {
			return totalN, err
		}
	}

	var end []byte
	if !config.Compact && len(keys) > 0 {
		end = append(end, '\n', '\r')
	}
	end = append(end, bytes.Repeat(config.Indent, depth)...)
	end = append(end, '}', ')')

	n, err = w.Write(end)
	totalN += n
	return totalN, err
}

func prettyPrintListPattern(
	w io.Writer, tuplePattern bool,
	generalElementPattern Pattern, elementPatterns []Pattern,
	config *PrettyPrintConfig, depth int, parentIndentCount int,

) (int, error) {

	if generalElementPattern != nil {
		b := utils.StringAsBytes("%[]")

		if tuplePattern {
			b = utils.StringAsBytes("%tuple(")
		}

		var n int
		totalN, err := w.Write(b)
		if err != nil {
			return totalN, err
		}
		n, err = generalElementPattern.PrettyPrint(w, config, depth, parentIndentCount)
		totalN += n

		if err != nil {
			return totalN, err
		}

		if tuplePattern {
			n, err := w.Write(utils.StringAsBytes(")"))
			if err != nil {
				return totalN, err
			}
			totalN += n
		}

		return totalN, err
	}

	if depth > config.MaxDepth && len(elementPatterns) > 0 {
		b := utils.StringAsBytes("%[(...)]")
		if tuplePattern {
			b = utils.StringAsBytes("%tuple(...)")
		}

		return w.Write(b)
	}

	var n int
	start := utils.StringAsBytes("%[")
	if tuplePattern {
		start = utils.StringAsBytes("%tuple(%[")
	}
	totalN, err := w.Write(start)
	if err != nil {
		return totalN, err
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)
	printIndices := !config.Compact && len(elementPatterns) > 10

	for i, v := range elementPatterns {

		if !config.Compact {

			b := []byte{'\n', '\r'}
			b = append(b, indent...)

			//index
			if printIndices {
				if config.Colorize {
					b = append(b, config.Colors.Index...)
				}
				if i < 10 {
					b = append(b, ' ')
				}
				b = append(b, strconv.FormatInt(int64(i), 10)...)
				b = append(b, ':', ' ')
				if config.Colorize {
					b = append(b, ANSI_RESET_SEQUENCE...)
				}
			}

			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		//element
		n, err := v.PrettyPrint(w, config, depth+1, indentCount)
		totalN += n
		if err != nil {
			return totalN, err
		}

		//comma & indent
		isLastEntry := i == len(elementPatterns)-1

		if !isLastEntry {
			n, err = w.Write([]byte{',', ' '})
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

	}

	var end []byte
	if !config.Compact && len(elementPatterns) > 0 {
		end = append(end, '\n', '\r')
	}
	end = append(end, bytes.Repeat(config.Indent, depth)...)
	if tuplePattern {
		end = append(end, "])"...)
	} else {
		end = append(end, "]"...)
	}

	n, err = w.Write(end)
	totalN += n
	return totalN, err
}

func (patt ListPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return prettyPrintListPattern(w, false, patt.generalElementPattern, patt.elementPatterns, config, depth, parentIndentCount)
}

func (patt TuplePattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return prettyPrintListPattern(w, true, patt.generalElementPattern, patt.elementPatterns, config, depth, parentIndentCount)
}

func (patt OptionPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *EventPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *MutationPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *ParserBasedPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (patt *IntRangeStringPattern) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", patt)
}

func (reader *Reader) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", reader)
}

func (writer *Writer) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", writer)
}

func (mt Mimetype) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", mt)
}
func (i FileInfo) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", i)
}

func (r *Routine) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", r)
}

func (g *RoutineGroup) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", g)
}

func (g *InoxFunction) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", g)
}

func (b *Bytecode) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", b)
}

func (it *KeyFilteredIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *ValueFilteredIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *KeyValueFilteredIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *indexableIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *immutableSliceIterator[T]) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it IntRangeIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it RuneRangeIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *PatternIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it indexedEntryIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *IpropsIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *EventSourceIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *DirWalker) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *ValueListIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *IntListIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (it *TupleIterator) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (t Type) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return w.Write([]byte(t.Name()))
}

func (tx *Transaction) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", tx)
}

func (r *RandomnessSource) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", r)
}

func (m *Mapping) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", m)
}

func (ns *PatternNamespace) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", ns)
}

func (port Port) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	repr := port.repr(false)
	if config.Colorize {
		var (
			totalN, n int
			err       error
		)
		for _, b := range [][]byte{config.Colors.NumberLiteral, repr, ANSI_RESET_SEQUENCE} {
			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
		return totalN, nil
	}

	return w.Write(repr)
}

func (u *UData) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {

	var (
		totalN int
		err    error
	)

	if config.Colorize {
		for _, b := range [][]byte{config.Colors.OtherKeyword, {'u', 'd', 'a', 't', 'a'}, ANSI_RESET_SEQUENCE} {
			n, err := w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}
	}

	if depth > config.MaxDepth {
		n, err := w.Write([]byte{'(', '.', '.', '.', ')', '}'})
		totalN += n
		return totalN, err
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	n, err := w.Write([]byte{' '})
	totalN += n

	if err != nil {
		return totalN, err
	}

	if u.Root != nil {
		n, err = u.Root.PrettyPrint(w, config, depth+1, indentCount)
		totalN += n
		if err != nil {
			return totalN, err
		}
		n, err = w.Write([]byte{' ', '{'})
		totalN += n
		if err != nil {
			return totalN, err
		}
	} else {
		n, err = w.Write([]byte{'{'})
		totalN += n
		if err != nil {
			return totalN, err
		}
	}

	for _, entry := range u.HiearchyEntries {

		if !config.Compact {
			b := []byte{'\n', '\r'}
			b = append(b, indent...)

			n, err = w.Write(b)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		if config.Colorize {
			n, err = w.Write(config.Colors.IdentifierLiteral)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		n, err = entry.PrettyPrint(w, config, depth+1, indentCount)
		totalN += n
		if err != nil {
			return totalN, err
		}
	}

	var end []byte
	if !config.Compact && len(u.HiearchyEntries) > 0 {
		end = append(end, '\n', '\r')
	}
	end = append(end, bytes.Repeat(config.Indent, depth)...)
	end = append(end, '}')

	n, err = w.Write(end)
	totalN += n
	return totalN, err
}

func (e UDataHiearchyEntry) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	totalN, err := e.Value.PrettyPrint(w, config, depth+1, indentCount)
	if err != nil {
		return totalN, err
	}

	if len(e.Children) > 0 {
		n, err := w.Write([]byte{' ', '{'})
		totalN += n
		if err != nil {
			return totalN, err
		}

		for _, entry := range e.Children {

			if !config.Compact {
				b := []byte{'\n', '\r'}
				b = append(b, indent...)

				n, err = w.Write(b)
				totalN += n
				if err != nil {
					return totalN, err
				}
			}

			if config.Colorize {
				n, err = w.Write(config.Colors.IdentifierLiteral)
				totalN += n
				if err != nil {
					return totalN, err
				}
			}

			n, err = entry.PrettyPrint(w, config, depth+1, indentCount)
			totalN += n
			if err != nil {
				return totalN, err
			}
		}

		var end []byte
		if !config.Compact && len(e.Children) > 0 {
			end = append(end, '\n', '\r')
		}
		end = append(end, bytes.Repeat(config.Indent, depth)...)
		end = append(end, '}')

		n, err = w.Write(end)
		totalN += n
		return totalN, err
	}
	return totalN, nil

}

func (c *StringConcatenation) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	//TODO: improve implementation

	return Str(c.GetOrBuildString()).PrettyPrint(w, config, depth, parentIndentCount)
}

func (c *BytesConcatenation) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	//TODO: improve implementation

	return c.GetOrBuildBytes().PrettyPrint(w, config, depth, parentIndentCount)
}

func (s *TestSuite) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (c *TestCase) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", c)
}

func (d *DynamicValue) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	totalN, err := w.Write([]byte{'d', 'y', 'n', '('})
	if err != nil {
		return totalN, err
	}

	n, err := d.Resolve(config.Context).PrettyPrint(w, config, depth+1, parentIndentCount)
	totalN += n
	if err != nil {
		return totalN, err
	}

	n, err = w.Write([]byte{')'})
	totalN += n
	return totalN, err
}

func (e *Event) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", e)
}

func (s *ExecutedStep) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (j *LifetimeJob) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", j)
}

func (watcher *GenericWatcher) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", watcher)
}

func (watcher *PeriodicWatcher) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", watcher)
}

func (m Mutation) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", m)
}

func (watcher *joinedWatchers) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", watcher)
}

func (watcher stoppedWatcher) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", watcher)
}

func (s *wrappedWatcherStream) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (s *ElementsStream) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (s *ReadableByteStream) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (s *WritableByteStream) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (s *ConfluenceStream) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (c Color) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", c)
}

func (r *RingBuffer) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", r)
}

func (c *DataChunk) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", c)
}

func (d *StaticCheckData) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", d)
}

func (d *SymbolicData) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", d)
}

func (m *Module) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", m)
}

func (s *GlobalState) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", s)
}

func (m Message) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", m)
}

func (s *Subscription) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", s)
}

func (p *Publication) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", p)
}

func (h *ValueHistory) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", h)
}

func (h *SynchronousMessageHandler) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", h)
}

func (g *SystemGraph) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", g)
}

func (n *SystemGraphNodes) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", n)
}

func (n *SystemGraphNode) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", n)
}

func (e SystemGraphEvent) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", e)
}

func (e SystemGraphEdge) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", e)
}
