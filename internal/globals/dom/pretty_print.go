package internal

import (
	"fmt"
	"io"

	core "github.com/inox-project/inox/internal/core"
)

func (n *Node) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", n)
}

func (p *NodePattern) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", p)
}

func (evs *DomEventSource) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", evs)
}

func (v *View) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", v)
}

func (v *ContentSecurityPolicy) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "ContentSecurityPolicy(%s)", v.String())
}
