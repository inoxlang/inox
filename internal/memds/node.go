package memds

type Node interface {
	Id() NodeId
}

type NodeId int64
