package compressarch

import (
	"compress/gzip"
	"io"
)

func UnGzip(writer io.Writer, gzipped io.Reader) error {
	archive, err := gzip.NewReader(gzipped)
	if err != nil {
		return err
	}
	defer archive.Close()
	_, err = io.Copy(writer, archive)
	return err
}
