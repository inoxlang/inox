package setcoll

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

func (s *Set) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	ctx := config.Context
	closestState := ctx.MustGetClosestState()
	s._lock(closestState)
	defer s._unlock(closestState)

	utils.Must(fmt.Fprintf(w, "%#v", s))
}

func (n *SetPattern) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%T", n))
}
