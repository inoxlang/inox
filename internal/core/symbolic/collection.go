package symbolic

type Collection interface {
	Container
	_collection()
}

type CollectionMixin struct {
}

func (m CollectionMixin) _collection() {

}
