package in_mem_ds

import "errors"

var (
	ErrInvalidOrOutOfBoundsNodeId = errors.New("invalid or out of bound node id ")
)

type Node[T any] interface {
	Id() NodeId
	Data() T
}

type NodeId int64
