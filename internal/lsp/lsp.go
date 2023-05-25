package internal

import (
	"io"
	"log"
	"os"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"

	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	_ "net/http/pprof"
)

const (
	JSONRPC_VERSION = "2.0"
)

var HOVER_PRETTY_PRINT_CONFIG = &pprint.PrettyPrintConfig{
	MaxDepth: 7,
	Indent:   []byte{' ', ' '},
	Colorize: false,
	Compact:  false,
}

type LSPServerOptions struct {
	WASM struct {
		StdioInput  io.Reader
		StdioOutput io.Writer
	}
}

func StartLSPServer(opts LSPServerOptions) {

	f, err := os.OpenFile("/tmp/.inox-lsp.debug.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		log.Panicln(err)
	}

	logger := log.New(f, "", 0)
	logs.Init(logger)

	defer func() {
		e := recover()

		if e != nil {
			logs.Println(e)
		}

		f.Close()
	}()

	options := &lsp.Options{
		CompletionProvider: &defines.CompletionOptions{
			TriggerCharacters: &[]string{"."},
		},
		TextDocumentSync: defines.TextDocumentSyncKindFull,
	}

	if opts.WASM.StdioInput != nil {
		options.StdioInput = opts.WASM.StdioInput
		options.StdioOutput = opts.WASM.StdioOutput
	}

	server := lsp.NewServer(options)

	filesystem := NewFilesystem()

	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			//TODO: change path pattern
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		},
		Filesystem: filesystem,
	})
	state := core.NewGlobalState(compilationCtx)
	state.Logger = zerolog.New(utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			logs.Println(utils.BytesAsString(p))
			return len(p), nil
		},
	})

	registerHandlers(server, filesystem, compilationCtx)
	server.Run()
}
