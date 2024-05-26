package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

type Bytecode struct {
	Bytecode any //if nil, any function is matched
}

func (b *Bytecode) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*Bytecode)
	if !ok {
		return false
	}
	if b.Bytecode == nil {
		return true
	}

	if other.Bytecode == nil {
		return false
	}

	return utils.SamePointer(b.Bytecode, other.Bytecode)
}

func (b *Bytecode) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if b.Bytecode == nil {
		w.WriteName("bytecode")
		return
	}

	w.WriteNameF("bytecode(%v)", b.Bytecode)
}

func (b *Bytecode) WidestOfType() Value {
	return &Bytecode{}
}
