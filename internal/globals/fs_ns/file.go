package fs_ns

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"

	"github.com/inoxlang/inox/internal/core/permbase"
)

const (
	FS_WRITE_LIMIT_NAME          = "fs/write"
	FS_READ_LIMIT_NAME           = "fs/read"
	FS_TOTAL_NEW_FILE_LIMIT_NAME = "fs/total-new-files"
	FS_NEW_FILE_RATE_LIMIT_NAME  = "fs/create-file"

	FS_WRITE_MIN_CHUNK_SIZE = 100_000
	FS_READ_MIN_CHUNK_SIZE  = 1_000_000
	DEFAULT_R_FILE_FMODE    = fs.FileMode(0o400)
	DEFAULT_FILE_FMODE      = fs.FileMode(0o600)
	DEFAULT_DIR_FMODE       = fs.FileMode(0o700)
)

// this file contains the filesystem related types that implement the GoValue interface

type File struct {
	f    afs.File
	path core.Path
}

// OpenExisting is the implementation of fs.open, it calls the internal openExistingFile function.
func OpenExisting(ctx *core.Context, args ...core.Value) (*File, error) {
	var pth core.Path
	var write bool

	for _, arg := range args {

		switch a := arg.(type) {
		case core.Path:
			if pth != "" {
				return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("path")
			}
			pth = a
		case core.Option:
			switch a.Name {
			case "w":
				if boolean, ok := a.Value.(core.Bool); ok {
					write = bool(boolean)
				} else {
					return nil, errors.New("-w should have a boolean value")
				}
			}
		default:
			return nil, fmt.Errorf("invalid argument %v", a)
		}
	}

	return openExistingFile(ctx, pth, write)
}

func openExistingFile(ctx *core.Context, pth core.Path, write bool) (*File, error) {
	fls := ctx.GetFileSystem()
	absPath, err := pth.ToAbs(fls)
	if err != nil {
		return nil, err
	}
	perm := core.FilesystemPermission{Kind_: permbase.Read, Entity: absPath}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	var flag int
	if write {
		flag = os.O_RDWR
	} else {
		flag = os.O_RDONLY
	}
	underlyingFile, err := fls.OpenFile(string(absPath), flag, 0)
	if err != nil {
		return nil, err
	}

	return &File{f: underlyingFile, path: absPath}, nil
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
	return core.NewMutableByteSlice(b, ""), err
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

	perm := core.FilesystemPermission{Kind_: permbase.Read, Entity: f.path}
	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	rate, err := ctx.GetByteRate(FS_READ_LIMIT_NAME)
	if err != nil {
		return nil, err
	}

	fls := ctx.GetFileSystem()
	stat, err := afs.FileStat(f.f, fls)
	if err != nil {
		return nil, err
	}

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

		stat, err := afs.FileStat(f.f, fls)
		if err != nil {
			return nil, err
		}

		if totalN >= int64(stat.Size()) || totalN >= count || err == io.EOF {
			break
		}
	}

	return b, nil
}

func (f *File) write(ctx *core.Context, data core.Readable) error {
	perm := core.FilesystemPermission{Kind_: permbase.WriteStream, Entity: f.path}
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
		d = slice.UnderlyingBytes()
	}

	ctx.Take(FS_WRITE_LIMIT_NAME, int64(len(d)))
	_, err := f.f.Write(d)
	return err
}

func (f *File) close(ctx *core.Context) {
	f.f.Close()
}

func (f *File) info(ctx *core.Context) (core.FileInfo, error) {
	stat, err := afs.FileStat(f.f, ctx.GetFileSystem())
	if err != nil {
		return core.FileInfo{}, err
	}

	return makeFileInfo(stat, string(f.path), ctx.GetFileSystem()), nil
}
