package core

import "fmt"

func FmtErrNArgumentsExpected(count string) error {
	return fmt.Errorf("invalid number of arguments, %s arguments were expected", count)
}

func FmtMissingArgument(argName string) error {
	return fmt.Errorf("missing %s argument", argName)
}

func FmtErrArgumentProvidedAtLeastTwice(name string) error {
	return fmt.Errorf("%s argument provided at least twice", name)
}

func FmtErrXProvidedAtLeastTwice(name string) error {
	return fmt.Errorf("%s provided at least twice", name)
}

func FmtErrInvalidArgument(v Value) error {
	return fmt.Errorf("invalid argument of type %T", v)
}

func FmtErrInvalidArgumentAtPos(v Value, pos int) error {
	return fmt.Errorf("invalid argument of type %T as position %d", v, pos)
}

func FmtErrOptionProvidedAtLeastTwice(name string) error {
	return fmt.Errorf("%s option provided at least twice", name)
}

func FmtErrInvalidOptionName(name string) error {
	return fmt.Errorf("invalid option name '%s'", name)
}

func FmtMissingPropInArgX(propName string, argName string) error {
	return fmt.Errorf("missing property .%s in %s argument", propName, argName)
}

func FmtPropOfArgXShouldBeOfTypeY(propName string, argName string, typename string, value Value) error {
	return fmt.Errorf("property .%s of %s argument should be of type %s but is a value of type %T: %s", propName, argName, typename, value, Stringify(value, nil))
}

func FmtPropOfArgXShouldBeY(propName string, argName string, info string) error {
	return fmt.Errorf("property .%s of %s argument should be %s", propName, argName, info)
}

func FmtUnexpectedValueAtKeyofArgShowVal(val Value, key string, argName string) error {
	return fmt.Errorf("unexpected value at .%s of %s argument: %#v", key, argName, val)
}

func FmtUnexpectedElementInPropIterableShowVal(element Value, propertyName string) error {
	return fmt.Errorf("unexpected element in .%s: %#v", propertyName, element)
}

func FmtUnexpectedElementInPropIterable(propertyName string, s string) error {
	return fmt.Errorf("unexpected element in .%s: %s", propertyName, s)
}

func FmtUnexpectedElementInPropIterableOfArgX(propertyName string, argName string, s string) error {
	return fmt.Errorf("unexpected element in .%s of %s argument: %s", propertyName, argName, s)
}

func FmtUnexpectedElementAtIndexKeyxofArgShowVal(element Value, keyIndex string, argName string) error {
	return fmt.Errorf("unexpected element at index key .%s of %s argument: %#v", keyIndex, argName, element)
}

func FmtUnexpectedElementAtIndeKeyXofArg(keyIndex string, argName string, s string) error {
	return fmt.Errorf("unexpected element at index key .%s of %s argument: %s", keyIndex, argName, s)
}
