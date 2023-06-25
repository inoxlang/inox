package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []Walkable{(*Path)(nil)}
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

func (r *AnyWalkable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnyWalkable) IsWidenable() bool {
	return false
}

func (r *AnyWalkable) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%walkable")))
	return
}

func (r *AnyWalkable) WidestOfType() SymbolicValue {
	return &AnyWalkable{}
}

func (r *AnyWalkable) WalkerElement() SymbolicValue {
	return ANY
}

// A Walker represents a symbolic Walker.
type Walker struct {
	_ int
}

func (r *Walker) Test(v SymbolicValue) bool {
	_, ok := v.(*Walker)

	return ok
}

func (r *Walker) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Walker) IsWidenable() bool {
	return false
}

func (r *Walker) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%walker")))
	return
}

func (r *Walker) WidestOfType() SymbolicValue {
	return &Walker{}
}
