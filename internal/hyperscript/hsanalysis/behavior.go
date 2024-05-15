package hsanalysis

import (
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/sourcecode"
)

type Behavior struct {
	Name      string
	FullName  string
	Namespace []string
	Location  sourcecode.PositionRange

	Features []any

	HandledEvents                 []DOMEvent
	InitialElementScopeVarNames   []string // example: {":a", ":b"}
	InitializedDataAttributeNames []string // data-xxx attributes that are properly initialized, example: {"data-count", "data-x"}
	Installs                      []*InstallFeature
	AppliedInstalls               []*InstallFeature

	//Note: applying an install updates InitialElementScopeVarNames and InitializedDataAttributeNames.
}

func MakeBehaviorFromNode(node hscode.JSONMap, location sourcecode.PositionRange) *Behavior {
	hscode.AssertIsNodeOfType(node, hscode.BehaviorFeature)

	behavior := Behavior{
		Name:     node["name"].(string),
		FullName: node["fullName"].(string),
		Location: location,
	}

	namespace, ok := node["nameSpace"].([]any)
	if ok {
		for _, name := range namespace {
			behavior.Namespace = append(behavior.Namespace, name.(string))
		}
	}

	behavior.Features, _ = node["features"].([]any)

	return &behavior
}
