package core

import (
	"bytes"
	"fmt"
)

// An Option represents an option with a name and value, Option implements value.
// Inox's flag literals (e.g. `--verbose`) and option expressions (e.g. `--val=100`) evaluate to an Option.
type Option struct {
	Name  string
	Value Value
}

func (opt Option) String() string {
	buff := bytes.NewBufferString("-")

	if len(opt.Name) > 1 {
		buff.WriteRune('-')
	}

	buff.WriteString(opt.Name)

	if boolean, ok := opt.Value.(Bool); !bool(boolean) || !ok {
		buff.WriteRune('=')
		buff.WriteString(fmt.Sprint(opt.Value))
	}

	return buff.String()
}

func SumOptions(ctx *Context, config *Object, options ...Option) (Value, error) {
	sum := &Object{}
	for _, option := range options {
		if sum.HasProp(ctx, option.Name) {
			return Nil, fmt.Errorf("duplicate option '%s'", option.Name)
		}
		if err := sum.SetProp(ctx, option.Name, option.Value); err != nil {
			return Nil, err
		}
	}

	return sum, nil
}
