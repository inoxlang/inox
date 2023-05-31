package dom_ns

import (
	"errors"
	"fmt"
	"reflect"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	_dom_symbolic "github.com/inoxlang/inox/internal/globals/dom_ns/symbolic"
)

var (
	NODE_PATTERN = &core.TypePattern{
		Name:          "dom.node",
		Type:          reflect.TypeOf(&Node{}),
		SymbolicValue: _dom_symbolic.NewDomNode(&symbolic.Any{}),
		CallImpl: func(values []core.Value) (core.Pattern, error) {
			if len(values) != 1 {
				return nil, errors.New("missing description")
			}
			desc, ok := values[0].(*core.Object)
			if !ok {
				return nil, errors.New("description should be an object")
			}

			var modelPattern core.Pattern
			for k, v := range desc.EntryMap() {
				switch k {
				case "model":
					if patt, ok := v.(core.Pattern); ok {
						modelPattern = patt
					} else {
						modelPattern = core.NewExactValuePattern(v)
					}
				default:
					return nil, fmt.Errorf("invalid key '%s' in description", k)
				}
			}

			return &NodePattern{modelPattern: modelPattern}, nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			if len(values) != 1 {
				return nil, errors.New("missing description")
			}
			desc, ok := values[0].(*symbolic.Object)
			if !ok {
				return nil, errors.New("description should be an object")
			}

			var modelPattern symbolic.Pattern
			err := desc.ForEachEntry(func(k string, v symbolic.SymbolicValue) error {
				switch k {
				case "model":
					if patt, ok := v.(symbolic.Pattern); ok {
						modelPattern = patt
					} else {
						modelPattern = symbolic.NewExactValuePattern(v)
					}
				default:
					return fmt.Errorf("invalid key '%s' in description", k)
				}
				return nil
			})

			if err != nil {
				return nil, err
			}

			return _dom_symbolic.NewDomNodePattern(modelPattern), nil
		},
	}
)

func _a(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "a", desc)
}

func _div(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "div", desc)
}

func _span(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "span", desc)
}

func _ul(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "ul", desc)
}

func _ol(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "ol", desc)
}

func _li(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "li", desc)
}

func _svg(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "svg", desc)
}

func _h1(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "h1", desc)
}

func _h2(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "h2", desc)
}

func _h3(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "h3", desc)
}

func _h4(ctx *core.Context, desc *core.Object) *Node {
	return NewNode(ctx, "h4", desc)
}
