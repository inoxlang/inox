package fs_ns

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

func (fls *MetaFilesystem) TakeFilesystemSnapshot(config core.FilesystemSnapshotConfig) (core.FilesystemSnapshot, error) {
	if !fls.snapshoting.CompareAndSwap(false, true) {
		return nil, core.ErrAlreadyBeingSnapshoted
	}
	defer fls.snapshoting.Store(false)

	size, err := fls.computeUsedSpace(false)

	if err != nil {
		return nil, err
	}

	if size > METAFS_MAX_SNAPSHOTABLE_SIZE {
		max, err := commonfmt.FmtByteCount(int64(METAFS_MAX_SNAPSHOTABLE_SIZE), -1)
		if err != nil {
			panic(err)
		}
		return nil, fmt.Errorf("snapshoting of meta filesystems only support filesystems up to %s", max)
	}

	switch fls.underlying.(type) {
	case *OsFilesystem, *MemFilesystem:
	default:
		return nil,
			errors.New("for now snapshoting is only supported when the underlying filesystem is the OS filesystem or a memory filesystem")
	}

	snapshot := &InMemorySnapshot{
		MetadataMap:  make(map[string]*core.EntrySnapshotMetadata),
		FileContents: make(map[string]core.AddressableContent),
	}

	fls.lock.Lock()
	defer fls.lock.Unlock()
	fls.untrackSomeClosedFiles(100)

	//files being written to.
	var writableFiles []*metaFsFile
	writableFilePaths := map[string]struct{}{}

top:
	for _, files := range fls.openFiles {
		for sameFile := range files {
			if !config.IsFileIncluded(sameFile.path) {
				continue top
			}

			if !IsReadOnly(sameFile.flag) {
				writableFiles = append(writableFiles, sameFile)
				writableFilePaths[sameFile.normalizedPath] = struct{}{}

				sameFile.snapshoting.Store(true)
				break
			}
		}
	}

	defer func() {
		for _, file := range writableFiles {
			file.snapshoting.Store(false)
		}
	}()

	//add writable files to the snapshot
	for _, file := range writableFiles {
		normalizedPath := NormalizeAsAbsolute(file.metadata.path.UnderlyingString())
		concreteFilePath := file.metadata.concreteFile.UnderlyingString()

		file.underlying.Sync()

		content, err := util.ReadFile(fls.underlying, concreteFilePath)
		if err != nil {
			return nil, err
		}
		checkSum := sha256.Sum256(content)

		//add the file's content and metadata to the snapshot
		metadata := &core.EntrySnapshotMetadata{
			Size:             core.ByteCount(len(content)),
			AbsolutePath:     file.metadata.path,
			CreationTime:     file.metadata.creationTime,
			ModificationTime: file.metadata.modificationTime,
			Mode:             core.FileMode(file.metadata.mode),
			ChecksumSHA256:   checkSum,
		}

		snapshot.MetadataMap[normalizedPath] = metadata
		snapshot.FileContents[normalizedPath] = AddressableContentBytes{
			Sha256: checkSum,
			Data:   content,
		}
	}

	includableFiles := map[ /*normalized path*/ string]struct{}{"/": {}}
	maps.Copy(includableFiles, writableFilePaths)

	// determine what remaining files are includable
	fls.Walk(func(normalizedPath string, path core.Path, metadata *metaFsFileMetadata) error {
		if !config.IsFileIncluded(path) {
			return nil
		}

		includableFiles[normalizedPath] = struct{}{}
		return nil
	})

	// add directory hierarchy of includable files
	for includable := range includableFiles {
		for i := 1; i < len(includable); i++ {
			if includable[i] == '/' {
				includableFiles[includable[:i]] = struct{}{}
			}
		}
	}

	//add other files to the snapshot
	err = fls.Walk(func(normalizedPath string, path core.Path, metadata *metaFsFileMetadata) error {
		if _, ok := writableFilePaths[normalizedPath]; ok {
			//already in the snapshot
			return nil
		}
		if _, ok := includableFiles[normalizedPath]; !ok {
			return nil
		}

		var content []byte
		var checksum [32]byte

		if !metadata.mode.IsDir() {
			concreteFilePath := metadata.concreteFile.UnderlyingString()
			content, err = util.ReadFile(fls.underlying, concreteFilePath)
			if err != nil {
				return err
			}
			checksum = sha256.Sum256(content)
		}

		//add the file's content and metadata to the snapshot
		entryMetadata := &core.EntrySnapshotMetadata{
			Size:             core.ByteCount(len(content)),
			AbsolutePath:     path,
			CreationTime:     metadata.creationTime,
			ModificationTime: metadata.modificationTime,
			Mode:             core.FileMode(metadata.mode),
			ChecksumSHA256:   checksum,
			ChildNames: utils.FilterMapSlice(metadata.children, func(childName core.String) (string, bool) {
				childPath := normalizedPath + "/" + string(childName)
				if normalizedPath == "/" {
					childPath = childPath[1:]
				}

				if _, ok := includableFiles[childPath]; !ok {
					return "", false
				}
				return string(childName), true
			}),
		}

		snapshot.MetadataMap[normalizedPath] = entryMetadata

		if !entryMetadata.IsDir() {
			snapshot.FileContents[normalizedPath] = AddressableContentBytes{
				Sha256: checksum,
				Data:   content,
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return snapshot, nil
}
