package core

// A symbolScope represents a symbol scope during compilation.
type symbolScope int

const (
	GLOBAL_SCOPE symbolScope = iota + 1
	LOCAL_SCOPE
)

// symbol represents a symbol in the symbol table.
type symbol struct {
	Name       string
	Index      int
	IsConstant bool
}

// A symbolTable represents a symbol table for a single module during compilation.
type symbolTable struct {
	store           map[string]*symbol
	nextSymbolIndex int
}

func newSymbolTable() *symbolTable {
	return &symbolTable{
		store: make(map[string]*symbol),
	}
}

// Define defines a new symbol in the table and returns it.
func (t *symbolTable) Define(name string) *symbol {
	nextIndex := t.nextSymbolIndex
	symbol := &symbol{
		Name:  name,
		Index: nextIndex,
	}
	t.nextSymbolIndex++

	t.store[name] = symbol
	return symbol
}

// Resolve returns the symbol with a given name in the table.
func (t *symbolTable) Resolve(name string) (*symbol, bool) {
	symbol, ok := t.store[name]
	return symbol, ok
}

func (t *symbolTable) SymbolCount() int {
	return t.nextSymbolIndex
}

func (t *symbolTable) SymbolNames() []string {
	var names []string
	for name := range t.store {
		names = append(names, name)
	}
	return names
}
