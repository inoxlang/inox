package hsanalysis

import (
	"slices"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/sourcecode"
)

type InstallFeature struct {
	BehaviorFullName string
	ResolvedBehavior *Behavior
	Fields           []ArgListField

	Location sourcecode.PositionRange
}

func MakeInstallFeatureFromNode(node hscode.JSONMap, location sourcecode.PositionRange) *InstallFeature {
	hscode.AssertIsNodeOfType(node, hscode.InstallFeature)

	feature := InstallFeature{
		BehaviorFullName: node["fullName"].(string),
		Location:         location,
	}

	for _, field := range node["fields"].([]any) {
		fieldMap := field.(hscode.JSONMap)

		feature.Fields = append(feature.Fields, ArgListField{
			Name:  fieldMap["name"].(hscode.JSONMap)["value"].(string),
			Value: fieldMap["value"].(hscode.JSONMap),
		})
	}

	return &feature
}

type ArgListField struct {
	Name  string
	Value hscode.JSONMap
}

func (i *InstallFeature) Resolve(behaviors []*Behavior) (success bool) {
	if i.ResolvedBehavior != nil {
		return true
	}

	wantedBehaviorName := i.BehaviorFullName

	for _, behavior := range behaviors {
		if behavior.FullName != wantedBehaviorName {
			continue
		}
		i.ResolvedBehavior = behavior
		return true
	}

	return false
}

func applyResolvedInstalls(
	installs []*InstallFeature,
	appliedInstalls *[]*InstallFeature,
	elementScopeVarNames *[]string,
	dataAttrNames *[]string,
	handledEvents *[]DOMEvent,
) []Error {
	for _, install := range installs {
		if install.ResolvedBehavior == nil || slices.Contains(*appliedInstalls, install) {
			continue
		}
		*appliedInstalls = append(*appliedInstalls, install)
		resolvedBehavior := install.ResolvedBehavior

		for _, varName := range resolvedBehavior.InitialElementScopeVarNames {
			if !slices.Contains(*elementScopeVarNames, varName) {
				*elementScopeVarNames = append(*elementScopeVarNames, varName)
			}
		}
		for _, attrName := range resolvedBehavior.InitializedDataAttributeNames {
			if !slices.Contains(*dataAttrNames, attrName) {
				*dataAttrNames = append(*dataAttrNames, attrName)
			}
		}
		for _, event := range resolvedBehavior.HandledEvents {
			if !slices.ContainsFunc(*handledEvents, func(e DOMEvent) bool { return e.Type == event.Type }) {
				*handledEvents = append(*handledEvents, event)
			}
		}
	}

	return nil
}
