package project

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project/imageconsts"
)

var (
	_ = core.Image((*Image)(nil))
)

type Image struct {
	filesystem core.FilesystemSnapshot
	info       ImageInfo
}

func (img *Image) ProjectID() core.ProjectID {
	return img.info.ProjectID
}

func (img *Image) FilesystemSnapshot() core.FilesystemSnapshot {
	return img.filesystem
}

func (p *Project) BaseImage() (core.Image, error) {
	snapshot, err := p.stagingFilesystem.TakeFilesystemSnapshot(core.FilesystemSnapshotConfig{
		GetContent: func(ChecksumSHA256 [32]byte) core.AddressableContent {
			return nil
		},
		InclusionFilters: []core.PathPattern{"/..."},
		ExclusionFilters: imageconsts.ABSOLUTE_EXCLUSION_FILTERS,
	})

	if err != nil {
		return nil, err
	}

	return &Image{
		filesystem: snapshot,
		info: ImageInfo{
			ProjectID: p.id,
		},
	}, nil
}
