package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"
)

func CloudProxy(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {

	//read & check arguments
	flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
	var configOrConfigFile string

	flags.StringVar(&configOrConfigFile, "config", "", "JSON configuration or JSON file")

	if showHelp(flags, mainSubCommandArgs, outW) { //only show help
		return
	}

	err := flags.Parse(mainSubCommandArgs)
	if err != nil {
		fmt.Fprintln(errW, "cloud-proxy:", err)
		return
	}

	var proxyConfig cloudproxy.CloudProxyConfig

	configOrConfigFile = strings.TrimSpace(configOrConfigFile)
	if configOrConfigFile != "" {
		if configOrConfigFile[0] == '{' {
			err := json.Unmarshal([]byte(configOrConfigFile), &proxyConfig)
			if err != nil {
				fmt.Fprintln(errW, "cloud-proxy: failed to unmarshal configuration argument", err)
				return ERROR_STATUS_CODE
			}
		} else {
			content, err := os.ReadFile(configOrConfigFile)
			if err != nil {
				fmt.Fprintln(errW, "cloud-proxy: failed to read configuration file:", err)
				return ERROR_STATUS_CODE
			}
			err = json.Unmarshal(content, &proxyConfig)
			if err != nil {
				fmt.Fprintln(errW, "cloud-proxy: failed to unmarshal configuration file:", err)
				return ERROR_STATUS_CODE
			}
		}
	} //else empty configuration

	//proxy

	err = cloudproxy.Run(cloudproxy.CloudProxyArgs{
		Config:                proxyConfig,
		OutW:                  outW,
		ErrW:                  errW,
		GoContext:             context.Background(),
		RestrictProcessAccess: true,
		Filesystem:            fs_ns.GetOsFilesystem(),
	})
	if err != nil {
		fmt.Fprintln(errW, err)
		return ERROR_STATUS_CODE
	}

	return 0
}
