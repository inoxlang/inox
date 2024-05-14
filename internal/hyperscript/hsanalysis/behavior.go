package hsanalysis

import "github.com/inoxlang/inox/internal/hyperscript/hscode"

type Behavior struct {
	Name      string
	FullName  string
	Namespace []string
	Features  []any
}

func MakeBehaviorFromBehaviorFeature(node hscode.JSONMap) Behavior {
	hscode.AssertIsNodeOfType(node, hscode.BehaviorFeature)

	var behavior Behavior

	behavior.Name = node["name"].(string)
	behavior.FullName = node["fullName"].(string)

	namespace, ok := node["nameSpace"].([]any)
	if ok {
		for _, name := range namespace {
			behavior.Namespace = append(behavior.Namespace, name.(string))
		}
	}

	behavior.Features, _ = node["features"].([]any)

	return behavior
}
