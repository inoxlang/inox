package systemd

import (
	"fmt"
	"io"
	"os/exec"
	"time"
)

func StartInoxd(unitName string, restart bool, out io.Writer, errOut io.Writer) error {
	systemctlPath, err := getSystemctlPath()
	if err != nil {
		return err
	}

	subcmd := "start"
	if restart {
		subcmd = "restart"
	}

	startCmd := exec.Command(systemctlPath, subcmd, unitName)
	startCmd.Stderr = errOut
	startCmd.Stdout = out

	fmt.Fprintln(out, "\nstart inoxd service:")
	fmt.Fprintln(out, startCmd.String())

	err = startCmd.Run()
	if err != nil {
		return err
	}

	//wait a bit before getting the status
	time.Sleep(time.Second)

	//get and print status

	statusCmd := exec.Command(systemctlPath, "status", unitName)

	//wrap out & errOut to disable interactivity.
	//TODO: force systemctl to use colors, setting SYSTEMD_COLORS=1 and SYSTEMD_URLIFY=0 does not seem to work (?).
	statusCmd.Stdout = io.MultiWriter(out)
	statusCmd.Stderr = io.MultiWriter(errOut)

	fmt.Fprintln(out, "\nget status:")
	fmt.Fprintln(out, statusCmd.String())

	err = statusCmd.Run()

	//ignore error if systemctl's exit status is 3 because it does not seem to signal an important issue.
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
		return nil
	}

	return err
}
