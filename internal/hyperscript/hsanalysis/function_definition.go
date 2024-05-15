package hsanalysis

import "github.com/inoxlang/inox/internal/hyperscript/hscode"

type FunctionDefinition struct {
	Name        string
	Namespace   string
	FullName    string
	ArgNames    []string
	CommandList []any
}

func MakeFunctionDefinitionFromNode(node hscode.JSONMap) FunctionDefinition {
	hscode.AssertIsNodeOfType(node, hscode.DefFeature)

	var def FunctionDefinition

	def.Name = node["name"].(string)

	for _, argToken := range node["args"].([]any) {
		m := argToken.(hscode.JSONMap)
		token := hscode.TokenFrom(m)
		def.ArgNames = append(def.ArgNames, token.Value)
	}

	def.CommandList, _ = hscode.GetCommandList(node["start"])

	return def
}
