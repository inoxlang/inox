package binary

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/inoxlang/inox/internal/compressarch"
)

func Upgrade(outW io.Writer) error {
	url, tarGz, err := downloadLatestReleaseArchive(outW)
	if err != nil {
		return err
	}

	return installInoxBinary(inoxBinaryInstallation{
		path:           INOX_BINARY_PATH,
		oldpath:        OLDINOX_BINARY_PATH,
		tempPath:       TEMPINOX_BINARY_PATH,
		downloadURL:    url.String(),
		checkExecution: false,
		gzippedTarball: tarGz,
	})
}

type inoxBinaryInstallation struct {
	path, oldpath, tempPath string
	downloadURL             string
	gzippedTarball          []byte
	checkExecution          bool
}

func installInoxBinary(args inoxBinaryInstallation) error {
	unzipped := bytes.NewBuffer(nil)

	//decompress and untar the gzipped tarball

	err := compressarch.UnGzip(unzipped, bytes.NewReader(args.gzippedTarball))

	if err != nil {
		return fmt.Errorf("failed to ungzip %s", args.downloadURL)
	}

	inoxBinary := bytes.NewBuffer(nil)

	err = compressarch.UntarInMemory(unzipped, func(info fs.FileInfo, reader io.Reader) error {
		if info.Name() == filepath.Base(INOX_BINARY_PATH) {
			if info.Size() > MAX_INOX_BINARY_SIZE {
				return fmt.Errorf("binary is too big (size %d)", info.Size())
			}
			_, err := io.Copy(inoxBinary, reader)
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}

	err = os.WriteFile(args.tempPath, inoxBinary.Bytes(), INOX_BINARY_PERMS)
	if err != nil {
		return err
	}

	defer os.Remove(args.tempPath)

	if args.checkExecution {
		//check that the downloaded binary work
		cmd := exec.Command(args.tempPath, "help")
		cmd.WaitDelay = 100 * time.Millisecond

		if err != nil {
			return fmt.Errorf("error while checking that the downloaded binary work")
		}
	}

	//rename the binary of the previously installed version
	err = os.Rename(args.path, args.oldpath)

	if err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", args.path, args.oldpath, err)
	}

	//install the new version
	err = os.Rename(args.tempPath, args.path)

	if err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", args.tempPath, args.path, err)
	}
	return nil
}
