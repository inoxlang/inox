package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/projectserver"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

func LegacyLSP(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
	panic(errors.New("disabled"))

	//read and check arguments

	flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
	var host string
	flags.StringVar(&host, "h", "", "host")

	if showHelp(flags, mainSubCommandArgs, outW) { //only show help
		return
	}

	err := flags.Parse(mainSubCommandArgs)
	if err != nil {
		fmt.Fprintln(errW, "lsp:", err)
		return
	}

	//create the LSP server configuration from the provided arguments.

	opts := projectserver.LSPServerConfiguration{}
	var out io.Writer

	if host != "" {
		u := checkLspHost(host, errW)
		if u == nil {
			return
		}

		opts.Websocket = &projectserver.WebsocketServerConfiguration{Addr: u.Host}

		out = os.Stdout //we can log to stdout since we will not be in Stdio mode
	} else { //stdio
		f, err := os.OpenFile("/tmp/.inox-lsp.debug.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err != nil {
			log.Panicln(err)
		}
		out = f
		defer f.Close()
	}

	//create context and state

	perms := []core.Permission{
		//TODO: change path pattern
		core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
	}

	if opts.Websocket != nil {
		perms = append(perms, core.WebsocketPermission{Kind_: permbase.Provide})
	}

	filesystem := projectserver.NewDefaultFilesystem()
	initialWorkingDir := utils.Must(os.Getwd())
	ctx := core.NewContext(core.ContextConfig{
		Permissions:             perms,
		Filesystem:              filesystem,
		InitialWorkingDirectory: core.DirPathFrom(initialWorkingDir),
	})

	state := core.NewGlobalState(ctx)
	state.Out = out
	state.Logger = zerolog.New(out)
	state.OutputFieldsInitialized.Store(true)

	//restrict filesystem access at the process level and  start the LSP server.

	inoxprocess.RestrictProcessAccess(ctx, inoxprocess.ProcessRestrictionConfig{AllowBrowserAccess: true})

	if err := projectserver.StartLSPServer(ctx, opts); err != nil {
		fmt.Fprintln(errW, "failed to start LSP server:", err)
	}

	return 0
}
