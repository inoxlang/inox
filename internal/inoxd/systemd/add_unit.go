//go:build linux

package systemd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/inoxlang/inox/internal/inoxd"
	"github.com/inoxlang/inox/internal/project_server"
	"github.com/inoxlang/inox/internal/utils"
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

	InoxCloud              bool
	EnvFilePath            string //optional
	ExposeProjectServers   bool
	ExposeWebServers       bool
	TunnelProviderName     string //optional
	AllowBrowserAutomation bool
}

func CheckFileDoesNotExist() error {
	path := INOX_SERVICE_UNIT_PATH
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("%w: %s", ErrUnitFileExists, path)
}

// WriteInoxUnitFile writes the unit file for inoxd at INOX_SERVICE_UNIT_PATH, if the file already exists
// ErrUnitFileExists is returned and unitName is set.
func WriteInoxUnitFile(args InoxUnitParams) (unitName string, _ error) {
	if (args.TunnelProviderName != "" && (args.ExposeProjectServers || args.ExposeWebServers)) ||
		(args.InoxCloud && (args.ExposeProjectServers || args.ExposeWebServers)) {
		return "", errors.New("invalid arguments")
	}

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
				Value: "Inox Daemon (inoxd)",
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

	daemonConfig := inoxd.DaemonConfig{
		InoxCloud: args.InoxCloud,
		Server: project_server.IndividualServerConfig{
			BehindCloudProxy:       args.InoxCloud,
			BindToAllInterfaces:    args.ExposeProjectServers,
			AllowBrowserAutomation: args.AllowBrowserAutomation,
		},
		ExposeWebServers: args.ExposeProjectServers,
		TunnelProvider:   args.TunnelProviderName,
	}

	configString := fmt.Sprintf(`'-config=%s'`, utils.Must(json.Marshal(daemonConfig)))

	serviceSection := unit.UnitSection{
		Section: "Service",
		Entries: []*unit.UnitEntry{
			{
				Name: "ExecStart",
				//inox daemon '-config=....'
				Value: DEFAULT_INOX_PATH + " " + inoxd.DAEMON_SUBCMD + " " + configString,
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

	if args.EnvFilePath != "" {
		//https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html#EnvironmentFile=

		serviceSection.Entries = append(serviceSection.Entries, &unit.UnitEntry{
			Name:  "EnvironmentFile",
			Value: args.EnvFilePath,
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
	systemctlPath, err := getSystemctlPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(systemctlPath, "enable", unitName)
	cmd.Stderr = errOut
	cmd.Stdout = out

	fmt.Fprintln(out, "\nenable inoxd service:")
	fmt.Fprintln(out, cmd.String())

	return cmd.Run()
}
