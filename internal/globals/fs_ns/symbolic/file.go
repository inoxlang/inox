package fs_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

type File struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *File) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

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

func (f *File) Prop(name string) symbolic.Value {
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

func (r *File) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("file")
}

func (r *File) WidestOfType() symbolic.Value {
	return &File{}
}
