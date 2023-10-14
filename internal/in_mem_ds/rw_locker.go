package in_mem_ds

type RWLocker interface {
	Lock()
	Unlock()

	RLock()
	RUnlock()
}
