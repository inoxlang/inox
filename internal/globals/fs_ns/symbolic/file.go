package fs_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

type File struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *File) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*File)
	return ok
}

func (f *File) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "read":
		return symbolic.WrapGoMethod(f.read), true
	case "write":
		return symbolic.WrapGoMethod(f.write), true
	case "close":
		return symbolic.WrapGoMethod(f.close), true
	case "info":
		return symbolic.WrapGoMethod(f.info), true
	}
	return nil, false
}

func (f *File) Prop(name string) symbolic.SymbolicValue {
	method, ok := f.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, f))
	}
	return method
}

func (*File) PropertyNames() []string {
	return []string{"read", "write", "close", "info"}
}

func (f *File) read(ctx *symbolic.Context) (*symbolic.ByteSlice, *symbolic.Error) {
	return &symbolic.ByteSlice{}, nil
}

func (f *File) write(ctx *symbolic.Context, data symbolic.Readable) *symbolic.Error {
	return nil
}

func (f *File) close(ctx *symbolic.Context) {
}

func (f *File) info(ctx *symbolic.Context) (*symbolic.FileInfo, *symbolic.Error) {
	return &symbolic.FileInfo{}, nil
}

func (r *File) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%file")))
	return
}

func (r *File) WidestOfType() symbolic.SymbolicValue {
	return &File{}
}
