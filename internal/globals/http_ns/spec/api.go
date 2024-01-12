package spec

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/inoxlang/inox/internal/core"
)

var (
	ErrEndpointNotFound    = errors.New("endpoint not found")
	ErrAPINotFinalized     = errors.New("API value is not finalized")
	ErrAPIAlreadyFinalized = errors.New("API value is already finalized")
	ErrAPIBeingFinalized   = errors.New("API value is being finalized")
)

// API is a high level type that contains several endpoints, it is immutable.
type API struct {
	endpoints             map[string]*ApiEndpoint
	patternToEndpointPath map[string]string //example: /users/* -> /users/{id}
	tree                  *EndpointTreeNode
}

func NewEmptyAPI() *API {
	return &API{}
}

func NewAPI(endpoints map[string]*ApiEndpoint) (*API, error) {
	api := &API{
		endpoints: endpoints,
	}

	// clean paths
	for endpointPath, endpt := range api.endpoints {
		cleanPath := filepath.Clean(endpointPath)

		if cleanPath != "/" {
			cleanPath = strings.TrimSuffix(cleanPath, "/")
		}

		if endpointPath != cleanPath {
			delete(api.endpoints, endpointPath)
			api.endpoints[cleanPath] = endpt
			endpt.path = cleanPath
		}
	}

	//build the endpoint tree

	api.tree = &EndpointTreeNode{path: "/", namedChildren: map[string]*EndpointTreeNode{}}

	for endpointPath := range api.endpoints {
		if endpointPath == "" || endpointPath[0] != '/' {
			return nil, fmt.Errorf("invalid endpoint path %q", endpointPath)
		}

		if endpointPath == "/" {
			continue
		}

		segments := strings.Split(endpointPath[1:], "/")
		currentNode := api.tree

		//update the endpoint tree
		for i, segment := range segments {
			path := "/" + strings.Join(segments[:i+1], "/")

			if len(segment) == 0 {
				return nil, fmt.Errorf("invalid endpoint path %q: one of the segment is empty", endpointPath)
			}
			if segment[0] == '{' { //parametrized
				if segment[len(segment)-1] != '}' {
					return nil, fmt.Errorf("invalid endpoint path %q: invalid parametrized segment %s", endpointPath, segment)
				}
				child := currentNode.parametrizedChild
				if child == nil {
					child = &EndpointTreeNode{
						path:    path,
						segment: segment,
					}
					currentNode.parametrizedChild = child
				}
				currentNode = child
			} else {
				child, ok := currentNode.namedChildren[segment]
				if !ok {
					if currentNode.namedChildren == nil {
						currentNode.namedChildren = map[string]*EndpointTreeNode{}
					}
					child = &EndpointTreeNode{
						path:    path,
						segment: segment,
					}
					currentNode.namedChildren[segment] = child
				}
				currentNode = child
			}
		}
	}

	// set the endpoint of all nodes in the tree
	err := api._walkEndpointTree(api.tree, func(node *EndpointTreeNode) error {
		node.endpoint = api.endpoints[node.path] //ok if nil
		return nil
	})
	if err != nil {
		return nil, err
	}

	return api, nil
}

// APIEndpoint represents an endpoint and its supported operations (GET, POST, ...).
// APIEndpoint is immutable.
type ApiEndpoint struct {
	path     string //may have parameters
	catchAll bool

	//Only set if filesystem routing is used.
	//If it is set .operations is nil.
	catchAllHandler *core.Module

	operations []ApiOperation
}

func (e ApiEndpoint) PathWithParams() string {
	return e.path
}

func (e ApiEndpoint) CatchAll() bool {
	return e.catchAll
}

func (e ApiEndpoint) CatchAllHandler() (*core.Module, bool) {
	return e.catchAllHandler, e.catchAllHandler != nil
}

func (e ApiEndpoint) Operations() []ApiOperation {
	return e.operations[0:len(e.operations):len(e.operations)]
}

type ApiOperation struct {
	id         string //optional
	endpoint   *ApiEndpoint
	httpMethod string

	jsonRequestBody    core.Pattern
	jsonResponseBodies map[uint16]core.Pattern

	handlerModule *core.Module //only set if filesystem routing is used.
}

func (op ApiOperation) HttpMethod() string {
	return op.httpMethod
}

func (op ApiOperation) HandlerModule() (*core.Module, bool) {
	return op.handlerModule, op.handlerModule != nil
}

type EndpointTreeNode struct {
	path              string
	segment           string                       //examples: name, group, {id}
	namedChildren     map[string]*EndpointTreeNode // examples if EndpointTree is /data: /data/name, /data/group
	parametrizedChild *EndpointTreeNode            // example if EndpointTree is /users: /users/{id}
	endpoint          *ApiEndpoint                 //can be nil
}

func (api *API) GetEndpoint(path string) (*ApiEndpoint, error) {
	path = filepath.Clean(path)

	if path == "/" {
		endpoint, ok := api.endpoints["/"]
		if !ok {
			return nil, ErrEndpointNotFound
		}
		return endpoint, nil
	}

	path = strings.TrimSuffix(path, "/")
	segments := strings.Split(path, "/")[1:]

	node := api.tree.namedChildren[segments[0]]
	if node == nil {
		return nil, ErrEndpointNotFound
	}

	for i, segment := range segments {
		if i == 0 {
			continue
		}

		if node.parametrizedChild != nil {
			//TODO: check
			node = node.parametrizedChild
		} else if child, ok := node.namedChildren[segment]; ok {
			node = child
		} else {
			return nil, ErrEndpointNotFound
		}
	}

	if node.endpoint == nil {
		return nil, ErrEndpointNotFound
	}

	return node.endpoint, nil
}

type HandlerModuleVisitFn func(
	mod *core.Module,
	endpoint *ApiEndpoint,
	//not set if $endpoint.CatchAll() is true.
	operation ApiOperation,
) error

// ForEachHandlerModule visits all handler modules in the API.
// If $endpoint.CatchAll() is true the handler handles all operations and $operation is not set.
func (api *API) ForEachHandlerModule(visit HandlerModuleVisitFn) error {
	for _, endpt := range api.endpoints {
		if endpt.catchAllHandler != nil {
			visit(endpt.catchAllHandler, endpt, ApiOperation{})
		}

		for _, oper := range endpt.operations {
			if oper.handlerModule != nil {
				err := visit(oper.handlerModule, endpt, oper)
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
