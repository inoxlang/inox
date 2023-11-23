package cloudflared

import (
	"errors"
	"io"
	"os"
	"os/exec"
)

const (
	CERT_PEM_LOCATION = "/root/.cloudflared/cert.pem"
)

func Login(outW, errW io.Writer) (certPemLocation string, _ error) {
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

	if _, err := os.Stat(CERT_PEM_LOCATION); err != nil {
		return "", errors.New("command succeeded but no cert.pem file was created")
	}

	return CERT_PEM_LOCATION, nil
}
