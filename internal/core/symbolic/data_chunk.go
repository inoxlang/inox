package internal

var (
	DATA_CHUNK_PROPNAMES = []string{"data"}
)

// A DataChunk represents a symbolic DataChunk.
type DataChunk struct {
	UnassignablePropsMixin
	data SymbolicValue
}

func NewChunk(data SymbolicValue) *DataChunk {
	return &DataChunk{data: data}
}

func (c *DataChunk) Test(v SymbolicValue) bool {
	switch other := v.(type) {
	case *DataChunk:
		return c.data.Test(other.data)
	default:
		return false
	}
}

func (r *DataChunk) WidestOfType() SymbolicValue {
	return &DataChunk{
		data: ANY,
	}
}

func (r *DataChunk) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (r *DataChunk) Prop(name string) SymbolicValue {
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

func (r *DataChunk) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *DataChunk) IsWidenable() bool {
	return false
}

func (r *DataChunk) String() string {
	return "chunk"
}
