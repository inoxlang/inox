package fs_ns

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/inoxlang/inox/internal/buntdb"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
)

func (fls *MetaFilesystem) getFileMetadata(pth core.Path, usedTx *buntdb.Tx) (*metaFsFileMetadata, bool, error) {
	if !pth.IsAbsolute() {
		return nil, false, errors.New("file's path should be absolute")
	}

	if fls.closed.Load() {
		return nil, false, ErrClosedFilesystem
	}

	var lastModificationTime core.DateTime
	var hasLastModifTime bool
	func() {
		fls.lastModificationTimesLock.RLock()
		defer fls.lastModificationTimesLock.RUnlock()
		lastModificationTime, hasLastModifTime = fls.lastModificationTimes[NormalizeAsAbsolute(pth.UnderlyingString())]
	}()

	key := getKvKeyFromPath(pth)

	var (
		serializedMetadata string
		err                error
	)

	metadata := metaFsFileMetadata{path: pth}

	if usedTx == nil {
		//create a temporary transaction
		usedTx, err = fls.metadata.Begin(false)
		if err != nil {
			return nil, false, err
		}
		defer func() {
			// Read-only transactions can only be rolled back, not committed.
			usedTx.Rollback()
		}()
	}

	serializedMetadata, err = usedTx.Get(key.UnderlyingString())

	if err != nil {
		if errors.Is(err, buntdb.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, fmtFailedToGetFileMetadataError(pth, err)
	}

	err = metadata.initFromJSON(serializedMetadata, hasLastModifTime, lastModificationTime)
	if err != nil {
		return nil, false, err
	}

	return &metadata, true, nil
}

func (fls *MetaFilesystem) setFileMetadata(metadata *metaFsFileMetadata, tx *buntdb.Tx) error {
	if !metadata.path.IsAbsolute() {
		return errors.New("file's path should be absolute")
	}

	json := metadata.marshalJSON()
	key := getKvKeyFromPath(metadata.path)

	var noIssue bool
	if tx == nil {
		//create a temporary transaction
		var err error
		tx, err = fls.metadata.Begin(true)
		if err != nil {
			return err
		}
		defer func() {
			if noIssue {
				tx.Commit()
			} else {
				tx.Rollback()
			}
		}()
	}
	_, _, err := tx.Set(string(key), json, nil)
	noIssue = err == nil
	return err
}

func (fls *MetaFilesystem) deleteFileMetadata(pth core.Path, tx *buntdb.Tx) error {
	key := getKvKeyFromPath(pth)

	var noIssue bool
	if tx == nil {
		//create a temporary transaction
		var err error
		tx, err = fls.metadata.Begin(true)
		if err != nil {
			return err
		}
		defer func() {
			if noIssue {
				tx.Commit()
			} else {
				tx.Rollback()
			}
		}()
	}

	_, err := tx.Delete(string(key))
	noIssue = err == nil
	return nil
}

// a metaFsFileMetadata is the metadata about a file or directory.
type metaFsFileMetadata struct {
	path             core.Path
	concreteFile     *core.Path //nil if dir
	mode             fs.FileMode
	creationTime     core.DateTime
	modificationTime core.DateTime

	//the targets of symlinks are directly stored in the metadata,
	//there is no underlying file.
	symlinkTarget *core.Path

	//name of children if directory
	children []core.String
}

func (m *metaFsFileMetadata) ChildrenPaths() []core.Path {
	children := make([]core.Path, len(m.children))
	for i, childName := range m.children {
		children[i] = core.Path(filepath.Join(m.path.UnderlyingString(), string(childName)))
	}
	return children
}

func (m *metaFsFileMetadata) initFromJSON(serialized string, updateLastModiftime bool, newModifTime core.DateTime) error {

	it := jsoniter.NewIterator(jsoniter.ConfigDefault).ResetBytes(utils.StringAsBytes(serialized))

	hasMode := false
	hasCreationTime := false
	hasModifTime := false
	hasUnderlyingFile := false

	it.ReadObjectMinimizeAllocationsCB(func(it *jsoniter.Iterator, key []byte, allocated bool) bool {
		keyString := utils.BytesAsString(key)

		switch keyString {
		case METAFS_FILE_MODE_PROPNAME:
			hasMode = true

			integer := it.ReadUint32()
			m.mode = fs.FileMode(integer)
		case METAFS_CREATION_TIME_PROPNAME:
			hasCreationTime = true

			var creationTime time.Time
			data, _ := it.ReadStringAsBytes()
			utils.PanicIfErr(creationTime.UnmarshalText(data))

			m.creationTime = core.DateTime(creationTime)
		case METAFS_MODIF_TIME_PROPNAME:
			hasModifTime = true

			var modifTime time.Time
			data, _ := it.ReadStringAsBytes()
			utils.PanicIfErr(modifTime.UnmarshalText(data))
			m.modificationTime = core.DateTime(modifTime)
		case METAFS_UNDERLYING_FILE_PROPNAME:
			hasUnderlyingFile = true

			path := core.Path(it.ReadString())
			m.concreteFile = &path
		case METAFS_SYMLINK_TARGET_PROPNAME:
			path := core.Path(it.ReadString())
			m.symlinkTarget = &path
		case METAFS_CHILDREN_PROPNAME:
			it.ReadArrayCB(func(i *jsoniter.Iterator) bool {
				m.children = append(m.children, core.String(it.ReadString()))
				return true
			})
		default:
			it.ReportError("read metadata", "unexpected property: "+keyString)
		}

		return it.Error == nil
	})

	if it.Error != nil {
		return fmt.Errorf("invalid metadata for file %s, %w", m.path, it.Error)
	}

	fmtMissingPropErrr := func(propName string) error {
		return fmt.Errorf("invalid metadata for file %s, missing property .%s", m.path, propName)
	}

	if !hasMode {
		return fmtMissingPropErrr(METAFS_FILE_MODE_PROPNAME)
	}

	if !hasCreationTime {
		return fmtMissingPropErrr(METAFS_CREATION_TIME_PROPNAME)
	}

	if !hasModifTime {
		return fmtMissingPropErrr(METAFS_MODIF_TIME_PROPNAME)
	}

	if !m.mode.IsDir() && !hasUnderlyingFile {
		return errors.New("missing path of nderlying file")
	}

	if updateLastModiftime {
		m.modificationTime = core.DateTime(newModifTime)
	}

	return nil
}

func (m *metaFsFileMetadata) marshalJSON() string {
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)

	stream.WriteObjectStart()

	stream.WriteObjectField(METAFS_FILE_MODE_PROPNAME)
	stream.WriteUint32(uint32(m.mode))
	stream.WriteMore()

	stream.WriteObjectField(METAFS_CREATION_TIME_PROPNAME)
	stream.Write(utils.Must(time.Time(m.creationTime).MarshalJSON()))
	stream.WriteMore()

	stream.WriteObjectField(METAFS_MODIF_TIME_PROPNAME)
	stream.Write(utils.Must(time.Time(m.modificationTime).MarshalJSON()))
	stream.WriteMore()

	if m.mode.IsDir() {
		stream.WriteObjectField(METAFS_CHILDREN_PROPNAME)
		stream.WriteArrayStart()

		for i, child := range m.children {
			if i != 0 {
				stream.WriteMore()
			}
			stream.WriteString(string(child))
		}
		stream.WriteArrayEnd()
	} else {
		stream.WriteObjectField(METAFS_UNDERLYING_FILE_PROPNAME)
		stream.WriteString(m.concreteFile.UnderlyingString())
	}

	stream.WriteObjectEnd()
	return string(stream.Buffer())
}

func fmtFailedToGetFileMetadataError(pth core.Path, err error) error {
	return fmt.Errorf("failed to get metadata for file %s: %w", pth, err)
}
