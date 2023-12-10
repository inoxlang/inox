package memds

type ThreadSafety bool

const (
	ThreadSafe   = ThreadSafety(true)
	ThreadUnsafe = ThreadSafety(false)
)
