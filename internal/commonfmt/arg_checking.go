package commonfmt

import "fmt"

func FmtUnexpectedPropInArgX(propName string, argName string) error {
	return fmt.Errorf("unexpected property .%s in %s argument", propName, argName)
}

func FmtInvalidValueForPropXOfArgY(propName string, argName string, msg string) error {
	return fmt.Errorf("invalid value for property .%s of %s argument: %s", propName, argName, msg)
}
