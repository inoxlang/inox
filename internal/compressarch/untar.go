package compressarch

import (
	"archive/tar"
	"bytes"
	"io"
	"io/fs"
)

func UntarInMemory(tarball []byte, entryCallbackFunc func(info fs.FileInfo, reader io.Reader) error) error {
	tarReader := tar.NewReader(bytes.NewReader(tarball))

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		info := header.FileInfo()

		err = entryCallbackFunc(info, tarReader)
		if err != nil {
			return err
		}
	}
	return nil
}
