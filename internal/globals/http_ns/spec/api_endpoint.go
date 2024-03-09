package spec

import (
	"errors"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils/pathutils"
)

const (
	MAX_PATH_PARAM_COUNT = 5
)

// APIEndpoint represents an endpoint and its supported operations (GET, POST, ...).
// APIEndpoint is immutable.
type ApiEndpoint struct {
	path                     string                //may have parameters
	pathSegments             []EndpointPathSegment //may have parameters
	hasMethodAgnosticHandler bool

	//Only set if filesystem routing is used. If set .operations is nil.
	methodAgnosticHandler *core.ModulePreparationCache

	operations []ApiOperation
}

func (e ApiEndpoint) PathWithParams() string {
	return e.path
}

func (e ApiEndpoint) HasMethodAgnosticHandler() bool {
	return e.hasMethodAgnosticHandler
}

func (e ApiEndpoint) MethodAgnosticHandler() (*core.ModulePreparationCache, bool) {
	return e.methodAgnosticHandler, e.methodAgnosticHandler != nil
}

func (e ApiEndpoint) Operations() []ApiOperation {
	return e.operations[0:len(e.operations):len(e.operations)]
}

func (e ApiEndpoint) ForEachPathSegment(fn func(segment EndpointPathSegment) error) error {
	for _, segment := range e.pathSegments {
		err := fn(segment)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e ApiEndpoint) GetPathParams(path string) (params PathParams, count int, err error) {

	segmentIndex := -1
	paramIndex := 0

	err = pathutils.ForEachPathSegment(path, func(segment string, startIndex, endIndex int) error {
		segmentIndex++

		if segmentIndex >= len(e.pathSegments) {
			return errors.New("invalid path")
		}
		correspondingSegment := e.pathSegments[segmentIndex]
		if correspondingSegment.ParameterName != "" {
			params[paramIndex] = PathParam{Name: correspondingSegment.ParameterName, Value: segment}
			paramIndex++
		}
		return nil
	})

	if err != nil {
		params = PathParams{}
		count = 0
	}
	count = paramIndex

	return
}

type EndpointPathSegment struct {
	Constant      string
	ParameterName string
}

type PathParams [5]PathParam

type PathParam struct {
	Name  string
	Value string
}

func (param PathParam) IsSet() bool {
	return param != PathParam{}
}

type ApiOperation struct {
	id         string //optional
	endpoint   *ApiEndpoint
	httpMethod string

	jsonRequestBody    core.Pattern
	jsonResponseBodies map[uint16]core.Pattern

	handlerModule *core.ModulePreparationCache //only set if filesystem routing is used.
}

func (op ApiOperation) HttpMethod() string {
	return op.httpMethod
}

func (op ApiOperation) HandlerModule() (*core.ModulePreparationCache, bool) {
	return op.handlerModule, op.handlerModule != nil
}

func (op ApiOperation) JSONRequestBodyPattern() (core.Pattern, bool) {
	return op.jsonRequestBody, op.jsonRequestBody != nil
}
