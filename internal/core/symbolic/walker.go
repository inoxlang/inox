package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_WALKABLE = &AnyWalkable{}
	ANY_WALKER   = &Walker{}

	_ = []Walkable{(*Path)(nil), (*UData)(nil)}
)

// An Walkable represents a symbolic Walkable.
type Walkable interface {
	SymbolicValue
	WalkerElement() SymbolicValue
	WalkerNodeMeta() SymbolicValue
}

// An AnyWalkable represents a symbolic Walkable we do not know the concrete type.
type AnyWalkable struct {
	_ int
}

func (r *AnyWalkable) Test(v SymbolicValue) bool {
	_, ok := v.(*AnyWalkable)

	return ok
}

func (r *AnyWalkable) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%walkable")))
}

func (r *AnyWalkable) WidestOfType() SymbolicValue {
	return ANY_WALKABLE
}

func (r *AnyWalkable) WalkerElement() SymbolicValue {
	return ANY
}

// A Walker represents a symbolic Walker.
type Walker struct {
	//after any update make sure ANY_WALKER is still valid

	_ int
}

func (r *Walker) Test(v SymbolicValue) bool {
	_, ok := v.(*Walker)

	return ok
}

func (r *Walker) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%walker")))
}

func (r *Walker) WidestOfType() SymbolicValue {
	return ANY_WALKER
}
