package compressarch

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/mimeconsts"
)

var (
	WORTH_COMPRESSING_CONTENT_TYPES = map[string]struct{}{
		mimeconsts.JS_CTYPE:   {},
		mimeconsts.CSS_CTYPE:  {},
		mimeconsts.JSON_CTYPE: {},
		mimeconsts.HTML_CTYPE: {},
	}
)

// A FileCompressor is a stateful file content compressor that uses GZip for compression
// and caches compressed files.
type FileCompressor struct {
	cacheLock sync.Mutex
	cache     map[string]*compressedFile
}

func NewFileCompressor() *FileCompressor {
	return &FileCompressor{
		cache: make(map[string]*compressedFile),
	}
}

type compressedFile struct {
	lock                      sync.Mutex
	bytes                     []byte //not reused between updates, nil if isCompressedContentLarger.
	lastFileMtime             time.Time
	isCompressedContentLarger bool
}

type ContentCompressionParams struct {
	Ctx           context.Context
	ContentReader io.Reader
	Path          string
	LastMtime     time.Time
}

// CompressFile compresses the file content if it's worth it. If the file is compressed the returned reader is not
// nil and isCompressed is true. Compressed content is cached for later use. An error is returned if .Ctx is done or if there
// was an error during compression. ContentReader may not be read at all and is never closed.
func (c *FileCompressor) CompressFileContent(args ContentCompressionParams) (_ io.ReadSeeker, isCompressed bool, _ error) {
	ctx := args.Ctx
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}

	contentReader := args.ContentReader
	path := filepath.Clean(args.Path)
	ext := filepath.Ext(path)
	lastMtime := args.LastMtime
	mimeType := mimeconsts.TypeByExtensionWithoutParams(ext)

	if mimeType == "" {
		return nil, false, nil
	}

	if _, ok := WORTH_COMPRESSING_CONTENT_TYPES[mimeType]; !ok {
		return nil, false, nil
	}

	c.cacheLock.Lock()
	entry, found := c.cache[path]
	c.cacheLock.Unlock()

	if found {
		entry.lock.Lock()
		defer entry.lock.Unlock()

		if entry.lastFileMtime.Equal(lastMtime) {
			//content has not changed.
			if entry.isCompressedContentLarger {
				return nil, false, nil
			}
			return &compressedFileReader{position: 0, bytes: entry.bytes}, true, nil
		}
		entry.lastFileMtime = lastMtime
		entry.isCompressedContentLarger = false
	} else {
		entry = &compressedFile{lastFileMtime: lastMtime}
		entry.lock.Lock()
		defer entry.lock.Unlock()

		c.cache[path] = entry
	}

	buf := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(buf)

	n, err := io.Copy(gzipWriter, contentReader)
	if err != nil {
		// No need to call gw.Close() here since we're discarding the result, and gzip.Writer.Close isn't needed for cleanup.
		return nil, false, err
	}
	err = gzipWriter.Close()
	if err != nil {
		return nil, false, err
	}
	if int64(buf.Len()) >= n {
		entry.isCompressedContentLarger = true
		return nil, false, nil
	}
	entry.bytes = buf.Bytes()
	return &compressedFileReader{position: 0, bytes: entry.bytes}, true, nil
}

type compressedFileReader struct {
	position int64
	bytes    []byte
}

func (r *compressedFileReader) Read(p []byte) (n int, err error) {
	if r.position >= int64(len(r.bytes)) {
		err = io.EOF
	}
	n = copy(p, r.bytes[r.position:])
	r.position += int64(n)
	return n, err
}

func (r *compressedFileReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		r.position += offset
	case io.SeekStart:
		r.position = offset
	case io.SeekEnd:
		r.position = int64(len(r.bytes)) + offset
	default:
		return 0, os.ErrInvalid
	}

	return r.position, nil
}
