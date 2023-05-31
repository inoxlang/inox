package html_ns

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	core "github.com/inoxlang/inox/internal/core"
	. "github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_PRETTY_PRINT_COST       = 5000
	MAX_PRETTY_PRINT_LINE_COUNT = 200
)

func (n *HTMLNode) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	if depth > config.MaxDepth || computeApproximativePrintCost(n.node) > MAX_PRETTY_PRINT_COST || computeApproximateRequiredLines(n.node) > MAX_PRETTY_PRINT_LINE_COUNT {
		Must(fmt.Fprint(w, "(...big html tree...)", n))
	}

	lexer := lexers.Get("html")
	if lexer == nil {
		lexer = lexers.Fallback
	}

	//chroma.Coalesce not working for HTML ?

	parentIndent := bytes.Repeat(config.Indent, parentIndentCount)

	htmlString := string(RenderToString(config.Context, n))
	iterator := Must(lexer.Tokenise(nil, htmlString))

	for token := iterator(); token != chroma.EOF; token = iterator() {

		if config.Colorize {
			switch token.Type {
			case chroma.NameTag:
				w.Write(config.Colors.XmlTagName)
			case chroma.Punctuation:
				w.Write(config.Colors.DiscreteColor)
			case chroma.LiteralString:
				w.Write(config.Colors.StringLiteral)
			case chroma.NameAttribute:
				w.Write(config.Colors.IdentifierLiteral)
			default:
			}
		}

		s := token.String()

		if strings.ContainsAny(s, "\n") {
			s = strings.ReplaceAll(s, "\n", "\n"+BytesAsString(parentIndent))
		}

		w.Write(StringAsBytes(s))
		if config.Colorize {
			w.Write(core.ANSI_RESET_SEQUENCE)
		}
	}
}
