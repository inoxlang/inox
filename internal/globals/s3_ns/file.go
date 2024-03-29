package s3_ns

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/minio/minio-go/v7"
)

const (
	PERM_METADATA_KEY = "Perm" //a user metadata key seems to require
)

var (
	ErrLockNotSupported        = errors.New("lock not supported by s3 filesystem")
	ErrCannotWriteToReadOnly   = errors.New("cannot write to read-only file")
	ErrCannotTruncateReadOnly  = errors.New("cannot truncate a read-only file")
	ErrCannotReadFromWriteOnly = errors.New("cannot read from write-only file")

	_ afs.SyncCapable = (*s3WriteFile)(nil)
)

// s3ReadFile represents a file opened in read mode.
// upon creation, the file is loaded from S3.
type s3ReadFile struct {
	ctx      *core.Context
	client   *S3Client
	bucket   string
	filename string
	perm     fs.FileMode
	closed   atomic.Bool
	reader   *bytes.Reader // buffer for file content
}

func newS3ReadFile(ctx *core.Context, client *S3Client, bucket, filename string) (*s3ReadFile, error) {
	key := toObjectKey(filename)
	res, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to perform GetObject operation: %w", err)
	}

	//read the file contents
	buf, err := io.ReadAll(res)
	if err != nil {
		return nil, fmt.Errorf("unable to read file body: %w", err)
	}
	reader := bytes.NewReader(buf)

	info, err := core.DoIO2(ctx, res.Stat)
	if err != nil {
		return nil, fmt.Errorf("unable to get info of file: %w", err)
	}

	perm, err := getPerm(info)
	if err != nil {
		return nil, err
	}

	return &s3ReadFile{
		ctx:      ctx,
		client:   client,
		bucket:   bucket,
		filename: filename,
		perm:     fs.FileMode(perm),
		reader:   reader,
	}, nil
}

func (f *s3ReadFile) Name() string {
	return f.filename
}

func (f *s3ReadFile) Write(p []byte) (n int, err error) {
	return 0, ErrCannotWriteToReadOnly
}

func (f *s3ReadFile) Read(p []byte) (n int, err error) {
	if f.reader == nil {
		return 0, os.ErrClosed
	}
	return f.reader.Read(p)
}

func (f *s3ReadFile) ReadAt(p []byte, off int64) (n int, err error) {
	if f.reader == nil {
		return 0, os.ErrClosed
	}
	return f.reader.ReadAt(p, off)
}

func (f *s3ReadFile) Seek(offset int64, whence int) (int64, error) {
	if f.reader == nil {
		return 0, os.ErrClosed
	}
	return f.reader.Seek(offset, whence)
}

func (f *s3ReadFile) Close() error {
	if !f.closed.CompareAndSwap(false, true) {
		return os.ErrClosed
	}

	f.reader = nil

	return nil
}

func (f *s3ReadFile) Lock() error {
	return ErrLockNotSupported
}

func (f *s3ReadFile) Unlock() error {
	return ErrLockNotSupported
}

func (f *s3ReadFile) Truncate(size int64) error {
	return ErrCannotTruncateReadOnly
}

// s3WriteFile stores a file opened in write mode, a buffer is created to store the file contents.
// Upon close, the file is uploaded to S3.
type s3WriteFile struct {
	ctx      *core.Context
	fs       *S3Filesystem
	client   *S3Client
	bucket   string
	filename string
	closed   atomic.Bool

	flag         int
	perm         fs.FileMode
	content      *fs_ns.InMemFileContent
	position     int64
	positionLock sync.RWMutex
}

type newS3WriteFileInput struct {
	client         *S3Client
	fs             *S3Filesystem
	filename       string
	flag           int
	maxStorage     int64
	storageSize    *atomic.Int64
	tryReadContent bool
	perm           fs.FileMode
}

func newS3WriteFile(ctx *core.Context, input newS3WriteFileInput) (*s3WriteFile, error) {
	//S3 objects have no creation time
	ignoredTime := time.Now()

	key := toObjectKey(input.filename)

	file := &s3WriteFile{
		ctx:      ctx,
		fs:       input.fs,
		client:   input.client,
		bucket:   input.fs.bucket.name,
		filename: input.filename,
		perm:     input.perm, //will be changed if the object exists
		flag:     input.flag,
	}

	var content []byte
	if input.tryReadContent {
		//note: GetObject requests are lazy so after this call we don't know if the object exists yet
		res, err := input.client.GetObject(ctx, file.bucket, key, minio.GetObjectOptions{})

		if err != nil {
			return nil, fmt.Errorf("unable to perform GetObject operation: %w", err)
		}

		info, err := core.DoIO2(ctx, res.Stat)
		if !isNoSuchKeyError(err) && err != nil {
			return nil, fmt.Errorf("unable to get S3 object: %w", err)
		}

		if err == nil {
			//read the file contents
			buf, err := io.ReadAll(res)
			if err != nil {
				return nil, fmt.Errorf("unable to read file body: %w", err)
			}

			content = buf

			perm, err := getPerm(info)
			if err != nil {
				return nil, err
			}
			file.perm = perm
		}
	}

	file.content = fs_ns.NewInMemFileContent(input.filename, content, ignoredTime, input.maxStorage, input.storageSize)

	file.ctx.OnDone(func(timeoutCtx context.Context, teardownStatus core.GracefulTeardownStatus) error {
		file.closed.Store(true)

		go func() {
			defer utils.Recover()
			if file.content.ShouldBePersisted() {
				file.sync(true)
			}
		}()

		return nil
	})

	return file, nil
}

func (f *s3WriteFile) Name() string {
	return f.filename
}

func (f *s3WriteFile) Write(p []byte) (n int, err error) {
	if f.closed.Load() {
		return 0, os.ErrClosed
	}

	f.positionLock.Lock()
	defer f.positionLock.Unlock()
	position := f.position

	return f.content.WriteAt(p, position)
}

func (f *s3WriteFile) Read(b []byte) (int, error) {
	if f.closed.Load() {
		return 0, os.ErrClosed
	}

	if !fs_ns.IsReadAndWrite(f.flag) {
		return 0, ErrCannotReadFromWriteOnly
	}

	f.positionLock.Lock()
	defer f.positionLock.Unlock()
	position := f.position

	n, err := f.ReadAt(b, position)
	f.position += int64(n)

	if err == io.EOF && n != 0 {
		err = nil
	}

	return n, err
}

func (f *s3WriteFile) ReadAt(p []byte, off int64) (int, error) {
	if f.closed.Load() {
		return 0, os.ErrClosed
	}

	if !fs_ns.IsReadAndWrite(f.flag) {
		return 0, errors.New("read not supported")
	}

	n, err := f.content.ReadAt(p, off)

	return n, err
}

// TODO: make thread safe
func (f *s3WriteFile) Seek(offset int64, whence int) (int64, error) {
	if f.closed.Load() {
		return 0, os.ErrClosed
	}

	if !fs_ns.IsReadAndWrite(f.flag) {
		return 0, errors.New("read not supported")
	}

	f.positionLock.Lock()
	defer f.positionLock.Unlock()

	switch whence {
	case io.SeekCurrent:
		f.position += offset
	case io.SeekStart:
		f.position = offset
	case io.SeekEnd:
		f.position = int64(f.content.Len()) + offset
	}

	return f.position, nil
}

func (f *s3WriteFile) Truncate(size int64) error {
	if f.closed.Load() {
		return os.ErrClosed
	}
	return f.content.Truncate(size)
}

func (f *s3WriteFile) Sync() error {
	if f.closed.Load() {
		return os.ErrClosed
	}
	return f.sync(false)
}

func (f *s3WriteFile) sync(contextDone bool) error {
	err := f.content.Persist(func(p []byte) error {
		key := toObjectKey(f.filename)
		opts := minio.PutObjectOptions{
			UserMetadata: map[string]string{
				PERM_METADATA_KEY: strconv.FormatUint(uint64(f.perm), 8),
			},
		}
		reader := bytes.NewReader(p)

		var err error
		if contextDone {
			_, err = f.client.PutObjectNoCtx(f.bucket, key, reader, int64(len(p)), opts)
		} else {
			_, err = f.client.PutObject(f.ctx, f.bucket, key, reader, int64(len(p)), opts)
		}

		if err != nil {
			return fmt.Errorf("unable to sync file %s: %w", f.filename, err)
		}
		return nil
	})

	if err != nil {
		return err
	}

	f.fs.pendingCreationsLock.Lock()
	defer f.fs.pendingCreationsLock.Unlock()
	delete(f.fs.pendingCreations, fs_ns.NormalizeAsAbsolute(f.filename))
	return nil
}

func (f *s3WriteFile) Close() error {
	if !f.closed.CompareAndSwap(false, true) {
		return os.ErrClosed
	}

	return f.sync(false)
}

func (f *s3WriteFile) Lock() error {
	return ErrLockNotSupported
}

func (f *s3WriteFile) Unlock() error {
	return ErrLockNotSupported
}

func isNoSuchKeyError(err error) bool {
	errResp, ok := err.(minio.ErrorResponse)
	return ok && errResp.Code == "NoSuchKey"
}

func getPerm(info minio.ObjectInfo) (fs.FileMode, error) {
	permVal := info.UserMetadata[PERM_METADATA_KEY]
	if permVal == "" {
		return 0, fmt.Errorf("missing '%s' in metadata of S3 object", PERM_METADATA_KEY)
	}
	perm, err := strconv.ParseUint(permVal, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid '%s' in metadata of S3 object", PERM_METADATA_KEY)
	}

	return fs.FileMode(perm).Perm(), nil
}
