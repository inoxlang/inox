package symbolic

import (
	"errors"
	"fmt"
	"maps"
)

var (
	_ = []PatternNext{
		(*ExactValuePattern)(nil), (*ExactStringPattern)(nil), (*ObjectPattern)(nil),
	}

	ErrRuntimeValuesNotSupported = errors.New("removing exact value patterns from pattern is not supported")
)

// PatternNext is the interface that all patterns will implement in the future.
type PatternNext interface {
	Pattern

	//WithExactValuePatternsRemoved should replace exact value patterns with the static type (pattern)
	//of their value.
	WithExactValuePatternsRemoved() (Pattern, error)
}

func RemoveExactValuePatterns(p Pattern) (Pattern, error) {
	pattern, ok := p.(PatternNext)
	if !ok {
		return nil, ErrRuntimeValuesNotSupported
	}
	return pattern.WithExactValuePatternsRemoved()
}

func (p *ExactValuePattern) WithExactValuePatternsRemoved() (Pattern, error) {
	return getStatic(p.value), nil
}

func (p *ExactStringPattern) WithExactValuePatternsRemoved() (Pattern, error) {
	if p.runTimeValue != nil {
		return getStatic(p.runTimeValue), nil
	}

	if p.concretizable != nil {
		return getStatic(p.concretizable), nil
	}

	return &TypePattern{val: ANY_SERIALIZABLE}, nil
}

func (p *ObjectPattern) WithExactValuePatternsRemoved() (Pattern, error) {
	if p.complexPropertyConstraints != nil {
		return nil, fmt.Errorf("%w: object pattern with complex constraints", ErrRuntimeValuesNotSupported)
	}

	transformed := &ObjectPattern{
		inexact:      p.inexact,
		readonly:     p.readonly,
		entries:      maps.Clone(p.entries),
		dependencies: maps.Clone(p.dependencies),
	}

	for name, pattern := range transformed.entries {
		pattern, err := RemoveExactValuePatterns(pattern)
		if err != nil {
			return nil, fmt.Errorf("entry %s of object pattern: %w", name, err)
		}
		transformed.entries[name] = pattern
	}

	for name, dependencies := range transformed.dependencies {
		if dependencies.pattern != nil {
			pattern, err := RemoveExactValuePatterns(dependencies.pattern)
			if err != nil {
				return nil, fmt.Errorf("dependencies of entry %s of object pattern: %w", name, err)
			}

			dependencies.pattern = pattern
			transformed.dependencies[name] = dependencies
		}
	}

	return transformed, nil
}
