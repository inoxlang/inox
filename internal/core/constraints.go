package core

import (
	"errors"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	cmap "github.com/orcaman/concurrent-map/v2"
)

// A ConstraintId represents an id that is used to retrieve the constraint (pattern) of a Value.
type ConstraintId uint64

const (
	CONSTRAINTS_KEY = "_constraints_"
)

var (
	constraintMap = cmap.NewWithCustomShardingFunction[ConstraintId, Pattern](
		func(key ConstraintId) uint32 {
			return uint32(key % 16)
		},
	)
	nextConstraintId = ConstraintId(1)

	ErrConstraintViolation = errors.New("constraint violation")
)

func (id ConstraintId) HasConstraint() bool {
	return id > 0
}

func GetConstraint(constraintId ConstraintId) (Pattern, bool) {
	return constraintMap.Get(constraintId)
}

func initializeConstraintMetaproperty(v *Object, block *parse.InitializationBlock) {
	id := nextConstraintId
	nextConstraintId++

	patt := &ObjectPattern{
		complexPropertyPatterns: []*ComplexPropertyConstraint{},
		inexact:                 true,
	}

	for _, stmt := range block.Statements {

		var props []string

		parse.Walk(stmt, func(node, parent, scopeNode parse.Node, ancestorChain []parse.Node, after bool) (parse.TraversalAction, error) {
			if membExpr, ok := node.(*parse.MemberExpression); ok && utils.Implements[*parse.SelfExpression](membExpr.Left) {
				props = append(props, membExpr.PropertyName.Name)
			}
			return parse.ContinueTraversal, nil
		}, nil)

		patt.complexPropertyPatterns = append(patt.complexPropertyPatterns, &ComplexPropertyConstraint{
			Properties: props,
			Expr:       stmt,
		})

	}

	constraintMap.Set(id, patt)
	v.constraintId = id
}
