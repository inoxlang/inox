package symbolic

type ExceptedValueInfo struct {
	value Value
}

func (i ExceptedValueInfo) Value() Value {
	return i.value
}
