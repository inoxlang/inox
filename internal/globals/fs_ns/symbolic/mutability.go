package fs_ns

func (f *File) IsMutable() bool {
	return true
}

func (f *Filesystem) IsMutable() bool {
	return true
}
