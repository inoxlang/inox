package core

import "errors"

var (
	ErrDiscardedChunk = errors.New("chunk is discarded")

	CHUNK_PROPNAMES = []string{"data"}
)

// A DataChunk represents a chunk of any kind of data, DataChunk implements Value.
type DataChunk struct {
	data         Value
	read         bool
	discarded    bool
	getData      func(c *DataChunk) (Value, error)
	getElemCount func(c *DataChunk) int
	merge        func(c *DataChunk, other *DataChunk) error
	//writeToWriter func(c *Chunk, writer *Writer) error
	discard func(c *DataChunk) error
}

func (c *DataChunk) Data(ctx *Context) (Value, error) {
	if c.discarded {
		return nil, ErrDiscardedChunk
	}
	c.read = true
	if c.getData == nil {
		return c.data, nil
	}
	return c.getData(c)
}

func (c *DataChunk) ElemCount() int {
	if c.getElemCount == nil {
		return c.data.(Indexable).Len()
	}
	return c.getElemCount(c)
}

func (c *DataChunk) MergeWith(ctx *Context, other *DataChunk) error {
	if c.discarded {
		return ErrDiscardedChunk
	}

	return c.merge(c, other)
}

func (c *DataChunk) Discard(ctx *Context) error {
	if c.read {
		return nil
	}

	defer func() {
		c.discarded = true
	}()

	return c.discard(c)
}

func (*DataChunk) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (c *DataChunk) Prop(ctx *Context, name string) Value {
	switch name {
	case "data":
		data, err := c.Data(ctx)
		if err != nil {
			panic(err)
		}
		return data
	}
	method, ok := c.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, c))
	}
	return method
}

func (*DataChunk) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*DataChunk) PropertyNames(ctx *Context) []string {
	return CHUNK_PROPNAMES
}
