package internal

import (
	"fmt"
	"io"
	"strings"

	"github.com/muesli/termenv"

	core "github.com/inoxlang/inox/internal/core"
	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	defaultPrettyPrintConfig = &core.PrettyPrintConfig{
		PrettyPrintConfig: pprint.PrettyPrintConfig{
			MaxDepth: 7,
			Colorize: true,
			Colors:   &pprint.DEFAULT_DARKMODE_PRINT_COLORS,
			Compact:  false,
			Indent:   []byte{' ', ' '},
		},
	}

	COLOR_NAME_TO_COLOR = map[core.Identifier]termenv.Color{
		"red":        termenv.ANSIRed,
		"bright-red": termenv.ANSIBrightRed,

		"blue":        termenv.ANSIBlue,
		"bright-blue": termenv.ANSIBrightBlue,

		"cyan":        termenv.ANSICyan,
		"bright-cyan": termenv.ANSIBrightCyan,

		"yellow":        termenv.ANSIYellow,
		"bright-yellow": termenv.ANSIBrightYellow,

		"green":        termenv.ANSIGreen,
		"bright-green": termenv.ANSIBrightGreen,

		"white":        termenv.ANSIWhite,
		"bright-white": termenv.ANSIBrightWhite,

		"black":        termenv.ANSIBlack,
		"bright-black": termenv.ANSIBrightBlack,

		"magenta":        termenv.ANSIMagenta,
		"bright-magenta": termenv.ANSIBrightMagenta,
	}
)

// evaluates the different parts of the prompt and print them
func printPrompt(writer io.Writer, state *core.TreeWalkState, config REPLConfiguration) (prompt_length int) {
	prompt, length := sprintPrompt(state, config)
	fmt.Fprint(writer, prompt)
	return length
}

// evaluates the different parts of the prompt and return the colorized prompt
func sprintPrompt(state *core.TreeWalkState, config REPLConfiguration) (prompt string, prompt_length int) {

	for _, part := range config.prompt.GetOrBuildElements(state.Global.Ctx) {

		color := config.defaultFgColor.ToTermColor()

		list, isList := part.(*core.List)

		if isList && list.Len() == 3 {
			part = list.At(state.Global.Ctx, 0)
			colorIndex := 1
			if config.IsLight() {
				colorIndex = 2
			}

			colorIdent, isIdent := list.At(state.Global.Ctx, colorIndex).(core.Identifier)

			if isIdent {
				clr, ok := COLOR_NAME_TO_COLOR[colorIdent]
				if ok {
					color = clr
				}
			}
		}

		s := ""

		switch p := part.(type) {
		case core.Str:
			s = string(p)
		case core.AstNode:

			if call, isCall := p.Node.(*parse.CallExpression); isCall {

				idnt, isIdent := call.Callee.(*parse.IdentifierLiteral)
				if !isIdent || !utils.SliceContains(ALLOWED_PROMPT_FUNCTION_NAMES, idnt.Name) || len(call.Arguments) != 0 {
					panic(fmt.Errorf("writePrompt: only some restricted call expressions are allowed"))
				}

			} else if !parse.NodeIsSimpleValueLiteral(p.Node) && !parse.NodeIs(p.Node, (*parse.URLExpression)(nil)) {
				panic(fmt.Errorf("writePrompt: only url expressions, simple-value literals and some other restricted expressions can be evaluated"))
			}
			v, _ := core.TreeWalkEval(p.Node, state)
			s = fmt.Sprintf("%s", v)
		default:
		}

		//we print the part
		prompt_length += len([]rune(s))
		styled := termenv.String(s)
		styled = styled.Foreground(color)
		prompt += styled.String()
	}
	return
}

func getClosingDelimiter(openingDelim rune) rune {
	switch openingDelim {
	case '[':
		return ']'
	case '{':
		return '}'
	case '(':
		return ')'
	default:
		return openingDelim
	}
}

func moveCursor(writer io.Writer, row int, column int) {
	fmt.Fprintf(writer, termenv.CSI+termenv.CursorPositionSeq, row, column)
}

func clearScreen(writer io.Writer) {
	fmt.Fprintf(writer, termenv.CSI+termenv.EraseDisplaySeq, 2)
	moveCursor(writer, 1, 1)
	writer.Write([]byte(termenv.CSI + termenv.EraseEntireLineSeq))
}

func clearLine(writer io.Writer) {
	writer.Write([]byte(termenv.CSI + termenv.EraseEntireLineSeq))
}

func clearLineRight(writer io.Writer) {
	writer.Write([]byte(termenv.CSI + termenv.EraseLineRightSeq))
}

func clearLines(writer io.Writer, n int) {
	clearLine := fmt.Sprintf(termenv.CSI+termenv.EraseLineSeq, 2)
	cursorUp := fmt.Sprintf(termenv.CSI+termenv.CursorUpSeq, 1)
	fmt.Fprint(writer, clearLine+strings.Repeat(cursorUp+clearLine, n))
}

func moveCursorBack(writer io.Writer, n int) {
	if n == 0 {
		return
	}
	fmt.Fprintf(writer, termenv.CSI+termenv.CursorBackSeq, n)
}

func moveCursorForward(writer io.Writer, n int) {
	if n == 0 {
		return
	}
	fmt.Fprintf(writer, termenv.CSI+termenv.CursorForwardSeq, n)
}

func moveCursorUp(writer io.Writer, n int) {
	if n == 0 {
		return
	}
	fmt.Fprintf(writer, termenv.CSI+termenv.CursorUpSeq, n)
}
func moveCursorDown(writer io.Writer, n int) {
	if n == 0 {
		return
	}
	fmt.Fprintf(writer, termenv.CSI+termenv.CursorDownSeq, n)
}

func moveCursorNextLine(writer io.Writer, n int) {
	if n == 0 {
		return
	}
	fmt.Fprintf(writer, termenv.CSI+termenv.CursorNextLineSeq, n)
}

func saveCursorPosition(writer io.Writer) {
	fmt.Fprint(writer, termenv.CSI+termenv.SaveCursorPositionSeq)
}

func restoreCursorPosition(writer io.Writer) {
	fmt.Fprint(writer, termenv.CSI+termenv.RestoreCursorPositionSeq)
}
