package internal

import (
	"log"
	"os"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

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

func StartLSPServer() {

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

	server := lsp.NewServer(&lsp.Options{
		CompletionProvider: &defines.CompletionOptions{
			TriggerCharacters: &[]string{"."},
		},
		TextDocumentSync: defines.TextDocumentSyncKindFull,
	})

	filesystem := NewFilesystem()

	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			//TODO: change path pattern
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		},
		Filesystem: filesystem,
	})
	core.NewGlobalState(compilationCtx)

	registerHandlers(server, filesystem, compilationCtx)
	server.Run()
}
