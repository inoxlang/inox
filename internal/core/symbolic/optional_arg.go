package symbolic

// optional parameter in symbolic Go function parameters
type OptionalParam[T Value] struct {
	Value *T //nil if argument is not provided
}

func (p *OptionalParam[T]) _optionalParamType() {
	//type assertion
	_ = optionalParam(p)
}

func (p *OptionalParam[T]) setValue(v Value) {
	value := v.(T)
	p.Value = &value
}

func (p *OptionalParam[T]) new() optionalParam {
	return &OptionalParam[T]{}
}

type optionalParam interface {
	_optionalParamType()
	setValue(v Value)
	new() optionalParam
}
