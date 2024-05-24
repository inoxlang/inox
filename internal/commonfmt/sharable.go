package commonfmt

import "fmt"

func FmtNotSharableBecausePropertyNotSharable(propName, explanation string) string {
	return fmt.Sprintf("value is not sharable because .%s is not sharable: %s", propName, explanation)
}
