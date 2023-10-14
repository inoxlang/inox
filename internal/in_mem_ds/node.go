package in_mem_ds



type Node interface {
	Id() NodeId
}

type NodeId int64
