package symbolic

type Comparable interface {
	Value
	__comparable()
}

type ComparableMixin struct {
}

func __comparable() {

}
