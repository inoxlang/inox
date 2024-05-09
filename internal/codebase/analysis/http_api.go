package analysis

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/parse"
)

// tryGetStaticAndDynamicDirs tryies to determine the static and dynamic directories specified in the http.Server's configuration.
// Both results may be empty.
func (a *analyzer) tryGetStaticAndDynamicDirs(app InoxModuleInfo) (static, dynamic string) {

	defer func() {
		//Discard directories that are not absolute.
		if static != "" && static[0] != '/' {
			static = ""
		}
		if dynamic != "" && dynamic[0] != '/' {
			dynamic = ""
		}
	}()

	if app.SymbolicData == nil {
		return
	}

	serverCreationCall, _ := parse.FindNodeAndChain(app.Module.TopLevelNode, nil, func(n *parse.CallExpression, _ bool, ancestors []parse.Node) bool {
		return parse.IsIdentMemberExprWithNames(n.Callee, globalnames.HTTP_NS, "Server")
	})

	if serverCreationCall == nil && len(serverCreationCall.Arguments) != 2 {
		return
	}

	configArgNode := serverCreationCall.Arguments[1]
	symbConfigValue, ok := app.SymbolicData.GetMostSpecificNodeValue(configArgNode)
	if !ok {
		return "", ""
	}
	obj, ok := symbConfigValue.(*symbolic.Object)
	if !ok {
		return "", ""
	}

	//Try to get the routing configuration

	routingConfig, _, _ := obj.GetProperty(http_ns.HANDLING_DESC_ROUTING_PROPNAME)

	routingConfigObj, ok := routingConfig.(*symbolic.Object)
	if !ok {
		return
	}

	//Try to to get the static dir

	staticPropVal, _, _ := routingConfigObj.GetProperty(http_ns.STATIC_DIR_PROPNAME)
	if path, ok := staticPropVal.(*symbolic.Path); ok && path.IsConcretizable() {
		static, _ = path.StringValue()
	}

	//Try to to get the dynamic dir

	dynamicPropVal, _, _ := routingConfigObj.GetProperty(http_ns.DYNAMIC_DIR_PROPNAME)
	if path, ok := dynamicPropVal.(*symbolic.Path); ok && path.IsConcretizable() {
		dynamic, _ = path.StringValue()
	}

	return
}
