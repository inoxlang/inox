package fs_ns

func (f *File) IsMutable() bool {
	return true
}

func (evs *FilesystemEventSource) IsMutable() bool {
	return true
}

func (evs *FilesystemIL) IsMutable() bool {
	//TODO: could be false
	return true
}
