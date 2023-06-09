package commonfmt

import "fmt"

func FmtUnexpectedPropInArgX(propName string, argName string) error {
	return fmt.Errorf("unexpected property .%s in %s argument", propName, argName)
}

func FmtInvalidValueForPropXOfArgY(propName string, argName string, msg string) error {
	return fmt.Errorf("invalid value for property .%s of %s argument: %s", propName, argName, msg)
}

func FmtErrInvalidArgumentAtPos(pos int, explanation string) error {
	return fmt.Errorf("invalid argument at position %d: %s", pos, explanation)
}

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

func FmtErrOptionProvidedAtLeastTwice(name string) error {
	return fmt.Errorf("%s option provided at least twice", name)
}

func FmtErrInvalidOptionName(name string) error {
	return fmt.Errorf("invalid option name '%s'", name)
}

func FmtMissingPropInArgX(propName string, argName string) error {
	return fmt.Errorf("missing property .%s in %s argument", propName, argName)
}

func FmtPropOfArgXShouldBeY(propName string, argName string, info string) error {
	return fmt.Errorf("property .%s of %s argument should be %s", propName, argName, info)
}

func FmtUnexpectedElementInPropIterable(propertyName string, s string) error {
	return fmt.Errorf("unexpected element in .%s: %s", propertyName, s)
}

func FmtUnexpectedElementInPropIterableOfArgX(propertyName string, argName string, s string) error {
	return fmt.Errorf("unexpected element in .%s of %s argument: %s", propertyName, argName, s)
}

func FmtUnexpectedElementAtIndeKeyXofArg(keyIndex string, argName string, s string) error {
	return fmt.Errorf("unexpected element at index key .%s of %s argument: %s", keyIndex, argName, s)
}
