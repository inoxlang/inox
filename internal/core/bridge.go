package core

type StateBridge struct {
	GlobalVariableValues map[string]Value
	LocalVariableValues  map[string]Value
	Context              *Context //patterns and pattern namespaces
}

func (b *StateBridge) GetVariableValue(name string) (Value, bool) {
	val, ok := b.LocalVariableValues[name]
	if ok {
		return val, true
	}
	val, ok = b.GlobalVariableValues[name]
	return val, ok
}
