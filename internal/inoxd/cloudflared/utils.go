package cloudflared

import (
	"os/user"
)

func isRunningAsRoot() (bool, error) {
	currentUser, err := user.Current()
	if err != nil {
		return false, err
	}

	return currentUser.Uid == "0", nil
}
