package systemd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func StopRemoveUnit(unitName string, out io.Writer, errOut io.Writer) error {
	systemctlPath, err := getSystemctlPath()
	if err != nil {
		return err
	}

	//disable the service
	disableCmd := exec.Command(systemctlPath, "disable", unitName)
	disableCmd.Stderr = errOut
	disableCmd.Stdout = out

	fmt.Fprintln(out, disableCmd.String())
	err = disableCmd.Run()
	if err != nil {
		return errors.New("failed to disable the unit")
	}

	//remove the unit file.
	fmt.Fprintln(out, "remove "+INOX_SERVICE_UNIT_PATH)
	err = os.Remove(INOX_SERVICE_UNIT_PATH)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return nil
	}

	//stop the service, errors are printed only.
	stopCmd := exec.Command(systemctlPath, "stop", unitName)
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
