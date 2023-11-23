package systemd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/coreos/go-systemd/v22/unit"
)

func StopRemoveUnit(removeEnvFile bool, out io.Writer, errOut io.Writer) error {
	systemctlPath, err := getSystemctlPath()
	if err != nil {
		return err
	}

	//disable the service
	disableCmd := exec.Command(systemctlPath, "disable", INOX_SERVICE_UNIT_NAME)
	disableCmd.Stderr = errOut
	disableCmd.Stdout = out

	fmt.Fprintln(out, disableCmd.String())
	err = disableCmd.Run()
	if err != nil {
		return errors.New("failed to disable the unit")
	}

	if removeEnvFile {
		content, err := os.ReadFile(INOX_SERVICE_UNIT_PATH)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return nil
		}

		options, err := unit.Deserialize(bytes.NewReader(content))
		if err != nil {
			fmt.Fprintln(errOut, err)
			return nil
		}

		//search for the environment file path.
		for _, opt := range options {
			if opt.Name != "EnvironmentFile" {
				continue
			}

			err := os.RemoveAll(opt.Value)
			if err != nil {
				fmt.Fprintln(errOut, err)
			}
		}
	}

	//remove the unit file.
	fmt.Fprintln(out, "remove "+INOX_SERVICE_UNIT_PATH)
	err = os.Remove(INOX_SERVICE_UNIT_PATH)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return nil
	}

	//stop the service, errors are printed only.
	stopCmd := exec.Command(systemctlPath, "stop", INOX_SERVICE_UNIT_NAME)
	stopCmd.Stderr = errOut
	stopCmd.Stdout = out

	fmt.Fprintln(out, stopCmd.String())
	err = stopCmd.Run()
	if err != nil {
		fmt.Fprintln(errOut, "failed to stop the unit")
		//keep going
	}

	reloadCmd := exec.Command(systemctlPath, "daemon-reload")
	reloadCmd.Stderr = errOut
	reloadCmd.Stdout = out

	fmt.Fprintln(out, reloadCmd.String())
	err = reloadCmd.Run()
	if err != nil {
		fmt.Fprintln(errOut, err)
		//keep going
	}

	resetFailedCmd := exec.Command(systemctlPath, "reset-failed")
	resetFailedCmd.Stderr = errOut
	resetFailedCmd.Stdout = out

	fmt.Fprintln(out, resetFailedCmd.String())
	err = resetFailedCmd.Run()

	if err != nil {
		fmt.Fprintln(errOut, err)
		//keep going
	}

	return nil
}
