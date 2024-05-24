package core

import (
	"errors"

	"github.com/inoxlang/inox/internal/ast"
	cmap "github.com/orcaman/concurrent-map/v2"
)

const (
	VISIBILITY_KEY = "_visibility_"
)

var (
	visibilityMap = cmap.NewWithCustomShardingFunction[VisibilityId, *ValueVisibility](
		func(key VisibilityId) uint32 {
			return uint32(key % 16)
		},
	)
	nextVisibilityId = VisibilityId(1)
	ErrNotVisible    = errors.New("not visible by context")
)

type VisibilityId uint64

func (v VisibilityId) HasVisibility() bool {
	return v > 0
}

// A ValueVisibility specifies what parts of an Inox value are 'visible' during serialization.
type ValueVisibility struct {
	publicKeys      []string
	selfVisibleKeys []string
}

func GetVisibility(id VisibilityId) (*ValueVisibility, bool) {
	return visibilityMap.Get(id)
}

func initializeVisibilityMetaproperty(v *Object, block *ast.InitializationBlock) {

	visibility := &ValueVisibility{}

	objLiteral := block.Statements[0].(*ast.ObjectLiteral)

	//TODO: return error if invalid keys or if there are metaproperties

	for _, prop := range objLiteral.Properties {
		if prop.HasNoKey() {
			continue
		}

		switch prop.Name() {
		case "public":
			keyList := prop.Value.(*ast.KeyListExpression)
			visibility.publicKeys = make([]string, len(keyList.Keys))
			for i, n := range keyList.Names() {
				visibility.publicKeys[i] = n.Name
			}
		case "visible_by":
			dict := prop.Value.(*ast.DictionaryLiteral)

			for _, entry := range dict.Entries {
				switch keyNode := entry.Key.(type) {
				case *ast.UnambiguousIdentifierLiteral:
					switch keyNode.Name {
					case "self":
						keyList := entry.Value.(*ast.KeyListExpression)
						visibility.selfVisibleKeys = make([]string, len(keyList.Keys))
						for i, n := range keyList.Names() {
							visibility.selfVisibleKeys[i] = n.Name
						}
					}
				}
			}
		}
	}

	initializeObjectVisibility(v, visibility)
}

func initializeObjectVisibility(v *Object, visibility *ValueVisibility) {
	id := nextVisibilityId
	nextVisibilityId++

	visibilityMap.Set(id, visibility)

	v.ensureAdditionalFields()
	v.visibilityId = id
}
