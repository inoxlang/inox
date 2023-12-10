package memds

type RWLocker interface {
	Lock()
	Unlock()

	RLock()
	RUnlock()
}
