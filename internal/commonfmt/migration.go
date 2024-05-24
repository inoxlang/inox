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

func FmtInvalidLastSegmentOfMigrationPathShouldbeAnInteger(pth []string) error {
	return fmt.Errorf("last segment of migration path %s should be an integer instead", "/"+strings.Join(pth, "/"))
}

func FmtLastSegmentOfMigrationPathIsOutOfBounds(pth []string) error {
	return fmt.Errorf("last segment of migration path %s is out of bounds", "/"+strings.Join(pth, "/"))
}

func FmtMissingNextPattern(pth []string) error {
	return fmt.Errorf("missing next pattern for %s", "/"+strings.Join(pth, "/"))
}
