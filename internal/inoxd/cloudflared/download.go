package cloudflared

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
)

const (
	LINUX_ADM64_BINARY_URL    = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64"
	CLOUDFLARED_BINARY_FPERMS = fs.FileMode(0555)

	DEFAULT_CLOUDFLARED_BINARY_NAME = "cloudflared"
	DEFAULT_CLOUDFLARED_BINARY_PATH = "/usr/local/bin/" + DEFAULT_CLOUDFLARED_BINARY_NAME
)

func DownloadLatestBinaryFromGithub() ([]byte, error) {
	resp, err := http.Get(LINUX_ADM64_BINARY_URL)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return nil, err
	}

	bytes, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to download the cloudflared binary: %w", err)
	}

	return bytes, nil
}

func InstallBinary(bytes []byte) error {
	if yes, err := isRunningAsRoot(); err != nil {
		return err
	} else if !yes {
		return errors.New("installing the cloudflared binary requires to be running as root")
	}

	return os.WriteFile(DEFAULT_CLOUDFLARED_BINARY_PATH, bytes, CLOUDFLARED_BINARY_FPERMS)
}
