//go:build js

package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"syscall/js"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	_ "github.com/inoxlang/inox/internal/globals"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/rs/zerolog"

	lsp "github.com/inoxlang/inox/internal/project_server"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	LSP_INPUT_BUFFER_SIZE  = 5_000_000
	LSP_OUTPUT_BUFFER_SIZE = 5_000_000
	FS_SAVE_INTERVAL       = 1 * time.Second
	MAX_FS_STORAGE         = core.ByteCount(10_000_000)

	DATE_FORMAT = time.RFC3339

	OUT_PREFIX = "[vscode-inox]"
)

var printDebug *js.Value
var printTrace *js.Value
var getFileContent *js.Value
var getFilesystemMetadata *js.Value
var saveContent *js.Value
var saveFilesystemMetadata *js.Value

func main() {
	ctx := core.NewContext(core.ContextConfig{})
	state := core.NewGlobalState(ctx)
	state.Out = utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			if printDebug != nil {
				printDebug.Invoke(OUT_PREFIX, string(p))
			}
			return len(p), nil
		},
	}
	state.Logger = zerolog.New(state.Out)

	setupDoneChan := make(chan struct{}, 1)
	inputMessageChannel := make(chan string, 10)
	outputMessageChannel := make(chan string, 10)

	registerCallbacks(inputMessageChannel, outputMessageChannel, setupDoneChan)

	fmt.Println(OUT_PREFIX, "wait for setup() to be called by JS")
	<-setupDoneChan
	close(setupDoneChan)

	printDebug.Invoke(OUT_PREFIX, "create context & state for LSP server")

	serverCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.CreateFsReadPerm(core.INITIAL_WORKING_DIR_PATH_PATTERN),
		},
	})
	{
		serverState := core.NewGlobalState(serverCtx)
		serverState.Out = utils.FnWriter{
			WriteFn: func(p []byte) (n int, err error) {
				printDebug.Invoke(OUT_PREFIX, string(p))
				return len(p), nil
			},
		}
		serverState.Logger = zerolog.New(state.Out)
	}

	printDebug.Invoke(OUT_PREFIX, "start LSP server")

	go lsp.StartLSPServer(serverCtx, lsp.LSPServerOptions{
		ProjectMode:      true,
		UseContextLogger: true,
		MessageReaderWriter: jsonrpc.FnMessageReaderWriter{
			ReadMessageFn: func() (msg []byte, err error) {
				s, ok := <-inputMessageChannel
				if !ok {
					printTrace.Invoke(OUT_PREFIX, "input message channel is closed")
					return nil, io.EOF
				}
				printTrace.Invoke(OUT_PREFIX, fmt.Sprintf("%d-byte message read from input message channel", len(s)))

				return []byte(s), nil
			},
			WriteMessageFn: func(msg []byte) error {
				outputMessageChannel <- string(msg)
				return nil
			},
			CloseFn: func() error {
				close(inputMessageChannel)
				close(outputMessageChannel)
				return nil
			},
		},
		OnSession: func(rpcCtx *core.Context, s *jsonrpc.Session) error {
			printDebug.Invoke(OUT_PREFIX, "new LSP session")

			var sessionCtx *core.Context
			sessionCtx = core.NewContext(core.ContextConfig{
				Permissions:          rpcCtx.GetGrantedPermissions(),
				ForbiddenPermissions: rpcCtx.GetForbiddenPermissions(),

				ParentContext: rpcCtx,
				CreateFilesystem: func(ctx *core.Context) (afs.Filesystem, error) {
					mainFs, err := createFilesystem()
					if err != nil {
						printDebug.Invoke(OUT_PREFIX, err.Error())

						return nil, fmt.Errorf("failed to create filesystem for session: %w", err)
					}

					go func() {
						for {
							select {
							case <-sessionCtx.Done():
							case <-time.After(FS_SAVE_INTERVAL):
								saveFilesystem(mainFs)
							}
						}
					}()

					return lsp.NewFilesystem(mainFs, fs_ns.NewMemFilesystem(lsp.DEFAULT_MAX_IN_MEM_FS_STORAGE_SIZE)), nil
				},
			})
			tempState := core.NewGlobalState(sessionCtx)
			tempState.Logger = state.Logger
			tempState.Out = state.Out
			s.SetContextOnce(sessionCtx)

			printDebug.Invoke(OUT_PREFIX, "context of LSP session created")
			return nil
		},
	})

	fmt.Println(OUT_PREFIX, "end of main() reached: block with channel")

	channel := make(chan struct{})
	<-channel
}

func registerCallbacks(inputMessageChannel chan string, outputMessageChannel chan string, setupDoneChan chan struct{}) {
	exports := js.Global().Get("exports")

	exports.Set("write_lsp_message", js.FuncOf(func(this js.Value, args []js.Value) any {
		if printTrace != nil {
			printTrace.Invoke(OUT_PREFIX, "write_lsp_message() called by JS")
		}

		s := args[0].String()
		inputMessageChannel <- s
		return js.ValueOf(nil)
	}))

	exports.Set("read_lsp_message", js.FuncOf(func(this js.Value, args []js.Value) any {
		if printTrace != nil {
			printTrace.Invoke(OUT_PREFIX, "read_lsp_message() called by JS")
		}

		select {
		case msg := <-outputMessageChannel:
			return js.ValueOf(msg)
		case <-time.After(100 * time.Millisecond):
			return js.ValueOf(nil)
		}
	}))

	exports.Set("setup", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println(OUT_PREFIX, "setup() called by JS")

		if printDebug != nil {
			fmt.Println("setup() already called by JS !")
			return js.ValueOf("")
		}

		IWD := args[0].Get(core.INITIAL_WORKING_DIR_VARNAME).String()
		debug := args[0].Get("print_debug")
		printDebug = &debug

		trace := args[0].Get("print_trace")
		printTrace = &trace

		get := args[0].Get("get_file_content")
		getFileContent = &get

		getMetadata := args[0].Get("get_filesystem_metadata")
		getFilesystemMetadata = &getMetadata

		save := args[0].Get("save_file_content")
		saveContent = &save

		save_metadata := args[0].Get("save_filesystem_metadata")
		saveFilesystemMetadata = &save_metadata

		core.SetInitialWorkingDir(func() (string, error) {
			return IWD, nil
		})

		setupDoneChan <- struct{}{}

		return js.ValueOf("ok")
	}))

	fmt.Println(OUT_PREFIX, "handlers registered")
}

func saveFilesystem(fls *fs_ns.MemFilesystem) {
	snapshot := fls.TakeFilesystemSnapshot(func(ChecksumSHA256 [32]byte) fs_ns.AddressableContent {
		return nil
	})

	//save metadata
	metadata := []any{}

	for path, fileMetadata := range snapshot.Metadata {
		fileMetadataJSON := map[string]any{
			"path":         path,
			"absPath":      fileMetadata.AbsolutePath.UnderlyingString(),
			"creationTime": time.Time(fileMetadata.CreationTime).Format(DATE_FORMAT),
			"modifTime":    time.Time(fileMetadata.ModificationTime).Format(DATE_FORMAT),
			"mode":         fmt.Sprint(fileMetadata.Mode),
		}
		if fileMetadata.Mode.FileMode().IsDir() {
			fileMetadataJSON["childNames"] = utils.MapSlice(fileMetadata.ChildNames, func(s string) any { return s })
		} else {
			fileMetadataJSON["checksumSHA256"] = hex.EncodeToString(fileMetadata.ChecksumSHA256[:])
			fileMetadataJSON["size"] = fmt.Sprint(fileMetadata.Size)
		}
		metadata = append(metadata, fileMetadataJSON)
	}

	saveFilesystemMetadata.Invoke(metadata)

	//save file contents

	for _, content := range snapshot.FileContents {
		encodedContent := base64.StdEncoding.EncodeToString(utils.Must(io.ReadAll(content.Reader())))
		checksum := content.ChecksumSHA256()
		encodedChecksum := hex.EncodeToString(checksum[:])

		saveContent.Invoke(encodedChecksum, encodedContent)
	}
}

func createFilesystem() (*fs_ns.MemFilesystem, error) {
	results := getFilesystemMetadata.Invoke()
	metadata := results.Index(0)

	if metadata.Equal(js.Null()) {
		fls := fs_ns.NewMemFilesystem(MAX_FS_STORAGE)
		file := utils.Must(fls.Create("/main.ix"))
		if _, err := file.Write([]byte("manifest {\n\n}")); err != nil {
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
		return fls, nil
	}

	snapshot := fs_ns.FilesystemSnapshot{
		Metadata:     map[string]*fs_ns.FileMetadata{},
		FileContents: map[string]fs_ns.AddressableContent{},
	}

	//get metadata
	for i := 0; i < metadata.Length(); i++ {
		fileMetadata := metadata.Index(i)
		path := fileMetadata.Get("path").String()
		absPath := fileMetadata.Get("absPath").String()
		modeVal, err := strconv.ParseUint(fileMetadata.Get("mode").String(), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse .mode for file %s: %w", path, err)
		}
		mode := fs.FileMode(modeVal)

		creationTime, err := time.Parse(DATE_FORMAT, fileMetadata.Get("creationTime").String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse .creationTime date for file %s: %w", path, err)
		}
		modifTime, err := time.Parse(DATE_FORMAT, fileMetadata.Get("modifTime").String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse .modifTime date for file %s: %w", path, err)
		}

		metadata := &fs_ns.FileMetadata{
			AbsolutePath:     core.Path(absPath),
			CreationTime:     core.Date(creationTime),
			ModificationTime: core.Date(modifTime),
			Mode:             core.FileMode(mode),
		}
		snapshot.Metadata[path] = metadata

		var childNames []string
		if mode.IsDir() {
			array := fileMetadata.Get("childNames")
			for i := 0; i < array.Length(); i++ {
				childNames = append(childNames, array.Index(i).String())
			}
			metadata.ChildNames = childNames
		} else {
			size, err := strconv.ParseInt(fileMetadata.Get("size").String(), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse .size for file %s: %w", path, err)
			}

			checksumSha256, err := hex.DecodeString(fileMetadata.Get("checksumSHA256").String())
			if err != nil {
				return nil, fmt.Errorf("failed to decode .checksumSHA256 for file %s: %w", path, err)
			}

			metadata.Size = core.ByteCount(size)
			metadata.ChecksumSHA256 = [32]byte(checksumSha256)
		}
	}

	//get contents

	for path, metadata := range snapshot.Metadata {
		if metadata.Mode.FileMode().IsDir() {
			continue
		}
		checksum := hex.EncodeToString(metadata.ChecksumSHA256[:])
		results := getFileContent.Invoke(checksum)

		errString := results.Index(1).String()
		if errString != "" {
			return nil, fmt.Errorf("failed to get content of file %s: %s", path, errString)
		}
		encodedContent := results.Index(0).String()
		content, err := base64.StdEncoding.DecodeString(encodedContent)
		if err != nil {
			return nil, fmt.Errorf("failed to decode content of file %s: %s", path, err)
		}
		snapshot.FileContents[path] = fs_ns.AddressableContentBytes{
			Sha256: metadata.ChecksumSHA256,
			Data:   content,
		}
	}

	return fs_ns.NewMemFilesystemFromSnapshot(snapshot, MAX_FS_STORAGE), nil
}
