package http_ns

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/compressarch"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/mimeconsts"
)

type fileServingParams struct {
	ctx            *core.Context
	rw             http.ResponseWriter
	r              *http.Request
	pth            core.Path
	fileCompressor *compressarch.FileCompressor //optional
}

// serveFile opens the file at the specified path, tries to compress the content, and calls http.ServeContent.
// An error is returned in the following cases:
// - permission error (read perm is required).
// - failure to open the file.
// - failure to get creation & modification time from file info.
// Note that errors can still happen during the call to http.ServeContent but they are written to the *http.ResponseWriter.
func serveFile(args fileServingParams) error {
	ctx := args.ctx
	pth := args.pth

	fls := ctx.GetFileSystem()
	{
		var err error
		pth, err = pth.ToAbs(fls)
		if err != nil {
			return err
		}
	}

	perm := core.FilesystemPermission{Kind_: permbase.Read, Entity: pth}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	f, err := fls.Open(string(pth))
	if err != nil {
		return err
	}
	defer f.Close()

	//TODO: add implementation of afs.StatCapable to all file types in fs_ns.

	stat, err := afs.FileStat(f, fls)
	if err != nil {
		return err
	}

	modTime := stat.ModTime()

	var responseContent io.ReadSeeker = f

	if args.fileCompressor != nil {
		compressed, isCompressed, err := args.fileCompressor.CompressFileContent(compressarch.ContentCompressionParams{
			Ctx:           ctx,
			ContentReader: f,
			Path:          pth.UnderlyingString(),
			LastMtime:     modTime,
		})
		if err != nil {
			return err
		}
		if isCompressed {
			responseContent = compressed
			headers := args.rw.Header()

			//add Content-Type header because http.ServeContent won't be able
			//to easily detect the original content type after content has been gzip compressed.

			fileExtension := filepath.Ext(pth.UnderlyingString())
			mimeType := mime.TypeByExtension(fileExtension)
			if mimeType == "" {
				mimeType = mimeconsts.APP_OCTET_STREAM_CTYPE
			}

			headers.Set("Content-Type", mimeType)

			//add Content-Encoding header.
			headers.Set("Content-Encoding", "gzip")
		}
	}

	//TODO: pass a custom response writer that logs error and return a 404 status code.

	http.ServeContent(args.rw, args.r, string(pth), modTime, responseContent)
	return nil
}
