package commonfmt

import (
	"fmt"
	"strings"
)

func FmtValueAtPathDoesNotExist(pth string) error {
	return fmt.Errorf("%s does not exist", pth)
}

func FmtValueAtPathSegmentsDoesNotExist(pth []string) error {
	return fmt.Errorf("%s does not exist", "/"+strings.Join(pth, "/"))
}

func FmtValueAtPathSegmentsIsNotMigrationCapable(pth []string) error {
	return fmt.Errorf("%s is not migration capable", "/"+strings.Join(pth, "/"))
}

func FmtErrWhileCallingMigrationHandler(pth []string, err error) error {
	return fmt.Errorf("error while calling migration handler for %s", "/"+strings.Join(pth, "/"))
}

func FmtErrWhileCloningValueFor(pth []string, err error) error {
	return fmt.Errorf("error while cloning value for %s", "/"+strings.Join(pth, "/"))
}
