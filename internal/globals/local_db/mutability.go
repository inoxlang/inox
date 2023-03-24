package internal

func (kvs *LocalDatabase) IsMutable() bool {
	return true
}

func (kvs *SymbolicLocalDatabase) IsMutable() bool {
	return true
}
