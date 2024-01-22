package mapcoll

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
	"github.com/inoxlang/inox/internal/utils"
)

const (
	SERIALIZED_MAP_PATTERN_KEY_KEY = "key"
	SERIALIZED_MAP_PATTERN_VAL_KEY = "value"
)

var (
	MAP_PATTERN = &core.TypePattern{
		Name:          "Map",
		Type:          reflect.TypeOf((*Map)(nil)),
		SymbolicValue: coll_symbolic.ANY_MAP,
		CallImpl: func(typePattern *core.TypePattern, values []core.Serializable) (core.Pattern, error) {
			switch len(values) {
			case 0:
				return nil, commonfmt.FmtMissingArgument("key pattern and value pattern")
			case 1:
				return nil, commonfmt.FmtMissingArgument("value pattern")
			}

			keyPattern, ok := values[0].(core.Pattern)
			if !ok {
				return nil, core.FmtErrInvalidArgumentAtPos(keyPattern, 0)
			}

			valuePattern, ok := values[1].(core.Pattern)
			if !ok {
				return nil, core.FmtErrInvalidArgumentAtPos(keyPattern, 1)
			}

			return NewMapPattern(MapConfig{
				Key:   keyPattern,
				Value: valuePattern,
			}), nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.Value) (symbolic.Pattern, error) {
			switch len(values) {
			case 0:
				return nil, commonfmt.FmtMissingArgument("key pattern and value pattern")
			case 1:
				return nil, commonfmt.FmtMissingArgument("value pattern")
			}

			keyPattern, ok := values[0].(symbolic.Pattern)
			key := symbolic.AsSerializable(keyPattern.SymbolicValue())
			if !ok || !utils.Implements[symbolic.Serializable](key) {
				return nil, commonfmt.FmtErrInvalidArgumentAtPos(1, "key pattern should not match values that are not serializable")
			}

			if key.IsMutable() {
				return nil, commonfmt.FmtErrInvalidArgumentAtPos(1, "key pattern should not match values that are not mutable")
			}

			valuePattern, ok := values[1].(symbolic.Pattern)
			if !ok || !utils.Implements[symbolic.Serializable](symbolic.AsSerializable(valuePattern.SymbolicValue())) {
				return nil, commonfmt.FmtErrInvalidArgumentAtPos(1, "value pattern cannot match values that are not serializable")
			}

			return coll_symbolic.NewMapPattern(keyPattern, valuePattern), nil
		},
	}

	MAP_PATTERN_PATTERN = &core.TypePattern{
		Name:          "map-pattern",
		Type:          reflect.TypeOf((*MapPattern)(nil)),
		SymbolicValue: coll_symbolic.ANY_MAP_PATTERN,
	}

	_ core.DefaultValuePattern   = (*MapPattern)(nil)
	_ core.MigrationAwarePattern = (*MapPattern)(nil)
)

type MapPattern struct {
	config MapConfig

	core.NotCallablePatternMixin
}

func NewMapPattern(config MapConfig) *MapPattern {
	if config.Key == nil {
		config.Key = core.SERIALIZABLE_PATTERN
	}
	if config.Value == nil {
		config.Value = core.SERIALIZABLE_PATTERN
	}
	return &MapPattern{
		config: config,
	}
}

func (p *MapPattern) IsMutable() bool {
	return false
}

func (p *MapPattern) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*MapPattern)
	return ok && p.config.Equal(ctx, otherPatt.config, alreadyCompared, depth+1)
}

func (patt *MapPattern) Test(ctx *core.Context, v core.Value) bool {
	_, ok := v.(*Map)
	if !ok {
		return false
	}

	return true
	//TODO:
	//return patt.config.Equal(ctx, map_.config, map[uintptr]uintptr{}, 0)
}
func (p *MapPattern) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (p *MapPattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(core.ErrNotImplementedYet)
}

func (p *MapPattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}

func (p *MapPattern) DefaultValue(ctx *core.Context) (core.Value, error) {
	return NewMap(ctx, nil), nil
	//return NewMap(ctx, nil, p.config), nil
}

func (p *MapPattern) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	keyPatt, err := p.config.Key.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	valuePatt, err := p.config.Value.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return coll_symbolic.NewMapWithPatterns(keyPatt.(symbolic.Pattern), valuePatt.(symbolic.Pattern)), nil
}

func (p *MapPattern) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("map-pattern(")
	p.config.Key.PrettyPrint(w, config, depth+1, 0)
	w.WriteByte(',')
	p.config.Value.PrettyPrint(w, config, depth+1, 0)
	w.WriteByte(')')
}

func (p *MapPattern) GetMigrationOperations(ctx *core.Context, next core.Pattern, pseudoPath string) ([]core.MigrationOp, error) {
	return nil, errors.New("migrations are not supported by maps for now")
	// nextSet, ok := next.(*MapPattern)
	// if !ok {
	// 	return []core.MigrationOp{core.ReplacementMigrationOp{
	// 		Current:        p,
	// 		Next:           next,
	// 		MigrationMixin: core.MigrationMixin{PseudoPath: pseudoPath},
	// 	}}, nil
	// }

	// return core.GetMigrationOperations(ctx, p.config.Element, nextSet.config.Element, filepath.Join(pseudoPath, "*"))
}

func (p *MapPattern) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	if depth > core.MAX_JSON_REPR_WRITING_DEPTH {
		return core.ErrMaximumJSONReprWritingDepthReached
	}

	write := func(w *jsoniter.Stream) error {
		w.WriteObjectStart()

		//Write key pattern.
		w.WriteObjectField(SERIALIZED_MAP_PATTERN_KEY_KEY)
		keyPatternConfig := core.JSONSerializationConfig{ReprConfig: config.ReprConfig}
		err := p.config.Key.WriteJSONRepresentation(ctx, w, keyPatternConfig, depth+1)
		if err != nil {
			return err
		}

		w.WriteMore()

		//Write value pattern.
		w.WriteObjectField(SERIALIZED_MAP_PATTERN_VAL_KEY)
		valuePatternConfig := core.JSONSerializationConfig{ReprConfig: config.ReprConfig}
		err = p.config.Value.WriteJSONRepresentation(ctx, w, valuePatternConfig, depth+1)
		if err != nil {
			return err
		}

		w.WriteObjectEnd()
		return nil
	}

	if core.NoPatternOrAny(config.Pattern) {
		return core.WriteUntypedValueJSON(MAP_PATTERN_PATTERN.Name, func(w *jsoniter.Stream) error {
			return write(w)
		}, w)
	}
	return write(w)
}

func DeserializeMapPattern(ctx *core.Context, it *jsoniter.Iterator, pattern core.Pattern, try bool) (_ core.Pattern, finalErr error) {
	if it.WhatIsNext() != jsoniter.ObjectValue {
		if try {
			finalErr = core.ErrTriedToParseJSONRepr
			return
		}
		finalErr = core.ErrJsonNotMatchingSchema
		return
	}

	var (
		keyPattern   core.Pattern
		valuePattern core.Pattern
	)

	it.ReadObjectCB(func(it *jsoniter.Iterator, key string) bool {
		switch key {
		case SERIALIZED_MAP_PATTERN_KEY_KEY:
			if it.WhatIsNext() != jsoniter.ObjectValue {
				finalErr = errors.New("invalid representation of key pattern in representation of map pattern")
				return false
			}
			v, err := core.ParseNextJSONRepresentation(ctx, it, nil, false)
			if err != nil {
				finalErr = fmt.Errorf("invalid representation of key pattern in representation of map pattern: %w", err)
				return false
			}
			pattern, ok := v.(core.Pattern)
			if !ok {
				finalErr = errors.New("unexpected non-pattern as key pattern in representation of map pattern")
				return false
			}
			keyPattern = pattern
			return true
		case SERIALIZED_MAP_PATTERN_VAL_KEY:
			if it.WhatIsNext() != jsoniter.ObjectValue {
				finalErr = errors.New("invalid representation of value pattern in representation of map pattern")
				return false
			}
			v, err := core.ParseNextJSONRepresentation(ctx, it, nil, false)
			if err != nil {
				finalErr = fmt.Errorf("invalid representation of value pattern in representation of map pattern: %w", err)
				return false
			}
			pattern, ok := v.(core.Pattern)
			if !ok {
				finalErr = errors.New("unexpected non-pattern as value pattern in representation of map pattern")
				return false
			}
			valuePattern = pattern
			return true
		default:
			finalErr = fmt.Errorf("unexpected property %q in map pattern representation", key)
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

	if keyPattern == nil {
		finalErr = errors.New("missing key pattern in representation of map pattern")
		return
	}

	if valuePattern == nil {
		finalErr = errors.New("missing value pattern in representation of map pattern")
		return
	}

	return NewMapPattern(MapConfig{
		Key:   keyPattern,
		Value: valuePattern,
	}), nil
}
