//go:build linux

package systemdprovider

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/inoxlang/inox/internal/inoxd"
)

const (
	DEFAULT_INOX_PATH        = "/usr/local/bin/inox"
	SYSTEMD_DIR_PATH         = "/etc/systemd"
	INOX_SERVICE_UNIT_NAME   = "inox"
	INOX_SERVICE_UNIT_PATH   = SYSTEMD_DIR_PATH + "/system/" + INOX_SERVICE_UNIT_NAME + ".service"
	INOX_SERVICE_UNIT_FPERMS = 0o644

	SYSTEMCTL_CMD_NAME = "systemctl"
)

var (
	ErrUnitFileExists = errors.New("unit file already exists")
	ErrNoSystemd      = errors.New("systemd does not seem to be the init system on this machine")
	ErrNotRoot        = errors.New("current user is not root")
)

type InoxUnitParams struct {
	Username, Homedir string
	UID               int
	Log               io.Writer

	InoxCloud bool
	EnvFile   string //optional
}

// WriteInoxUnitFile writes the unit file for the inox service at INOX_SERVICE_UNIT_PATH, if the file already exists
// ErrUnitFileExists is returned and unitName is set.
func WriteInoxUnitFile(args InoxUnitParams) (unitName string, _ error) {
	path := INOX_SERVICE_UNIT_PATH
	unitName = strings.TrimSuffix(filepath.Base(path), ".service")

	if _, err := os.Stat(SYSTEMD_DIR_PATH); os.IsNotExist(err) {
		return "", fmt.Errorf("%w: dir %s does not exist", ErrNoSystemd, SYSTEMD_DIR_PATH)
	} else if err != nil {
		return "", err
	}

	if _, err := exec.LookPath(SYSTEMCTL_CMD_NAME); err != nil {
		return "", fmt.Errorf("%w: the %s command is not present", ErrNoSystemd, SYSTEMCTL_CMD_NAME)
	} else if err != nil {
		return "", err
	}

	if _, err := os.Stat(path); err == nil {
		return unitName, fmt.Errorf("%w: %s", ErrUnitFileExists, path)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	unitSection := unit.UnitSection{
		Section: "Unit",
		Entries: []*unit.UnitEntry{
			{
				Name:  "Description",
				Value: "Inox service (Inoxd)",
			},
			{
				Name:  "Requires",
				Value: "network.target",
			},
			{
				Name:  "After",
				Value: "multi-user.target",
			},
		},
	}

	inoxConfig := fmt.Sprintf(`'-config={"inoxCloud":%t,"serverConfig":{"maxWebsocketPerIp":2}}'`, args.InoxCloud)

	serviceSection := unit.UnitSection{
		Section: "Service",
		Entries: []*unit.UnitEntry{
			{
				Name: "ExecStart",
				//inox daemon '-config=....'
				Value: DEFAULT_INOX_PATH + " " + inoxd.DAEMON_SUBCMD + " " + inoxConfig,
			},
			{
				Name:  "Type",
				Value: "simple",
			},
			{
				Name:  "User",
				Value: args.Username,
			},
			{
				Name:  "WorkingDirectory",
				Value: args.Homedir,
			},

			{
				//https://systemd.io/CGROUP_DELEGATION
				//https://www.freedesktop.org/wiki/Software/systemd/ControlGroupInterface/
				Name:  "Delegate",
				Value: "yes",
			},
			{
				Name:  "Restart",
				Value: "always",
			},

			{
				//https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html#RestartSec=
				Name:  "RestartSec",
				Value: "1s",
			},

			{
				//https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html#StartLimitIntervalSec=interval
				Name:  "StartLimitIntervalSec",
				Value: "300",
			},
			{
				Name:  "StartLimitBurst",
				Value: "5",
			},
		},
	}

	if args.EnvFile != "" {
		//https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#EnvironmentFile=

		serviceSection.Entries = append(serviceSection.Entries, &unit.UnitEntry{
			Name:  "EnvironmentFile",
			Value: args.EnvFile,
		})
	}

	installSection := unit.UnitSection{
		Section: "Install",
		Entries: []*unit.UnitEntry{
			{
				Name:  "WantedBy",
				Value: "multi-user.target",
			},
		},
	}

	serialized, err := io.ReadAll(unit.SerializeSections([]*unit.UnitSection{
		&unitSection,
		&serviceSection,
		&installSection,
	}))

	if err != nil {
		return "", err
	}

	fmt.Fprintln(args.Log, "\nwrite "+path+":")

	return unitName, os.WriteFile(path, serialized, INOX_SERVICE_UNIT_FPERMS)
}

func EnableInoxd(unitName string, out io.Writer, errOut io.Writer) error {
	systemctlPath, err := exec.LookPath(SYSTEMCTL_CMD_NAME)
	if err != nil {
		return err
	}

	if !slices.Contains(SYSTEMCTL_ALLOWED_LOCATIONS, systemctlPath) {
		return fmt.Errorf("the binary for the %s command has an unexpected location: %q", SYSTEMCTL_CMD_NAME, systemctlPath)
	}

	cmd := exec.Command(systemctlPath, "enable", unitName)
	cmd.Stderr = errOut
	cmd.Stdout = out

	fmt.Fprintln(out, "\nenable inoxd service:")
	fmt.Fprintln(out, cmd.String())

	return cmd.Run()
}
