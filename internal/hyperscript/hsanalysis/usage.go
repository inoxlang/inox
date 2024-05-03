package hsanalysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
)

func AddUsedFeaturesAndCommands(node hscode.JSONMap, features map[string]hsgen.Definition, commands map[string]hsgen.Definition) {

	hscode.Walk(node, func(node hscode.JSONMap, nodeType hscode.NodeType, parent hscode.JSONMap, parentType hscode.NodeType, ancestorChain []hscode.JSONMap, after bool) (action hscode.AstTraversalAction, err error) {
		action = hscode.ContinueAstTraversal

		commandName := strings.TrimSuffix(string(nodeType), "Command")
		if commandName != string(nodeType) && hsgen.IsBuiltinCommandName(commandName) {
			def, ok := hsgen.GetBuiltinCommandDefinition(commandName)
			if ok {
				commands[def.Name] = def
			}
			//Note: some commands are also features (e.g. 'set').
			def, ok = hsgen.GetBuiltinFeatureDefinition(commandName)
			if ok {
				features[def.Name] = def
			}
		}

		featureName := strings.TrimSuffix(string(nodeType), "Feature")
		if featureName != string(nodeType) && hsgen.IsBuiltinFeatureName(featureName) {
			def, ok := hsgen.GetBuiltinFeatureDefinition(featureName)
			if ok {
				features[def.Name] = def
			}
		}

		return
	}, nil)

}

func GuessUsedFeaturesAndCommandsFromTokens(tokens []hscode.Token, features map[string]hsgen.Definition, commands map[string]hsgen.Definition) {
	//Find what features and commands are used.

	for _, token := range tokens {
		if token.Type != hscode.IDENTIFIER {
			continue
		}

		if hsgen.IsBuiltinFeatureName(token.Value) {
			def, ok := hsgen.GetBuiltinFeatureDefinition(token.Value)
			if ok {
				features[def.Name] = def
			}
		}

		if hsgen.IsBuiltinCommandName(token.Value) {
			def, ok := hsgen.GetBuiltinCommandDefinition(token.Value)
			if ok {
				commands[def.Name] = def
			}
		}

		//Note: some commands are also features (e.g. 'set').
	}
}
