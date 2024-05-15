package hsanalysis

import "github.com/inoxlang/inox/internal/hyperscript/hscode"

type InstallFeature struct {
	BehaviorFullName string
	Fields           []ArgListField
}

func MakeInstallFeatureFromNode(node hscode.JSONMap) *InstallFeature {
	hscode.AssertIsNodeOfType(node, hscode.InstallFeature)

	var feature InstallFeature

	feature.BehaviorFullName = node["fullName"].(string)

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
