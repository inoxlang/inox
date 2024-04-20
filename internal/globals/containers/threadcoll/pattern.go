package threadcoll

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/parse"
)

const (
	SERIALIZED_MSG_THREAD_PATTERN_ELEM_KEY = "element"
)

var (
	MSG_THREAD_PATTERN = &core.TypePattern{
		Name:          "MessageThread",
		Type:          reflect.TypeOf((*MessageThread)(nil)),
		SymbolicValue: coll_symbolic.ANY_THREAD,
		CallImpl: func(ctx *core.Context, typePattern *core.TypePattern, values []core.Serializable) (core.Pattern, error) {
			switch len(values) {
			case 0:
				return nil, commonfmt.FmtMissingArgument("element pattern")
			}

			elementPattern, ok := values[0].(*core.ObjectPattern)
			if !ok {
				return nil, core.FmtErrInvalidArgumentAtPos(elementPattern, 0)
			}

			return NewThreadPattern(ThreadConfig{Element: elementPattern}), nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.Value, optionalNode parse.Node) (symbolic.Pattern, error) {
			switch len(values) {
			case 0:
				return nil, commonfmt.FmtMissingArgument("element pattern")
			}

			elementPattern, ok := values[0].(*symbolic.ObjectPattern)
			if !ok {
				return nil, commonfmt.FmtErrInvalidArgumentAtPos(0, "an object pattern is expected")
			}

			return coll_symbolic.NewMessageThreadPattern(elementPattern), nil
		},
	}

	MSG_THREAD_PATTERN_PATTERN = &core.TypePattern{
		Name:          "message-thread-pattern",
		Type:          reflect.TypeOf((*ThreadPattern)(nil)),
		SymbolicValue: coll_symbolic.ANY_THREAD_PATTERN,
	}

	_ core.MigrationAwarePattern = (*ThreadPattern)(nil)
)

type ThreadPattern struct {
	config ThreadConfig

	core.NotCallablePatternMixin
}

type ThreadConfig struct {
	Element *core.ObjectPattern
}

func (c ThreadConfig) Equal(ctx *core.Context, otherConfig ThreadConfig, alreadyCompared map[uintptr]uintptr, depth int) bool {
	//TODO: check Repr config
	if (c.Element == nil) != (otherConfig.Element == nil) {
		return false
	}

	return c.Element == nil || c.Element.Equal(ctx, otherConfig.Element, alreadyCompared, depth+1)
}

func NewThreadPattern(config ThreadConfig) *ThreadPattern {
	if config.Element == nil {
		config.Element = core.EMPTY_INEXACT_OBJECT_PATTERN
	}
	return &ThreadPattern{
		config: config,
	}
}

func (p *ThreadPattern) IsMutable() bool {
	return false
}

func (p *ThreadPattern) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*ThreadPattern)
	return ok && p.config.Equal(ctx, otherPatt.config, alreadyCompared, depth+1)
}

func (patt *ThreadPattern) Test(ctx *core.Context, v core.Value) bool {
	thread, ok := v.(*MessageThread)
	if !ok {
		return false
	}

	return patt.config.Equal(ctx, thread.config, map[uintptr]uintptr{}, 0)
}
func (p *ThreadPattern) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (p *ThreadPattern) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("message-thread-pattern(")
	p.config.Element.PrettyPrint(w, config, depth+1, 0)
	w.WriteByte(')')
}

func (p *ThreadPattern) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	symbolicElemPattern, err := p.config.Element.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbolic value of element pattern: %w", err)
	}

	return coll_symbolic.NewMessageThreadPattern(symbolicElemPattern.(*symbolic.ObjectPattern)), nil
}

func (p *ThreadPattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(core.ErrNotImplementedYet)
}

func (p *ThreadPattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}

func (p *ThreadPattern) GetMigrationOperations(ctx *core.Context, next core.Pattern, pseudoPath string) ([]core.MigrationOp, error) {
	return nil, errors.New("migrations are not supported by maps for now")
}

func (p *ThreadPattern) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	if depth > core.MAX_JSON_REPR_WRITING_DEPTH {
		return core.ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()
		w.WriteObjectField(SERIALIZED_MSG_THREAD_PATTERN_ELEM_KEY)

		elemConfig := core.JSONSerializationConfig{ReprConfig: config.ReprConfig}
		err := p.config.Element.WriteJSONRepresentation(ctx, w, elemConfig, depth+1)
		if err != nil {
			return err
		}

		w.WriteObjectEnd()
		return nil
	}

	if core.NoPatternOrAny(config.Pattern) {
		return core.WriteUntypedValueJSON(MSG_THREAD_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func DeserializeMessageThreadPattern(ctx *core.Context, it *jsoniter.Iterator, pattern core.Pattern, try bool) (_ core.Pattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.ObjectValue {
		if try {
			finalErr = core.ErrTriedToParseJSONRepr
			return
		}
		finalErr = core.ErrJsonNotMatchingSchema
		return
	}

	var elementPattern *core.ObjectPattern

	it.ReadObjectCB(func(it *jsoniter.Iterator, key string) bool {
		switch key {
		case SERIALIZED_MSG_THREAD_PATTERN_ELEM_KEY:
			if it.WhatIsNext() != jsoniter.ObjectValue {
				finalErr = errors.New("invalid representation of element pattern in representation of thread pattern")
				return false
			}
			v, err := core.ParseNextJSONRepresentation(ctx, it, nil, false)
			if err != nil {
				finalErr = fmt.Errorf("invalid representation of element pattern in representation of thread pattern: %w", err)
				return false
			}
			pattern, ok := v.(*core.ObjectPattern)
			if !ok {
				finalErr = errors.New("unexpected element pattern in representation of thread pattern: not an object pattern")
				return false
			}
			elementPattern = pattern
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

	if elementPattern == nil {
		finalErr = errors.New("missing element pattern in representation of thread pattern")
		return
	}

	return NewThreadPattern(ThreadConfig{
		Element: elementPattern,
	}), nil
}
