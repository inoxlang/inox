package core

import "fmt"

func FmtErrInvalidArgument(v Value) error {
	return fmt.Errorf("invalid argument of type %T", v)
}

func FmtErrInvalidArgumentAtPos(v Value, pos int) error {
	return fmt.Errorf("invalid argument of type %T as position %d", v, pos)
}

func FmtPropOfArgXShouldBeOfTypeY(propName string, argName string, typename string, value Value) error {
	return fmt.Errorf("property .%s of %s argument should be of type %s but is a value of type %T: %s", propName, argName, typename, value, Stringify(value, nil))
}

func FmtUnexpectedValueAtKeyofArgShowVal(val Value, key string, argName string) error {
	return fmt.Errorf("unexpected value at .%s of %s argument: %#v", key, argName, val)
}

func FmtUnexpectedElementInPropIterableShowVal(element Value, propertyName string) error {
	return fmt.Errorf("unexpected element in .%s: %#v", propertyName, element)
}

func FmtUnexpectedElementAtIndexKeyxofArgShowVal(element Value, keyIndex string, argName string) error {
	return fmt.Errorf("unexpected element at index key .%s of %s argument: %#v", keyIndex, argName, element)
}
