package inoxd

import (
	"fmt"
	"io"
	"os/exec"
	"os/user"
	"slices"
)

const (
	INOXD_USERNAME = "inoxd"
)

var (
	ALLOWED_USERADD_LOCATIONS = []string{"/usr/sbin/useradd", "/sbin/useradd"}
)

func CreateInoxdUserIfNotExists(out io.Writer, errOut io.Writer) error {
	username := INOXD_USERNAME
	usr, err := user.Lookup(username)

	if _, ok := err.(user.UnknownUserError); ok {
		fmt.Fprintf(out, "user `%s` does not exist, create it\n", username)

		//create user

		useraddPath, err := exec.LookPath("useradd")
		if err != nil {
			return err
		}
		if !slices.Contains(ALLOWED_USERADD_LOCATIONS, useraddPath) {
			return fmt.Errorf("the binary for the useradd command has an unexpected location: %q", useraddPath)
		}

		cmd := exec.Command(useraddPath,
			"-m",                  //create the home directory
			"-s", "/sbin/nologin", //no shell
			username,
		)

		fmt.Fprintln(out, cmd.String())

		err = cmd.Run()
		return err
	} else if err != nil {
		return err
	} else {
		fmt.Fprintf(out, "user `%s` already exists (UID %s)\n", username, usr.Uid)
	}

	return nil
}
