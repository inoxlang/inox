package core

var _ Value = (*Opaque)(nil)

// Opaque is a Value that wraps a Golang value.
type Opaque struct {
	v any
}

func (o Opaque) Get() any {
	return o.v
}
