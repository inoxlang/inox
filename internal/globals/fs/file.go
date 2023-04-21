package internal

import (
	"io"
	"io/fs"
	"math"
	"os"

	core "github.com/inoxlang/inox/internal/core"
)

const (
	FS_WRITE_LIMIT_NAME          = "fs/write"
	FS_READ_LIMIT_NAME           = "fs/read"
	FS_TOTAL_NEW_FILE_LIMIT_NAME = "fs/total-new-file"
	FS_NEW_FILE_RATE_LIMIT_NAME  = "fs/new-file"

	FS_WRITE_MIN_CHUNK_SIZE = 100_000
	FS_READ_MIN_CHUNK_SIZE  = 1_000_000
	DEFAULT_FILE_FMODE      = fs.FileMode(0o400)
	DEFAULT_RW_FILE_FMODE   = fs.FileMode(0o600)
	DEFAULT_DIR_FMODE       = fs.FileMode(0o700)
)

// this file contains the filesystem related types that implement the GoValue interface

type File struct {
	f    *os.File
	path core.Path
}

func openExistingFile(ctx *core.Context, pth core.Path, write bool) (*File, error) {
	absPath := pth.ToAbs()
	perm := core.FilesystemPermission{Kind_: core.ReadPerm, Entity: absPath}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	var flag int
	if write {
		flag = os.O_RDWR
	} else {
		flag = os.O_RDONLY
	}
	underlyingFile, err := os.OpenFile(string(absPath), flag, 0)
	if err != nil {
		return nil, err
	}

	return &File{underlyingFile, absPath}, nil
}

func (f *File) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "read":
		return core.WrapGoMethod(f.read), true
	case "write":
		return core.WrapGoMethod(f.write), true
	case "close":
		return core.WrapGoMethod(f.close), true
	case "info":
		return core.WrapGoMethod(f.info), true
	}
	return nil, false
}

func (f *File) Prop(ctx *core.Context, name string) core.Value {
	method, ok := f.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, f))
	}
	return method
}

func (*File) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*File) PropertyNames(ctx *core.Context) []string {
	return []string{"read", "write", "close", "info"}
}

func (f *File) read(ctx *core.Context) (*core.ByteSlice, error) {
	b, err := f.doRead(ctx, false, FS_READ_MIN_CHUNK_SIZE)
	return &core.ByteSlice{Bytes: b, IsDataMutable: true}, err
}

// doRead reads up to count bytes, if count is -1 all the file is read.
func (f *File) doRead(ctx *core.Context, closeFile bool, count int64) ([]byte, error) {

	if count == -1 {
		count = math.MaxInt64
	}

	alreadyClosed := false

	defer func() {
		if closeFile && !alreadyClosed {
			f.close(ctx)
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	perm := core.FilesystemPermission{Kind_: core.ReadPerm, Entity: f.path}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	rate, err := ctx.GetByteRate(FS_READ_LIMIT_NAME)
	if err != nil {
		return nil, err
	}

	stat, _ := f.f.Stat()

	chunkSize := computeChunkSize(rate, int(stat.Size()))
	chunk := make([]byte, chunkSize)

	var b []byte
	var totalN int64 = 0
	n := len(chunk)

	for {
		select {
		case <-ctx.Done():
			if closeFile {
				f.close(ctx)
				alreadyClosed = true
			}
			return nil, ctx.Err()
		default:
		}

		if err := ctx.Take(FS_READ_LIMIT_NAME, int64(n)); err != nil {
			return nil, err
		}

		n, err = f.f.Read(chunk)
		if err != nil {
			return nil, err
		}

		b = append(b, chunk[0:n]...)
		totalN += int64(n)

		stat, _ := f.f.Stat()

		if totalN >= int64(stat.Size()) || totalN >= count || err == io.EOF {
			break
		}
	}

	return b, nil
}

func (f *File) write(ctx *core.Context, data core.Readable) error {
	perm := core.FilesystemPermission{Kind_: core.WriteStreamPerm, Entity: f.path}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	reader := data.Reader()

	var d []byte
	if reader.AlreadyHasAllData() {
		d = reader.GetBytesDataToNotModify()
	} else {
		slice, err := reader.ReadAll()
		if err != nil {
			return err
		}
		d = slice.Bytes
	}

	ctx.Take(FS_WRITE_LIMIT_NAME, int64(len(d)))
	_, err := f.f.Write(d)
	return err
}

func (f *File) close(ctx *core.Context) {
	f.f.Close()
}

func (f *File) info(ctx *core.Context) (core.FileInfo, error) {

	stat, err := f.f.Stat()
	if err != nil {
		return core.FileInfo{}, err
	}

	return makeFileInfo(stat, string(f.path)), nil
}
