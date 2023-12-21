package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestInitializeVisibilityMetaproperty(t *testing.T) {

	t.Run("public .a", func(t *testing.T) {
		obj := NewObject()

		initializeVisibilityMetaproperty(obj, &parse.InitializationBlock{
			Statements: []parse.Node{
				&parse.ObjectLiteral{
					Properties: []*parse.ObjectProperty{
						{
							Key: &parse.IdentifierLiteral{Name: "public"},
							Value: &parse.KeyListExpression{
								Keys: []parse.Node{&parse.IdentifierLiteral{Name: "a"}},
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

		initializeVisibilityMetaproperty(obj, &parse.InitializationBlock{
			Statements: []parse.Node{
				&parse.ObjectLiteral{
					Properties: []*parse.ObjectProperty{
						{
							Key: &parse.IdentifierLiteral{Name: "visible_by"},
							Value: &parse.DictionaryLiteral{
								Entries: []*parse.DictionaryEntry{
									{
										Key: &parse.UnambiguousIdentifierLiteral{Name: "self"},
										Value: &parse.KeyListExpression{
											Keys: []parse.Node{&parse.IdentifierLiteral{Name: "a"}},
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

		initializeVisibilityMetaproperty(obj, &parse.InitializationBlock{
			Statements: []parse.Node{
				&parse.ObjectLiteral{
					Properties: []*parse.ObjectProperty{
						{
							Key: &parse.IdentifierLiteral{Name: "public"},
							Value: &parse.KeyListExpression{
								Keys: []parse.Node{&parse.IdentifierLiteral{Name: "a"}},
							},
						},
						{
							Key: &parse.IdentifierLiteral{Name: "visible_by"},
							Value: &parse.DictionaryLiteral{
								Entries: []*parse.DictionaryEntry{
									{
										Key: &parse.UnambiguousIdentifierLiteral{Name: "self"},
										Value: &parse.KeyListExpression{
											Keys: []parse.Node{&parse.IdentifierLiteral{Name: "b"}},
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
