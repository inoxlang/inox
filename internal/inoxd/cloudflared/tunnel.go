package cloudflared

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type CreateTunnelParams struct {
	UniqueName            string
	OriginCertificatePath string

	OutW, ErrW io.Writer
}

type TunnelSensitiveData struct {
	Id           string          `json:"id"`
	CurrentToken string          `json:"token"`
	CredsContent json.RawMessage `json:"creds"`
}

func CreateTunnel(args CreateTunnelParams) (data TunnelSensitiveData, finalErr error) {
	if yes, err := isRunningAsRoot(); err != nil {
		finalErr = err
		return
	} else if !yes {
		finalErr = errors.New("not running as root")
		return
	}

	//prepare the command
	cmd := exec.Command(DEFAULT_CLOUDFLARED_BINARY_PATH, "tunnel", "create", "-o", "json", args.UniqueName)
	buff := bytes.NewBuffer(nil)

	cmd.Stdin = nil
	cmd.Stdout = buff //don't print tunnel information to the logs.
	cmd.Stderr = args.ErrW

	cmd.Env = append(cmd.Env, "TUNNEL_ORIGIN_CERT="+args.OriginCertificatePath)

	//execute the command

	fmt.Fprintln(args.OutW, "create cloudflare tunnel", args.UniqueName)
	err := cmd.Run()
	if err != nil {
		finalErr = err
		return
	}

	// read the output and the credentials file

	var cmdOutput struct {
		Id    string `json:"id"`
		Token string `json:"token"`
	}
	err = json.Unmarshal(buff.Bytes(), &cmdOutput)
	if err != nil {
		finalErr = fmt.Errorf("failed to unmarshal the command output: %w", err)
		return
	}

	credFilePath := filepath.Join(ROOT_CLOUDFLARED_DIR, cmdOutput.Id+".json")
	credsContent, err := os.ReadFile(credFilePath)
	if err != nil {
		finalErr = err
		return
	}

	return TunnelSensitiveData{
		Id:           cmdOutput.Id,
		CurrentToken: cmdOutput.Token,
		CredsContent: json.RawMessage(credsContent),
	}, nil
}
