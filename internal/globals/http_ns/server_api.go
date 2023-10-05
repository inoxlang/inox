package http_ns

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
)

func getServerAPI(ctx *core.Context, server *HttpServer) {

}

func getFilesystemRoutingServerAPI(ctx *core.Context, fls afs.Filesystem, dir string) (*API, error) {
	api := &API{}

	err := addFilesysteDirEndpoints(ctx, api, dir, "/", fls)
	if err != nil {
		return nil, err
	}

	return api, nil
}

// addFilesysteDirEndpoints recursively add the endpoints provided by dir and its subdirectories.
func addFilesysteDirEndpoints(ctx *core.Context, api *API, dir string, urlDirPath string, fls afs.Filesystem) error {
	entries, err := fls.ReadDir(dir)

	if err != nil {
		return err
	}

	urlDirPathNoTrailingSlash := strings.TrimSuffix(urlDirPath, urlDirPath)
	_ = urlDirPathNoTrailingSlash
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		entryName := entry.Name()
		absEntryPath := filepath.Join(dir, entryName)

		if entry.IsDir() {
			subDir := absEntryPath + "/"
			urlSubDir := filepath.Join(urlDirPath, entryName) + "/"

			err := addFilesysteDirEndpoints(ctx, api, subDir, urlSubDir, fls)
			if err != nil {
				return err
			}
			continue
		}

		//ignore non-Inox files
		if strings.HasSuffix(entryName, INOX_FILE_EXTENSION) {
			continue
		}

		entryNameNoExt := strings.TrimSuffix(entryName, INOX_FILE_EXTENSION)

		var endpointPath string

		if slices.Contains(FS_ROUTING_METHODS, entryNameNoExt) { //GET.ix, POST.ix, ...
			//add operation
			endpointPath = urlDirPathNoTrailingSlash
		} else {
			method, name, ok := strings.Cut(entryNameNoExt, "-")

			if ok && slices.Contains(FS_ROUTING_METHODS, method) { //POST-... , GET-...
				endpointPath = filepath.Join(urlDirPath, name)
			} else {
				endpointPath = filepath.Join(urlDirPath, entryNameNoExt)
			}
		}

		//add operation
		endpt := api.endpoints[endpointPath]
		if endpt == nil {
			endpt = &ApiEndpoint{}
			api.endpoints[endpointPath] = endpt
		}
		endpt.operations = append(endpt.operations, ApiOperation{})
		// operation := &endpt.operations[len(endpt.operations)-1]

		// chunk, err := core.ParseFileChunk(absEntryPath, fls)
		// if err != nil {
		// 	return fmt.Errorf("failed to parse %q: %w", absEntryPath, err)
		// }

		// if chunk.Node.Manifest == nil { //not a module
		// 	return fmt.Errorf("%q is not a module")
		// }

		// state, mod, _, err := mod.PrepareLocalScript(mod.ScriptPreparationArgs{
		// 	Fpath:                     absEntryPath,
		// 	ParsingCompilationContext: ctx,

		// 	ParentContext:         parentCtx,
		// 	ParentContextRequired: parentCtx != nil,

		// 	Out:                     io.Discard,
		// 	DataExtractionMode:      true,
		// 	ScriptContextFileSystem: fls,
		// 	PreinitFilesystem:       fls,

		// 	Project: project,
		// })

	}

	return nil
}
