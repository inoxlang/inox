package local_db_ns

func (kvs *LocalDatabase) IsMutable() bool {
	return true
}

func (kvs *SymbolicLocalDatabase) IsMutable() bool {
	return true
}
