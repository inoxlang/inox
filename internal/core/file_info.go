package core

import (
	"io/fs"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

var (
	_ fs.FileInfo = FileInfo{}
)

func init() {
	RegisterSymbolicGoFunction(FileModeFrom, func(ctx *symbolic.Context, firstArg symbolic.SymbolicValue) *symbolic.FileMode {
		_, ok := firstArg.(*symbolic.Int)
		if !ok {
			ctx.AddSymbolicGoFunctionError("argument should be an integer")
		}

		return symbolic.ANY_FILEMODE
	})
}

type ExtendedFileInfo interface {
	fs.FileInfo

	//the boolean result should be false if the creation time is not available.
	CreationTime() (time.Time, bool)
}

type FileInfo struct {
	BaseName_     string
	AbsPath_      Path
	Size_         ByteCount
	Mode_         FileMode
	ModTime_      Date
	CreationTime_ Date

	HasCreationTime bool
}

func (fi FileInfo) Name() string {
	return fi.BaseName_
}

func (fi FileInfo) Size() int64 {
	return int64(fi.Size_)
}

func (fi FileInfo) Mode() fs.FileMode {
	return fs.FileMode(fi.Mode_)
}

func (fi FileInfo) ModTime() time.Time {
	return time.Time(fi.ModTime_)
}

func (fi FileInfo) CreationTime() (time.Time, bool) {
	if !fi.HasCreationTime {
		return time.Time{}, false
	}
	return time.Time(fi.CreationTime_), true
}

func (fi FileInfo) IsDir() bool {
	return fi.Mode().IsDir()
}

func (fi FileInfo) Sys() any {
	return nil
}

func (i FileInfo) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (i FileInfo) Prop(ctx *Context, name string) Value {
	switch name {
	case "name":
		return Str(i.BaseName_)
	case "abs-path":
		return i.AbsPath_
	case "size":
		return i.Size_
	case "mode":
		return i.Mode_
	case "mod-time":
		return i.ModTime_
	case "is-dir":
		return Bool(i.IsDir())
	}
	method, ok := i.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, i))
	}
	return method
}

func (FileInfo) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (FileInfo) PropertyNames(ctx *Context) []string {
	return []string{"name", "abs-path", "size", "mode", "mod-time", "is-dir"}
}

func FileModeFrom(ctx *Context, firstArg Value) FileMode {
	integer, ok := firstArg.(Int)
	if !ok {
		panic(commonfmt.FmtErrInvalidArgumentAtPos(0, "should be an integer"))
	}

	return FileMode(integer)
}
