package project

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
)

const (
	MAX_IMG_INFO_FILE_SIZE   = 1_000_000
	MAX_UNCOMPRESSED_FS_SIZE = 100_000_000
)

var (
	ErrFilesystemSnapshotTooLarge = errors.New("filesystem snapshot is too large")
)

type ImageInfo struct {
	ProjectID core.ProjectID `json:"projectID"`
}

func (img ImageInfo) Validate() error {

	if err := img.ProjectID.Validate(); err != nil {
		return fmt.Errorf("invalid project ID in image info: %w", err)
	}

	return nil
}

// Zip writes to $w a the zip archive that can be used to re-create the image.
func (img *Image) Zip(ctx *core.Context, w io.Writer) error {
	archive := zip.NewWriter(w)
	snapshot := img.FilesystemSnapshot()

	snapshotSize := core.ByteCount(0)

	snapshot.ForEachEntry(func(m core.EntrySnapshotMetadata) error {
		if !m.IsDir() {
			snapshotSize += m.Size
		}
		return nil
	})

	if snapshotSize > MAX_UNCOMPRESSED_FS_SIZE {
		return ErrFilesystemSnapshotTooLarge
	}

	imgJson, err := json.Marshal(ImageInfo{
		ProjectID: img.info.ProjectID,
	})

	if err != nil {
		return fmt.Errorf("failed to marshal image information to JSON: %w", err)
	}

	err = snapshot.ForEachEntry(func(m core.EntrySnapshotMetadata) error {
		if ctx.IsDoneSlowCheck() {
			return ctx.Err()
		}

		relativePath := inoxconsts.FS_DIR_SLASH_IN_IMG_ZIP + string(m.AbsolutePath[1:])
		if m.IsDir() {
			relativePath = core.AppendTrailingSlashIfNotPresent(relativePath)
		}

		contentWriter, err := archive.Create(relativePath)
		if err != nil {
			return fmt.Errorf("failed to create %s in the zip archive: %w", relativePath, err)
		}

		if m.IsDir() {
			return nil
		}

		content, err := snapshot.Content(m.AbsolutePath.UnderlyingString())
		if err != nil {
			return fmt.Errorf("failed to get content of %s: %w", m.AbsolutePath, err)
		}

		//TODO: check context regularly during the copy

		_, err = io.Copy(contentWriter, content.Reader())
		if err != nil {
			return fmt.Errorf("failed to write the content of %s to the zip: %w", relativePath, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	imgJsonWriter, err := archive.Create(inoxconsts.IMAGE_INFO_FILE_IN_IMG_ZIP)
	if err != nil {
		return fmt.Errorf("failed to create %s in the zip archive: %w", inoxconsts.IMAGE_INFO_FILE_IN_IMG_ZIP, err)
	}

	_, err = imgJsonWriter.Write(imgJson)
	if err != nil {
		return fmt.Errorf("failed to write content of %s in the zip archive: %w", inoxconsts.IMAGE_INFO_FILE_IN_IMG_ZIP, err)
	}

	return archive.Close()
}

func NewImageFromZip(ctx *core.Context, r io.ReaderAt, archiveSize int64) (*Image, error) {
	archiveReader, err := zip.NewReader(r, archiveSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read image archive: %w", err)
	}

	//First we read the image information file.

	const IMG_INFO_PATH = inoxconsts.IMAGE_INFO_FILE_IN_IMG_ZIP

	imgInfoFile, err := archiveReader.Open(IMG_INFO_PATH)
	if err != nil {
		return nil, fmt.Errorf("failed to get or read %s from the zip archive: %w", IMG_INFO_PATH, err)
	}

	imgInfoReader := io.LimitReader(imgInfoFile, MAX_IMG_INFO_FILE_SIZE)

	imgInfoJson, err := io.ReadAll(imgInfoReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read the content of %s from the zip archive: %w", IMG_INFO_PATH, err)
	}

	var imgInfo ImageInfo

	err = json.Unmarshal(imgInfoJson, &imgInfo)

	if err != nil {
		return nil, fmt.Errorf("failed to ubmarshal image information from JSON: %w", err)
	}

	//Create a filesystem snapshot from the files in fs/.

	const FS_DIR = inoxconsts.FS_DIR_IN_IMG_ZIP
	const FS_DIR_SLASH = inoxconsts.FS_DIR_SLASH_IN_IMG_ZIP

	memFS := fs_ns.NewMemFilesystem(MAX_UNCOMPRESSED_FS_SIZE)

	for _, file := range archiveReader.File {

		if ctx.IsDoneSlowCheck() {
			return nil, ctx.Err()
		}

		if !strings.HasPrefix(file.Name, FS_DIR_SLASH) {
			continue
		}

		absolutePathInVirtualFs := strings.TrimPrefix(file.Name, FS_DIR)
		if file.FileInfo().IsDir() {
			err := memFS.MkdirAll(absolutePathInVirtualFs, fs_ns.DEFAULT_DIR_FMODE)
			if err != nil {
				return nil, fmt.Errorf("failed to create dir %s in temporary mem fs: %w", absolutePathInVirtualFs, err)
			}
			continue
		}

		memFile, err := memFS.Create(absolutePathInVirtualFs)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s in temporary mem fs: %w", absolutePathInVirtualFs, err)
		}

		err = func() error {
			reader, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open %s in zip archive: %w", absolutePathInVirtualFs, err)
			}
			defer reader.Close()

			limitReader := io.LimitReader(reader, MAX_UNCOMPRESSED_FS_SIZE)
			_, err = io.Copy(memFile, limitReader)
			if err != nil {
				return fmt.Errorf("failed to read %s in zip archive: %w", absolutePathInVirtualFs, err)
			}

			return nil
		}()

		if err != nil {
			return nil, err
		}
	}

	snapshot, err := memFS.TakeFilesystemSnapshot(core.FilesystemSnapshotConfig{
		GetContent: func(ChecksumSHA256 [32]byte) core.AddressableContent {
			return nil
		},
		InclusionFilters: []core.PathPattern{"/..."},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create filesystem snapshot for image: %w", err)
	}

	return &Image{
		filesystem: snapshot,
		info:       imgInfo,
	}, nil
}
