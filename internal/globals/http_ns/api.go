package http_ns

import (
	core "github.com/inoxlang/inox/internal/core"
)

type API struct {
	endpoints map[string]*ApiEndpoint
	tree      *EndpointTreeNode
}

func newEmptyAPI() *API {
	return &API{}
}

type ApiEndpoint struct {
	path string //may have parameters

	operations []ApiOperation
}

type ApiOperation struct {
	id         string //optional
	endpoint   *ApiEndpoint
	httpMethod string

	jsonRequestBody    core.Pattern
	jsonResponseBodies map[uint16]core.Pattern

	handlerModule *core.Module //only set if filesystem routing is used
}

type EndpointTreeNode struct {
	path              string
	segment           string                       //examples: name, group, {id}
	namedChildren     map[string]*EndpointTreeNode // examples if EndpointTree is /data: /data/name, /data/group
	parametrizedChild *EndpointTreeNode            // example if EndpointTree is /users: /users/{id}
	endpoint          *ApiEndpoint                 //can be nil
}

func (api *API) forEachHandlerModule(visit func(mod *core.Module) error) error {
	for _, endpt := range api.endpoints {
		for _, oper := range endpt.operations {
			if oper.handlerModule != nil {
				err := visit(oper.handlerModule)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (api *API) walkEndpointTree(visit func(node *EndpointTreeNode) error) error {
	return api._walkEndpointTree(api.tree, visit)
}

func (api *API) _walkEndpointTree(node *EndpointTreeNode, visit func(node *EndpointTreeNode) error) error {
	if err := visit(node); err != nil {
		return err
	}

	if node.parametrizedChild != nil {
		api._walkEndpointTree(node.parametrizedChild, visit)
	} else {
		for _, endpt := range node.namedChildren {
			api._walkEndpointTree(endpt, visit)
		}
	}

	return nil
}
