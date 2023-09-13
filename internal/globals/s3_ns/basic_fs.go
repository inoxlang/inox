package s3_ns

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-git/go-billy/v5"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/minio/minio-go/v7"
)

const (
	DIR_FMODE                = core.FileMode(0700 | fs.ModeDir)
	DEFAULT_MAX_STORAGE_SIZE = 100_000_000
	SUPPORTED_OPEN_FLAGS     = os.O_RDONLY | os.O_WRONLY | os.O_RDWR | os.O_APPEND | os.O_EXCL | os.O_CREATE | os.O_TRUNC
)

var (
	ErrOpenFlagNotSupported = errors.New("open flag not supported by S3 filesystem")

	_ = core.IWithSecondaryContext((*S3Filesystem)(nil))
)

type S3Filesystem struct {
	*s3Filesystem
	original     *S3Filesystem
	secondaryCtx *core.Context
}

type s3Filesystem struct {
	bucket         *Bucket
	creatorCtx     *core.Context
	maxStorageSize int64
	storageSize    atomic.Int64

	pendingCreations     map[string]*s3WriteFile
	pendingCreationsLock sync.Mutex
}

func NewS3Filesystem(ctx *core.Context, bucket *Bucket) *S3Filesystem {
	return &S3Filesystem{
		s3Filesystem: &s3Filesystem{
			creatorCtx:       ctx,
			bucket:           bucket,
			maxStorageSize:   DEFAULT_MAX_STORAGE_SIZE,
			pendingCreations: map[string]*s3WriteFile{},
		},
	}
}

func (fls *S3Filesystem) ctx() *core.Context {
	if fls.secondaryCtx != nil {
		return fls.secondaryCtx
	}
	return fls.creatorCtx
}

func (fls *S3Filesystem) client() *S3Client {
	return fls.bucket.client
}

func (fls *S3Filesystem) bucketName() string {
	return fls.bucket.name
}

func (fls *S3Filesystem) WithSecondaryContext(ctx *core.Context) any {
	if ctx == nil {
		panic(errors.New("nil context"))
	}
	if fls.secondaryCtx == ctx {
		return fls
	}
	return &S3Filesystem{
		secondaryCtx: ctx,
		original:     fls.WithoutSecondaryContext().(*S3Filesystem),
		s3Filesystem: fls.s3Filesystem,
	}
}

func (fls *S3Filesystem) WithoutSecondaryContext() any {
	if fls.original == nil {
		return fls
	}
	return fls.original
}

func (fls *S3Filesystem) Create(filename string) (billy.File, error) {
	return fls.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (fls *S3Filesystem) Open(filename string) (billy.File, error) {
	return fls.OpenFile(filename, os.O_RDONLY, 0666)
}

func (fls *S3Filesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	if flag&SUPPORTED_OPEN_FLAGS != flag {
		return nil, ErrOpenFlagNotSupported
	}

	ctx := fls.ctx()
	normalizedFilename := fs_ns.NormalizeAsAbsolute(filename)
	pendingCreationsLocked := false

	if fs_ns.IsExclusive(flag) {
		info, _ := core.DoIO2(fls.ctx(), func() (*minio.Object, error) {
			return fls.client().libClient.GetObject(ctx, fls.bucketName(), toObjectKey(filename), minio.GetObjectOptions{})
		})
		if info != nil {
			return nil, os.ErrExist
		}
		fls.pendingCreationsLock.Lock()
		_, ok := fls.pendingCreations[normalizedFilename]

		if ok {
			fls.pendingCreationsLock.Unlock()
			return nil, os.ErrExist
		}

		pendingCreationsLocked = true
	}

	if fs_ns.IsReadOnly(flag) {
		return newS3ReadFile(ctx, fls.client(), fls.bucketName(), filename)
	} else {
		if !pendingCreationsLocked {
			fls.pendingCreationsLock.Lock()
		}
		defer fls.pendingCreationsLock.Unlock()

		f, ok := fls.pendingCreations[normalizedFilename]
		if ok {
			return f, nil
		}

		file, err := newS3WriteFile(ctx, newS3WriteFileInput{
			client:         fls.client(),
			fs:             fls,
			bucket:         fls.bucketName(),
			filename:       filename,
			flag:           flag,
			tryReadContent: !fs_ns.IsExclusive(flag),
			maxStorage:     fls.maxStorageSize,
			storageSize:    &fls.storageSize,
			perm:           perm.Perm(),
		})

		if err != nil {
			return nil, err
		}
		fls.pendingCreations[normalizedFilename] = file

		if fs_ns.IsTruncate(flag) {
			file.Truncate(0)
		}

		if fs_ns.IsAppend(flag) {
			file.position = int64(file.content.Len())
		}

		return file, nil
	}
}

func (fls *S3Filesystem) Stat(filename string) (os.FileInfo, error) {
	filename = fs_ns.NormalizeAsAbsolute(filename)
	key := toObjectKey(filename)
	ctx := fls.ctx()

	fls.pendingCreationsLock.Lock()
	file, ok := fls.pendingCreations[filename]
	fls.pendingCreationsLock.Unlock()

	if ok {
		return core.FileInfo{
			BaseName_:       filepath.Base(filename),
			AbsPath_:        core.Path(filename),
			Size_:           core.ByteCount(file.content.Len()),
			Mode_:           core.FileMode(file.perm),
			ModTime_:        core.Date(file.content.ModifTime()),
			HasCreationTime: false,
		}, nil
	}

	client := fls.client().libClient

	info, err := client.StatObject(ctx, fls.bucketName(), key, minio.GetObjectOptions{})
	if err != nil {
		if !isNoSuchKeyError(err) {
			return nil, err
		}
		//check if dir by listing files
		channel := client.ListObjects(ctx, fls.bucketName(), minio.ListObjectsOptions{
			Prefix:    core.AppendTrailingSlashIfNotPresent(key),
			Recursive: true,
		})

		select {
		case object := <-channel:
			if object.Key == "" || object.Err != nil {
				break
			}
			//we set the modification time of the "directory" to the
			//modification time of the most recently changed file (recursive).
			mostRecentModifTime := object.LastModified

			for obj := range channel {
				if obj.LastModified.After(mostRecentModifTime) {
					mostRecentModifTime = obj.LastModified
				}
			}

			return core.FileInfo{
				BaseName_:       filepath.Base(filename),
				AbsPath_:        core.DirPathFrom(filename),
				Size_:           core.ByteCount(0),
				Mode_:           core.FileMode(DIR_FMODE),
				ModTime_:        core.Date(mostRecentModifTime),
				HasCreationTime: false,
			}, nil
		case <-time.After(time.Second):
		}
		return nil, os.ErrNotExist
	}

	perm, err := getPerm(info)
	if err != nil {
		return nil, err
	}

	return core.FileInfo{
		BaseName_:       filepath.Base(filename),
		AbsPath_:        core.Path(filename),
		Size_:           core.ByteCount(info.Size),
		Mode_:           core.FileMode(perm),
		ModTime_:        core.Date(info.LastModified),
		HasCreationTime: false,
	}, nil
}

func (fls *S3Filesystem) Rename(oldpath, newpath string) error {
	src := fs_ns.NormalizeAsAbsolute(oldpath)
	dst := fs_ns.NormalizeAsAbsolute(newpath)
	ctx := fls.ctx()
	client := fls.client()

	//copy the file
	_, err := core.DoIO2(ctx, func() (minio.UploadInfo, error) {
		return client.libClient.CopyObject(ctx, minio.CopyDestOptions{
			Bucket: fls.bucketName(),
			Object: toObjectKey(dst),
		}, minio.CopySrcOptions{
			Bucket: fls.bucketName(),
			Object: toObjectKey(src),
		})
	})

	if err != nil {
		return fmt.Errorf("failed to rename file: %s", err)
	}

	//delete the old file
	err = core.DoIO(ctx, func() error {
		return client.libClient.RemoveObject(ctx, fls.bucketName(), toObjectKey(src), minio.RemoveObjectOptions{
			ForceDelete: false,
		})
	})
	if err != nil {
		return fmt.Errorf("failed to remove file: %s", err)
	}

	return nil
}

func (fls *S3Filesystem) Remove(filename string) error {
	filename = fs_ns.NormalizeAsAbsolute(filename)
	key := toObjectKey(filename)

	client := fls.client().libClient
	ctx := fls.ctx()

	_, err := core.DoIO2(ctx, func() (minio.ObjectInfo, error) {
		return client.StatObject(ctx, fls.bucketName(), key, minio.GetObjectOptions{})
	})
	if err != nil && isNoSuchKeyError(err) {
		return os.ErrNotExist
	}

	//no error will be returned if the key does not exist
	err = core.DoIO(ctx, func() error {
		return client.RemoveObject(ctx, fls.bucketName(), key, minio.RemoveObjectOptions{})
	})
	if err != nil {
		return fmt.Errorf("failed to remove file: %s", err)
	}
	return nil
}

func (fls *S3Filesystem) Join(elem ...string) string {
	j := path.Join(elem...)
	c := path.Clean(j)
	return c
}

func (fls *S3Filesystem) RemoveAllObjects() {
	ctx := fls.ctx()

	ctx.DoIO(func() error {
		fls.bucket.RemoveAllObjects(ctx)
		return nil
	})
}

func toObjectKey(filename string) string {
	if filename[0] == '/' {
		return filename[1:]
	}
	return filename
}
