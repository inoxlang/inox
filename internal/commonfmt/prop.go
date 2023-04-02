package commonfmt

import "fmt"

func FmtFailedToSetPropXAcceptXButZProvided(name string, accepted string, provided string) error {
	return fmt.Errorf("failed to set property .%s, accepted type: %s but provided value's type is: %s", name, accepted, provided)
}
