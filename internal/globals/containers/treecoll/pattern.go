package treecoll

import (
	"errors"
	"fmt"
	"io"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/jsoniter"
)

const (
	SERIALIZED_TREE_NODE_PATTERN_VALUE_KEY = "value"
)

type TreeNodePattern struct {
	valuePattern core.Pattern
	core.NotCallablePatternMixin
}

func NewTreeNodePattern(valuePattern core.Pattern) *TreeNodePattern {
	return &TreeNodePattern{
		valuePattern: valuePattern,
	}
}

func (patt *TreeNodePattern) Test(ctx *core.Context, v core.Value) bool {
	node, ok := v.(*TreeNode)
	if !ok {
		return false
	}

	return patt.valuePattern.Test(ctx, node.data)
}

func (patt *TreeNodePattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(errors.New("cannot created random tree node"))
}

func (patt *TreeNodePattern) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (patt *TreeNodePattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}

func (p *TreeNodePattern) IsMutable() bool {
	return false
}

func (p *TreeNodePattern) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*TreeNodePattern)
	if !ok {
		return false
	}

	return p.valuePattern.Equal(ctx, otherPattern.valuePattern, map[uintptr]uintptr{}, 0)
}

func (p *TreeNodePattern) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	if depth > core.MAX_JSON_REPR_WRITING_DEPTH {
		return core.ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()
		w.WriteObjectField(SERIALIZED_TREE_NODE_PATTERN_VALUE_KEY)

		valuePatternConfig := core.JSONSerializationConfig{ReprConfig: config.ReprConfig}
		err := p.valuePattern.WriteJSONRepresentation(ctx, w, valuePatternConfig, depth+1)
		if err != nil {
			return err
		}

		w.WriteObjectEnd()
		return nil
	}

	if core.NoPatternOrAny(config.Pattern) {
		return core.WriteUntypedValueJSON(TREE_NODE_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func DeserializeTreeNodePattern(ctx *core.Context, it *jsoniter.Iterator, pattern core.Pattern, try bool) (_ core.Pattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.ObjectValue {
		if try {
			finalErr = core.ErrTriedToParseJSONRepr
			return
		}
		finalErr = core.ErrJsonNotMatchingSchema
		return
	}

	var valuePattern core.Pattern

	it.ReadObjectCB(func(it *jsoniter.Iterator, key string) bool {
		switch key {
		case SERIALIZED_TREE_NODE_PATTERN_VALUE_KEY:
			if it.WhatIsNext() != jsoniter.ObjectValue {
				finalErr = errors.New("invalid representation of value pattern in representation of tree node pattern")
				return false
			}
			v, err := core.ParseNextJSONRepresentation(ctx, it, nil, false)
			if err != nil {
				finalErr = fmt.Errorf("invalid representation of value pattern in representation of tree node pattern: %w", err)
				return false
			}
			pattern, ok := v.(core.Pattern)
			if !ok {
				finalErr = errors.New("unexpected non-pattern as value pattern in representation of tree node pattern")
				return false
			}
			valuePattern = pattern
			return true
		default:
			finalErr = fmt.Errorf("unexpected property %q in float range pattern representation", key)
			return false
		}
	})

	if finalErr != nil {
		return
	}

	if it.Error != nil && it.Error != io.EOF {
		finalErr = it.Error
		return
	}

	if valuePattern == nil {
		finalErr = errors.New("missing value pattern in representation of tree node pattern")
		return
	}

	return NewTreeNodePattern(valuePattern), nil
}
