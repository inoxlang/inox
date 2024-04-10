package fs_ns

import (
	"crypto/sha256"
	"slices"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils/pathutils"
)

func (fs *MemFilesystem) TakeFilesystemSnapshot(config core.FilesystemSnapshotConfig) (core.FilesystemSnapshot, error) {
	storage := fs.s
	storage.lock.RLock()
	defer storage.lock.RUnlock()

	snapshot := &InMemorySnapshot{
		MetadataMap:  make(map[string]*core.EntrySnapshotMetadata, len(storage.files)),
		FileContents: make(map[string]core.AddressableContent, len(storage.files)),
	}

	// determine what files are includable
	includableFiles := map[ /*normalized path*/ string]struct{}{"/": {}}
	for normalizedPath, f := range storage.files {
		if !config.IsFileIncluded(f.absPath) {
			continue
		}
		includableFiles[normalizedPath] = struct{}{}
	}

	// add directory hierarchy of includable files
	for includable := range includableFiles {
		for i := 1; i < len(includable); i++ {
			if includable[i] == '/' {
				includableFiles[includable[:i]] = struct{}{}
			}
		}
	}

	//add includable files & directories to the snapshot
	for normalizedPath, f := range storage.files {
		if _, ok := includableFiles[normalizedPath]; !ok {
			continue
		}

		f.content.lock.RLock()
		defer f.content.lock.RUnlock()

		info := f.FileInfoContentNotLocked()

		childrenMap := storage.children[normalizedPath]
		var childNames []string

		for childBaseName := range childrenMap {
			childPath := normalizedPath + "/" + string(childBaseName)
			if normalizedPath == "/" {
				childPath = childPath[1:]
			}

			if _, ok := includableFiles[childPath]; ok {
				childNames = append(childNames, childBaseName)
			}
		}

		absPath := f.absPath
		if info.Mode_.FileMode().IsDir() {
			absPath = pathutils.AppendTrailingSlashIfNotPresent(absPath)
		}

		metadata := &core.EntrySnapshotMetadata{
			Size:             info.Size_,
			AbsolutePath:     absPath,
			CreationTime:     info.CreationTime_,
			ModificationTime: info.ModTime_,
			Mode:             info.Mode_,
			ChildNames:       childNames,
		}

		snapshot.MetadataMap[normalizedPath] = metadata

		if !info.IsDir() {
			metadata.ChecksumSHA256 = sha256.Sum256(f.content.bytes)

			content := config.GetContent(metadata.ChecksumSHA256)
			if content == nil {
				content = AddressableContentBytes{
					Sha256: metadata.ChecksumSHA256,
					Data:   slices.Clone(f.content.bytes),
				}
			}

			snapshot.FileContents[normalizedPath] = content
		}

	}

	return snapshot, nil
}
