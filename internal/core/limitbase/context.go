package limitbase

type Context interface {
	IsDone() bool
	IsDoneSlowCheck() bool
	CancelGracefully()
}
