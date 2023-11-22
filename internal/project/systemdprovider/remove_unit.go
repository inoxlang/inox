package systemdprovider

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

	stopCmd := exec.Command(systemctlPath, "stop", unitName)
	stopCmd.Stderr = errOut
	stopCmd.Stdout = out

	fmt.Fprintln(out, stopCmd.String())
	err = stopCmd.Run()
	if err != nil {
		return errors.New("failed to stop the unit")
	}

	disableCmd := exec.Command(systemctlPath, "disable", unitName)
	stopCmd.Stderr = errOut
	stopCmd.Stdout = out

	fmt.Fprintln(out, disableCmd.String())
	err = disableCmd.Run()
	if err != nil {
		return errors.New("failed to disable the unit")
	}

	fmt.Fprintln(out, "remove "+INOX_SERVICE_UNIT_PATH)
	err = os.Remove(INOX_SERVICE_UNIT_PATH)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return nil
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
