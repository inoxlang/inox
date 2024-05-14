package sourcecode

type ChunkSource interface {
	Name() string             //unique name | URL | path
	UserFriendlyName() string //same as name but path values may be relative.
	Code() string
}

// File is a ChunkSource implementation that represents a source file,
// the file is not necessarily local.
type File struct {
	NameString             string
	UserFriendlyNameString string
	Resource               string //path or url
	ResourceDir            string //path or url
	IsResourceURL          bool
	CodeString             string
}

func (f File) Name() string {
	return f.NameString
}

func (f File) UserFriendlyName() string {
	if f.UserFriendlyNameString == "" {
		return f.NameString
	}
	return f.UserFriendlyNameString
}

func (f File) Code() string {
	return f.CodeString
}

// InMemorySource is a ChunkSource implementation that represents an in-memory chunk source.
type InMemorySource struct {
	NameString string
	CodeString string
}

func (s InMemorySource) Name() string {
	return s.NameString
}

func (s InMemorySource) UserFriendlyName() string {
	return s.NameString
}

func (s InMemorySource) Code() string {
	return s.CodeString
}
