package hsanalysis

import (
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/sourcecode"
)

type InstallFeature struct {
	BehaviorFullName string
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
