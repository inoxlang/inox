package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	TRANSACTION_PROPNAMES = []string{"start", "commit", "rollback"}
)

// A Transaction represents a symbolic Transaction.
type Transaction struct {
	UnassignablePropsMixin
	_ int
}

func (tx *Transaction) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

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

func (tx *Transaction) Prop(name string) Value {
	method, ok := tx.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, tx))
	}
	return method
}

func (tx *Transaction) PropertyNames() []string {
	return TRANSACTION_PROPNAMES
}

func (tx *Transaction) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "start":
		return WrapGoMethod(tx.Start), true
	case "commit":
		return WrapGoMethod(tx.Commit), true
	case "rollback":
		return WrapGoMethod(tx.Rollback), true
	}
	return nil, false
}

func (tx *Transaction) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("transaction")
	return
}

func (tx *Transaction) WidestOfType() Value {
	return &Transaction{}
}
