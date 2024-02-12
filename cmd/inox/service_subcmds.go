package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/inoxlang/inox/internal/inoxd"
	"github.com/inoxlang/inox/internal/inoxd/cloudflared"
	"github.com/inoxlang/inox/internal/inoxd/systemd"
	"github.com/inoxlang/inox/internal/utils"

	inoxdconsts "github.com/inoxlang/inox/internal/inoxd/consts"
)

func InstallService(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {

	//read and check arguments

	flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
	var inoxCloud bool
	var tunnelProvider string
	var exposeProjectServers bool
	var exposeWebServers bool
	var allowBrowserAutomation bool

	flags.BoolVar(&inoxCloud, "inox-cloud", false, "enable inox cloud")
	flags.StringVar(&tunnelProvider, "tunnel-provider", "", "name of the tunnel provider, only 'cloudflare' is supported for now")
	flags.BoolVar(&exposeProjectServers, "expose-project-servers", false, "allow project servers to bind to all interfaces")
	flags.BoolVar(&exposeWebServers, "expose-web-servers", false, "allow web servers to bind to all interfaces")
	flags.BoolVar(&allowBrowserAutomation, "allow-browser-automation", false, "allow project code to create and control a browser, and allow project servers to download a chromium binary if no browser is installed")

	if showHelp(flags, mainSubCommandArgs, outW) { //only show help
		return
	}

	err := flags.Parse(mainSubCommandArgs)
	if err != nil {
		fmt.Fprintln(errW, "ERROR:", err)
		return ERROR_STATUS_CODE
	}

	if tunnelProvider != "" && tunnelProvider != "cloudflare" {
		fmt.Fprintln(errW, "ERROR: only 'cloudflare' is supported as a tunnel provider for now")
		return ERROR_STATUS_CODE
	}

	if tunnelProvider != "" && exposeProjectServers {
		fmt.Fprintln(errW, "--expose-project-servers and --tunnel-provider are mutually exclusive flags")
		return ERROR_STATUS_CODE
	}

	if tunnelProvider != "" && exposeWebServers {
		fmt.Fprintln(errW, "--expose-web-servers and --tunnel-provider are mutually exclusive flags")
		return ERROR_STATUS_CODE
	}

	if inoxCloud && exposeProjectServers {
		fmt.Fprintln(errW, "--expose-project-servers and --inox-cloud are mutually exclusive flags")
		return ERROR_STATUS_CODE
	}

	if inoxCloud && exposeWebServers {
		fmt.Fprintln(errW, "--expose-web-servers and --inox-cloud are mutually exclusive flags")
		return ERROR_STATUS_CODE
	}

	//create the inoxd user and add the inoxd unit.

	if err := systemd.CheckFileDoesNotExist(); err != nil {
		fmt.Fprintln(errW, "ERROR:", err)
		return ERROR_STATUS_CODE
	}

	username, uid, homedir, err := inoxd.CreateInoxdUserIfNotExists(outW, errW)
	if err != nil {
		fmt.Fprintln(errW, "ERROR:", err)
		return ERROR_STATUS_CODE
	}
	utils.PrintSmallLineSeparator(outW)

	if tunnelProvider != "" {

		fmt.Fprintln(outW, "download cloudflared")
		binary, err := cloudflared.DownloadLatestBinaryFromGithub()
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}

		fmt.Fprintln(errW, "install the cloudflared binary")
		err = cloudflared.InstallBinary(binary)
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}
	}

	envFilePath, err := systemd.CreateInoxdEnvFileIfNotExists(outW, systemd.EnvFileCreationParams{})

	if err != nil {
		fmt.Fprintln(errW, "ERROR:", err)
		return ERROR_STATUS_CODE
	}
	utils.PrintSmallLineSeparator(outW)

	unitName, err := systemd.WriteInoxUnitFile(systemd.InoxUnitParams{
		Log: outW,

		Username: username,
		Homedir:  homedir,
		UID:      uid,

		ProjectsDir: inoxdconsts.PROJECTS_DIR,
		ProdDir:     inoxdconsts.PROD_DIR,

		InoxCloud: inoxCloud,

		EnvFilePath:            envFilePath,
		TunnelProviderName:     tunnelProvider,
		ExposeProjectServers:   exposeProjectServers,
		ExposeWebServers:       exposeWebServers,
		AllowBrowserAutomation: allowBrowserAutomation,
	})

	alreadyExists := errors.Is(err, systemd.ErrUnitFileExists)
	if err != nil {
		if alreadyExists {
			fmt.Fprintln(outW, err)
		} else {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}
	} else {
		fmt.Fprintln(outW, "unit file created")
		utils.PrintSmallLineSeparator(outW)
	}

	mkDir := func(dir string) {
		fmt.Fprintf(outW, "create directory %s and change its owner to %q\n", dir, username)
		os.MkdirAll(dir, 0700)
		os.Chown(dir, uid, -1)
		utils.PrintSmallLineSeparator(outW)
	}

	mkDir(inoxdconsts.DATA_DIR)
	mkDir(inoxdconsts.PROJECTS_DIR)
	mkDir(inoxdconsts.PROD_DIR)

	// enable & start inoxd
	if !alreadyExists {
		err = systemd.EnableInoxd(unitName, outW, errW)
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}
	}
	utils.PrintSmallLineSeparator(outW)

	restart := alreadyExists

	err = systemd.StartInoxd(unitName, restart, outW, errW)
	if err != nil {
		fmt.Fprintln(errW, "ERROR:", err)
		return
	}
	fmt.Fprintln(outW, "")

	return 0
}

func RemoveService(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
	//read and check arguments

	flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
	var removeTunnelConfigs bool
	var removeInoxdUser bool
	var removeInoxdHomedir bool
	var removeEnvFile bool
	var removeDataDir bool
	var removeAll bool

	flags.BoolVar(&removeTunnelConfigs, "remove-tunnel-configs", false, "remove all configuration files of tunnels")
	flags.BoolVar(&removeInoxdUser, "remove-inoxd-user", false, " remove the inoxd user, the homedir is not removed")
	flags.BoolVar(&removeInoxdHomedir, "remove-inoxd-homedir", false, "if --remove-inoxd-user is present the homedir is also removed")
	flags.BoolVar(&removeEnvFile, "remove-env-file", false, "remove the environment file specified in the unit file")
	flags.BoolVar(&removeDataDir, "dangerously-remove-data-dir", false, "DANGER: remove the data directory "+inoxdconsts.DATA_DIR+", it contains projects and production data")
	flags.BoolVar(&removeAll, "dangerously-remove-all", false, "DANGER: enable all --remove-xxx flags and --dangerously-remove-data-dir")

	if showHelp(flags, mainSubCommandArgs, outW) { //only show help
		return
	}

	err := flags.Parse(mainSubCommandArgs)
	if err != nil {
		fmt.Fprintln(errW, "ERROR:", err)
		return ERROR_STATUS_CODE
	}

	if removeAll {
		removeTunnelConfigs = true
		removeInoxdUser = true
		removeInoxdHomedir = true
		removeEnvFile = true
		removeDataDir = true
	}

	//perform removal(s)

	if removeTunnelConfigs {
		err = cloudflared.RemoveCloudflaredDir(outW)
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}
		utils.PrintSmallLineSeparator(outW)
	}

	if err := systemd.StopRemoveUnit(removeEnvFile, outW, errW); err != nil {
		fmt.Fprintln(errW, "ERROR:", err)
		//keep going
		utils.PrintSmallLineSeparator(outW)
	}

	if removeDataDir {
		fmt.Fprintln(outW, "remove ", inoxdconsts.DATA_DIR)
		err := os.RemoveAll(inoxdconsts.DATA_DIR)
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			//keep going
		}
		utils.PrintSmallLineSeparator(outW)
	}

	if removeInoxdUser {
		err = inoxd.RemoveInoxdUser(inoxd.UserRemovalParams{
			RemoveHomedir: removeInoxdHomedir,
			ErrOut:        errW,
			Out:           outW,
		})
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}
	}

	return 0
}
