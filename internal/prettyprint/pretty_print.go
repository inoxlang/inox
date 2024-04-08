package prettyprint

import "github.com/muesli/termenv"

var (
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
		MarkupTagName:     GetFullColorSequence(termenv.ANSIBlue, false),
		InvalidNode:       GetFullColorSequence(termenv.ANSIBrightRed, false),
		DiscreteColor:     GetFullColorSequence(termenv.ANSIBrightBlack, false),

		Folder:     GetFullColorSequence(termenv.ANSIBlue, false),
		Executable: GetFullColorSequence(termenv.ANSIBrightGreen, false),

		SuccessColor: GetFullColorSequence(termenv.ANSIBrightGreen, false),
		WarnColor:    GetFullColorSequence(termenv.ANSIYellow, false),
		ErrorColor:   GetFullColorSequence(termenv.ANSIRed, false),
	}

	DEFAULT_LIGHTMODE_PRINT_COLORS = PrettyPrintColors{
		ControlKeyword:    GetFullColorSequence(termenv.ANSI256Color(90), false),
		OtherKeyword:      GetFullColorSequence(termenv.ANSI256Color(26), false),
		PatternLiteral:    GetFullColorSequence(termenv.ANSI256Color(1), false),
		StringLiteral:     GetFullColorSequence(termenv.ANSI256Color(88), false),
		PathLiteral:       GetFullColorSequence(termenv.ANSI256Color(88), false),
		IdentifierLiteral: GetFullColorSequence(termenv.ANSI256Color(27), false),
		NumberLiteral:     GetFullColorSequence(termenv.ANSI256Color(28), false),
		Constant:          GetFullColorSequence(termenv.ANSI256Color(21), false),
		PatternIdentifier: GetFullColorSequence(termenv.ANSI256Color(28), false),
		CssTypeSelector:   GetFullColorSequence(termenv.ANSIBlack, false),
		CssOtherSelector:  GetFullColorSequence(termenv.ANSIYellow, false),
		MarkupTagName:     GetFullColorSequence(termenv.ANSIBlue, false),
		InvalidNode:       GetFullColorSequence(termenv.ANSI256Color(160), false),
		DiscreteColor:     GetFullColorSequence(termenv.ANSIBrightBlack, false),

		Folder:     GetFullColorSequence(termenv.ANSI256Color(26), false),
		Executable: GetFullColorSequence(termenv.ANSI256Color(28), false),

		SuccessColor: GetFullColorSequence(termenv.ANSIBrightGreen, false),
		WarnColor:    GetFullColorSequence(termenv.ANSIYellow, false),
		ErrorColor:   GetFullColorSequence(termenv.ANSIRed, false),
	}
)

type PrettyPrintColors struct {
	//inox code
	ControlKeyword, OtherKeyword, PatternLiteral, StringLiteral, PathLiteral, IdentifierLiteral,
	NumberLiteral, Constant, PatternIdentifier, CssTypeSelector, CssOtherSelector, MarkupTagName, InvalidNode,

	DiscreteColor, Folder, Executable,
	SuccessColor, WarnColor, ErrorColor []byte
}

type PrettyPrintConfig struct {
	MaxDepth                    int
	Colorize                    bool
	Colors                      *PrettyPrintColors
	Compact                     bool
	Indent                      []byte
	PrintDecodedTopLevelStrings bool
}

func GetFullColorSequence(color termenv.Color, bg bool) []byte {
	var b = []byte(termenv.CSI)
	b = append(b, []byte(color.Sequence(bg))...)
	b = append(b, 'm')
	return b
}
