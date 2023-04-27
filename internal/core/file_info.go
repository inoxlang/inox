package internal

type FileInfo struct {
	Name    Str
	AbsPath Path
	Size    ByteCount
	Mode    FileMode
	ModTime Date
	IsDir   Bool
}

func (i FileInfo) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (i FileInfo) Prop(ctx *Context, name string) Value {
	switch name {
	case "name":
		return i.Name
	case "abs-path":
		return i.AbsPath
	case "size":
		return i.Size
	case "mode":
		return i.Mode
	case "mod-time":
		return i.ModTime
	case "is-dir":
		return i.IsDir
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
