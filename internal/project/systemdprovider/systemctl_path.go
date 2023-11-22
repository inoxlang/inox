package systemdprovider

import (
	"fmt"
	"os/exec"
	"slices"
)

var (
	SYSTEMCTL_ALLOWED_LOCATIONS = []string{"/usr/bin/systemctl"}
)

func getSystemctlPath() (string, error) {
	systemctlPath, err := exec.LookPath(SYSTEMCTL_CMD_NAME)
	if err != nil {
		return "", err
	}

	if !slices.Contains(SYSTEMCTL_ALLOWED_LOCATIONS, systemctlPath) {
		return "", fmt.Errorf("the binary for the %s command has an unexpected location: %q", SYSTEMCTL_CMD_NAME, systemctlPath)
	}

	return systemctlPath, nil
}
