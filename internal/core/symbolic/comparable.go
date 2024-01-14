package symbolic

type Comparable interface {
	Value
	__comparable()
}

type ComparableMixin struct {
}

func (mixin ComparableMixin) __comparable() {

}
