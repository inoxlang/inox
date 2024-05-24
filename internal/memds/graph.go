package memds

import "errors"

var (
	ErrInvalidOrOutOfBoundsNodeId = errors.New("invalid or out of bound node id")
	ErrSrcNodeNotExist            = errors.New("source node does not exist")
	ErrDestNodeNotExist           = errors.New("destination node does not exist")
)

type GraphNode[T any] struct {
	Id   NodeId
	Data T
}

type GraphEdge[EdgeData any] struct {
	From NodeId
	To   NodeId
	Data EdgeData
}
