package core

// A GoValue represents a user defined Value or a core Value that is not deeply integrated with the evaluation logic.
type GoValue interface {
	Value
	IProps
	GetGoMethod(name string) (*GoFunction, bool)
}

func GetGoMethodOrPanic(name string, v GoValue) Value {
	method, ok := v.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, v))
	}
	return method
}
