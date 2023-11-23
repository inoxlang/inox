package inoxd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"slices"
	"strconv"
)

const (
	INOXD_USERNAME = "inoxd"
	HOMEDIR_PERMS  = 0740
)

var (
	ALLOWED_USERADD_LOCATIONS = []string{"/usr/sbin/useradd", "/sbin/useradd"}
	ALLOWED_USERDEL_LOCATIONS = []string{"/usr/sbin/userdel", "/sbin/userdel"}
)

func CreateInoxdUserIfNotExists(out io.Writer, errOut io.Writer) (username string, uid int, homedir string, _ error) {
	username = INOXD_USERNAME
	homedir = "/home/" + username
	usr, err := user.Lookup(username)

	if _, ok := err.(user.UnknownUserError); ok {
		fmt.Fprintf(out, "user `%s` does not exist, create it\n", username)

		// Find an available user ID.
		// Note: we set the UID because we need it to create a homedir owned by the user but
		// user.Lookup does not seem to be aware of the new user just after the creation (?).
		// We could call the id command or look in some logs to know the UID but this is unecessarily complicated
		// and less portable (?).

		uid = 1000
		_, err := user.LookupId(strconv.Itoa(uid))
		for err == nil {
			uid++
			_, err = user.LookupId(strconv.Itoa(uid))
		}
		if _, ok := err.(user.UnknownUserIdError); !ok {
			return "", -1, "", err
		}

		// create home dir

		ownerChangeNeeded := false

		if _, err := os.Stat(homedir); os.IsNotExist(err) {
			fmt.Fprintf(out, "create empty homedir %q\n", homedir)

			err := os.MkdirAll(homedir, HOMEDIR_PERMS)
			if err != nil {
				return "", -1, "", err
			}
			ownerChangeNeeded = true
		} else if err != nil {
			return "", -1, "", err
		} else {
			fmt.Fprintf(out, "homedir %q already exists\n", homedir)
		}

		//create user

		useraddPath, err := exec.LookPath("useradd")
		if err != nil {
			return "", -1, "", err
		}
		if !slices.Contains(ALLOWED_USERADD_LOCATIONS, useraddPath) {
			return "", -1, "", fmt.Errorf("the binary for the useradd command has an unexpected location: %q", useraddPath)
		}

		cmd := exec.Command(useraddPath,
			"-u", strconv.Itoa(uid),
			"-d", homedir,
			"-s", "/sbin/nologin", //no shell
			username,
		)

		fmt.Fprintln(out, cmd.String())

		err = cmd.Run()
		if err != nil {
			return "", -1, "", err
		}

		//change the owner of the homedir
		if ownerChangeNeeded {
			fmt.Fprintf(out, "change owner of dir %q to %q\n", homedir, username)
			err := os.Chown(homedir, uid, -1)
			if err != nil {
				return "", -1, "", err
			}
		}
	} else if err != nil {
		return "", -1, "", err
	} else {
		fmt.Fprintf(out, "user `%s` already exists (UID %s)\n", username, usr.Uid)
	}

	return
}

type UserRemovalParams struct {
	RemoveHomedir bool
	ErrOut, Out   io.Writer
}

func RemoveInoxdUser(args UserRemovalParams) error {
	cmdArgs := []string{INOXD_USERNAME}
	if args.RemoveHomedir {
		cmdArgs = append(cmdArgs, "--remove")
	}

	userdelPath, err := exec.LookPath("userdel")
	if err != nil {
		return err
	}
	if !slices.Contains(ALLOWED_USERDEL_LOCATIONS, userdelPath) {
		return fmt.Errorf("the binary for the userdel command has an unexpected location: %q", userdelPath)
	}

	cmd := exec.Command(userdelPath, cmdArgs...)
	cmd.Stdout = args.Out
	cmd.Stderr = args.ErrOut

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("the userdel command failed: %w", err)
	}
	return nil
}
