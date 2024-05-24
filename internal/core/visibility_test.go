package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/ast"

	"github.com/stretchr/testify/assert"
)

func TestInitializeVisibilityMetaproperty(t *testing.T) {

	t.Run("public .a", func(t *testing.T) {
		obj := NewObject()

		initializeVisibilityMetaproperty(obj, &ast.InitializationBlock{
			Statements: []ast.Node{
				&ast.ObjectLiteral{
					Properties: []*ast.ObjectProperty{
						{
							Key: &ast.IdentifierLiteral{Name: "public"},
							Value: &ast.KeyListExpression{
								Keys: []ast.Node{&ast.IdentifierLiteral{Name: "a"}},
							},
						},
					},
				},
			},
		})

		v, ok := GetVisibility(obj.visibilityId)
		if assert.True(t, ok) {
			assert.Equal(t, &ValueVisibility{publicKeys: []string{"a"}}, v)
		}
	})

	t.Run("visibile by self: .a", func(t *testing.T) {
		obj := NewObject()

		initializeVisibilityMetaproperty(obj, &ast.InitializationBlock{
			Statements: []ast.Node{
				&ast.ObjectLiteral{
					Properties: []*ast.ObjectProperty{
						{
							Key: &ast.IdentifierLiteral{Name: "visible_by"},
							Value: &ast.DictionaryLiteral{
								Entries: []*ast.DictionaryEntry{
									{
										Key: &ast.UnambiguousIdentifierLiteral{Name: "self"},
										Value: &ast.KeyListExpression{
											Keys: []ast.Node{&ast.IdentifierLiteral{Name: "a"}},
										},
									},
								},
							},
						},
					},
				},
			},
		})

		v, ok := GetVisibility(obj.visibilityId)
		if assert.True(t, ok) {
			assert.Equal(t, &ValueVisibility{selfVisibleKeys: []string{"a"}}, v)
		}
	})

	t.Run("public .a & visibile by self: .b", func(t *testing.T) {
		obj := NewObject()

		initializeVisibilityMetaproperty(obj, &ast.InitializationBlock{
			Statements: []ast.Node{
				&ast.ObjectLiteral{
					Properties: []*ast.ObjectProperty{
						{
							Key: &ast.IdentifierLiteral{Name: "public"},
							Value: &ast.KeyListExpression{
								Keys: []ast.Node{&ast.IdentifierLiteral{Name: "a"}},
							},
						},
						{
							Key: &ast.IdentifierLiteral{Name: "visible_by"},
							Value: &ast.DictionaryLiteral{
								Entries: []*ast.DictionaryEntry{
									{
										Key: &ast.UnambiguousIdentifierLiteral{Name: "self"},
										Value: &ast.KeyListExpression{
											Keys: []ast.Node{&ast.IdentifierLiteral{Name: "b"}},
										},
									},
								},
							},
						},
					},
				},
			},
		})

		v, ok := GetVisibility(obj.visibilityId)
		if assert.True(t, ok) {
			visibility := &ValueVisibility{publicKeys: []string{"a"}, selfVisibleKeys: []string{"b"}}
			assert.Equal(t, visibility, v)
		}
	})
}
