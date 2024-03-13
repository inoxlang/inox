package binary

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/klauspost/compress/zip"
)

const (
	WANTED_FILE_PERMISSIONS = fs.FileMode(0500)
	MAX_DENO_BINARY_SIZE    = 200_000_000
)

func Install(location string) error {
	err := tryInstall(location)

	if err == nil {
		return nil
	}

	//Remove the file (or directory) at $location and try again.
	removeErr := os.RemoveAll(location)
	if removeErr != nil {
		return err
	}
	return tryInstall(location)
}

func tryInstall(location string) error {

	assetInfo, archiveInfo, err := GetArchiveAssetInfo()

	if err != nil {
		return err
	}

	f, err := os.OpenFile(location, os.O_RDONLY, 0)

	if errors.Is(err, os.ErrNotExist) {
		//Download the archive, and extract the binary from it.

		_, p, err := DownloadArchive(archiveInfo, assetInfo)
		if err != nil {
			return err
		}
		reader, err := zip.NewReader(bytes.NewReader(p), int64(len(p)))
		if err != nil {
			return err
		}

		fileInZip, err := reader.Open("deno")
		if err != nil {
			return err
		}

		f, err := os.OpenFile(location, os.O_WRONLY|os.O_CREATE, WANTED_FILE_PERMISSIONS)
		if err != nil {
			return err
		}

		defer f.Close()

		_, err = io.Copy(f, fileInZip)
		return err
	}

	if err != nil {
		return fmt.Errorf("failed to check if the Deno binary is already installed at %q: %w", location, err)
	}

	if f != nil {
		defer f.Close()
	}

	stat, err := f.Stat()

	if err != nil {
		return fmt.Errorf("failed to check information about the binary at %q", location)
	}

	perms := stat.Mode().Perm()
	if perms != WANTED_FILE_PERMISSIONS {
		return fmt.Errorf("found a binary at %q but its permissions (unix) are too wide: %s", location, perms.String())
	}

	if stat.Size() > MAX_DENO_BINARY_SIZE {
		return fmt.Errorf("found a binary at %q but it seems to be way too large", location)
	}

	hasher := sha256.New()

	_, err = io.Copy(hasher, f)
	if stat.Size() > MAX_DENO_BINARY_SIZE {
		return fmt.Errorf("failed to read the content of the binary: %w", err)
	}

	installedBinaryHash := hex.EncodeToString(hasher.Sum(nil))

	if installedBinaryHash != archiveInfo.binaryChecksum {
		return fmt.Errorf("the binary at %q has not the expected checksum", location)
	}

	return nil
}
