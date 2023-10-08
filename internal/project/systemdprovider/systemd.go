//go:build linux

package systemdprovider

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/inoxlang/inox/internal/project_server/inoxd"
)

const (
	DEFAULT_INOX_PATH        = "/usr/local/bin/inox"
	SYSTEMD_DIR_PATH         = "/etc/systemd"
	INOX_SERVICE_UNIT_PATH   = SYSTEMD_DIR_PATH + "/system/inox.service"
	INOX_SERVICE_UNIT_FPERMS = 0o644
)

var (
	ErrUnitFileExists = errors.New("unit file already exists")
)

func WriteInoxUnitFile() error {
	path := INOX_SERVICE_UNIT_PATH

	if _, err := os.Stat(SYSTEMD_DIR_PATH); os.IsNotExist(err) {
		return fmt.Errorf("systemd does not seem to be the init system on this machine")
	} else if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %s", ErrUnitFileExists, path)
	} else if !os.IsNotExist(err) {
		return err
	}

	unitSection := unit.UnitSection{
		Section: "Unit",
		Entries: []*unit.UnitEntry{
			{
				Name:  "Description",
				Value: "Inox service",
			},
			{
				Name:  "User",
				Value: inoxd.INOXD_USERNAME,
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

	serviceSection := unit.UnitSection{
		Section: "Service",
		Entries: []*unit.UnitEntry{
			{
				Name:  "Typle",
				Value: "simple",
			},
			{
				Name:  "ExecStart",
				Value: DEFAULT_INOX_PATH,
			},
		},
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
		return err
	}

	return os.WriteFile(path, serialized, INOX_SERVICE_UNIT_FPERMS)
}
