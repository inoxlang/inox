package main

import (
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/tailwind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

func Controlled(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
	//read & parse arguments

	if len(mainSubCommandArgs) != 4 {
		fmt.Fprintln(errW, "4 arguments are expected after the subcommand name")
		return
	}

	u, err := url.Parse(mainSubCommandArgs[0])
	if err != nil {
		fmt.Fprintln(errW, "first argument is not a valid URL: %w", err)
		return
	}

	token, ok := inoxprocess.ControlledProcessTokenFrom(mainSubCommandArgs[1])
	if !ok {
		fmt.Fprintln(errW, "second argument is not a valid process token: %w", err)
		return
	}

	// decode the permissions of the controlled process
	core.RegisterPermissionTypesInGob()
	core.RegisterSimpleValueTypesInGob()

	decoder := gob.NewDecoder(hex.NewDecoder(strings.NewReader(mainSubCommandArgs[2])))
	var grantedPerms []core.Permission

	err = decoder.Decode(&grantedPerms)
	if err != nil {
		fmt.Fprintf(errW, "third argument is not a valid encoding of permissions: %s\n", err.Error())
		return
	}

	decoder = gob.NewDecoder(hex.NewDecoder(strings.NewReader(mainSubCommandArgs[3])))
	var forbiddenPerms []core.Permission

	err = decoder.Decode(&forbiddenPerms)
	if err != nil {
		fmt.Fprintf(errW, "fourth argument is not a valid encoding of permissions: %s\n", err.Error())
		return
	}

	// connect to the control server
	ctx := core.NewContext(core.ContextConfig{
		Permissions:             grantedPerms,
		ForbiddenPermissions:    forbiddenPerms,
		Filesystem:              fs_ns.GetOsFilesystem(),
		InitialWorkingDirectory: core.DirPathFrom(utils.Must(os.Getwd())),
		Limits:                  core.GetDefaultScriptLimits(),
	})
	state := core.NewGlobalState(ctx)
	state.Out = os.Stdout
	state.Logger = zerolog.New(state.Out)
	state.OutputFieldsInitialized.Store(true)

	inoxprocess.RestrictProcessAccess(ctx, inoxprocess.ProcessRestrictionConfig{
		AllowBrowserAccess: true,
		BrowserBinPath:     chrome_ns.BROWSER_BINPATH,
	})

	client, err := inoxprocess.ConnectToProcessControlServer(ctx, u, token)
	if err != nil {
		fmt.Fprintln(errW, err)
		return
	}

	CancelOnSigintSigterm(state.Ctx, ROOT_CTX_TEARDOWN_TIMEOUT)

	//Initializations.

	tailwind.InitSubset() //TODO: add condition

	//Start the control loop.

	_ = client.StartControl()
	return 0
}
