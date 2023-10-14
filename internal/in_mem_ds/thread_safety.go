package in_mem_ds

type ThreadSafety bool

const (
	ThreadSafe   = ThreadSafety(true)
	ThreadUnsafe = ThreadSafety(false)
)
