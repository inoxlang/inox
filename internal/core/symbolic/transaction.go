package internal

// A Transaction represents a symbolic Transaction.
type Transaction struct {
	UnassignablePropsMixin
	_ int
}

func (tx *Transaction) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *Transaction:
		return true
	default:
		return false
	}
}

func (tx *Transaction) Start(ctx *Context) *Error {
	return nil
}

func (tx *Transaction) Commit(ctx *Context) *Error {
	return nil
}

func (tx *Transaction) Rollback(ctx *Context) *Error {
	return nil
}

func (tx *Transaction) Prop(name string) SymbolicValue {
	method, ok := tx.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, tx))
	}
	return method
}

func (tx *Transaction) PropertyNames() []string {
	return []string{"start", "commit", "rollback"}
}

func (tx *Transaction) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "start":
		return &GoFunction{fn: tx.Start}, true
	case "commit":
		return &GoFunction{fn: tx.Commit}, true
	case "rollback":
		return &GoFunction{fn: tx.Rollback}, true
	}
	return &GoFunction{}, false
}

func (tx *Transaction) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (tx *Transaction) IsWidenable() bool {
	return false
}

func (tx *Transaction) String() string {
	return "%transaction"
}

func (tx *Transaction) WidestOfType() SymbolicValue {
	return &Transaction{}
}
