package cloudflared

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// LoginToGetOriginCertificate executes the cloudlfared login command that shows a link leading to the Cloudflare dashboard
// to allow the installation of a certificate. The created cert.pem file is read and its content is returned.
func LoginToGetOriginCertificate(outW, errW io.Writer) (certPemContent string, _ error) {
	if yes, err := isRunningAsRoot(); err != nil {
		return "", err
	} else if !yes {
		return "", errors.New("not running as root")
	}

	//the command will do nothing if cert.pem already exists.
	cmd := exec.Command(DEFAULT_CLOUDFLARED_BINARY_PATH, "login")

	cmd.Stdin = nil
	cmd.Stdout = outW
	cmd.Stderr = errW

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	bytes, err := os.ReadFile(CERT_PEM_LOCATION)

	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("command succeeded but no cert.pem file was created")
		}
		return "", fmt.Errorf("failed to read the content of origin cerficate: %w", err)
	}

	return string(bytes), nil
}
