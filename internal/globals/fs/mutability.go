package internal

func (f *File) IsMutable() bool {
	return true
}

func (evs *FilesystemEventSource) IsMutable() bool {
	return true
}
