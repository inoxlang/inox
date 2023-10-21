package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	DATA_CHUNK_PROPNAMES = []string{"data"}
)

// A DataChunk represents a symbolic DataChunk.
type DataChunk struct {
	UnassignablePropsMixin
	data Value
}

func NewChunk(data Value) *DataChunk {
	return &DataChunk{data: data}
}

func (c *DataChunk) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch other := v.(type) {
	case *DataChunk:
		return c.data.Test(other.data, state)
	default:
		return false
	}
}

func (r *DataChunk) WidestOfType() Value {
	return &DataChunk{
		data: ANY,
	}
}

func (r *DataChunk) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (r *DataChunk) Prop(name string) Value {
	switch name {
	case "data":
		return r.data
	}
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (*DataChunk) PropertyNames() []string {
	return DATA_CHUNK_PROPNAMES
}

func (r *DataChunk) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("chunk")
}
