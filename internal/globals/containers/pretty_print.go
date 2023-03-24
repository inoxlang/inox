package internal

import (
	"fmt"
	"io"

	core "github.com/inox-project/inox/internal/core"
)

func (s *Set) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (s *Stack) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (q *Queue) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", q)
}

func (t *Thread) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", t)
}

func (m *Map) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", m)
}

func (g *Graph) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", g)
}

func (n GraphNode) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", n)
}

func (r *Ranking) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", r)
}

func (r *Rank) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", r)
}

func (it *CollectionIterator) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (wk *GraphWalker) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", wk)
}

func (it *TreeIterator) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", it)
}

func (t *Tree) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", t)
}

func (n TreeNode) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", n)
}

func (n *TreeNodePattern) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", n)
}
