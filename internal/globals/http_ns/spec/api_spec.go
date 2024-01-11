package spec

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	base "github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	openapiutils "github.com/pb33f/libopenapi/utils"
)

var (
	ErrFailedCreateApiFromOApenpiSpec = errors.New("failed to create API from Open API/Swagger specification")
	ErrUnsupportedMediaTypeinSpec     = errors.New("unsupported media type (in spec)")
	ErrOpenAPIV2SpecNotSupported      = errors.New("specification in the Open Api 2.0 & Swagger formats are not supported, see https://converter.swagger.io/#/Converter/convertByContent to convert to OpenAPI 3+ format")
)

func createAPIFromOpenAPISpec(spec []byte, baseURL core.URL) (*API, error) {
	config := datamodel.DocumentConfiguration{
		AllowFileReferences:   true,
		AllowRemoteReferences: true,
	}
	var err error
	config.BaseURL, err = url.Parse(string(baseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create API: invalid base url: %w", err)
	}

	doc, err := libopenapi.NewDocumentWithConfiguration(spec, &config)
	if err != nil {
		return nil, fmt.Errorf("%w %w", ErrFailedCreateApiFromOApenpiSpec, err)
	}
	specInfo := doc.GetSpecInfo()

	switch specInfo.SpecType {
	case openapiutils.OpenApi3:
		model, errs := doc.BuildV3Model()
		if len(errs) > 0 {
			return nil, fmt.Errorf("%w: %w", ErrFailedCreateApiFromOApenpiSpec, utils.CombineErrors(errs...))
		}
		return getAPIFromOpenApiV3Spec(model)
	case openapiutils.OpenApi2:
		return nil, ErrOpenAPIV2SpecNotSupported
	default:
		return nil, ErrFailedCreateApiFromOApenpiSpec
	}
}

func getAPIFromOpenApiV3Spec(docModel *libopenapi.DocumentModel[v3high.Document]) (*API, error) {
	model := docModel.Model
	index := docModel.Index
	_ = index

	endpoints := map[string]*ApiEndpoint{}

	for path, item := range model.Paths.PathItems {
		endpoint := &ApiEndpoint{
			path: path,
		}
		if item.Head != nil {
			op, err := getApiOperation("HEAD", item.Head, endpoint)
			if err != nil {
				return nil, err
			}
			endpoint.operations = append(endpoint.operations, op)
		}
		if item.Get != nil {
			op, err := getApiOperation("GET", item.Get, endpoint)
			if err != nil {
				return nil, err
			}
			endpoint.operations = append(endpoint.operations, op)
		}
		if item.Post != nil {
			op, err := getApiOperation("POST", item.Post, endpoint)
			if err != nil {
				return nil, err
			}
			endpoint.operations = append(endpoint.operations, op)
		}
		if item.Put != nil {
			op, err := getApiOperation("PUT", item.Put, endpoint)
			if err != nil {
				return nil, err
			}
			endpoint.operations = append(endpoint.operations, op)
		}
		if item.Patch != nil {
			op, err := getApiOperation("PATCH", item.Patch, endpoint)
			if err != nil {
				return nil, err
			}
			endpoint.operations = append(endpoint.operations, op)
		}
		if item.Delete != nil {
			op, err := getApiOperation("DELETE", item.Delete, endpoint)
			if err != nil {
				return nil, err
			}
			endpoint.operations = append(endpoint.operations, op)
		}
		if item.Options != nil {
			op, err := getApiOperation("OPTIONS", item.Options, endpoint)
			if err != nil {
				return nil, err
			}
			endpoint.operations = append(endpoint.operations, op)
		}
		endpoints[path] = endpoint
	}

	return &API{
		endpoints: endpoints,
	}, nil
}

func getApiOperation(method string, op *v3high.Operation, endpoint *ApiEndpoint) (ApiOperation, error) {
	apiOp := ApiOperation{
		endpoint:   endpoint,
		httpMethod: method,
		id:         op.OperationId,
	}

	if op.RequestBody != nil {
		for name, mediaType := range op.RequestBody.Content {
			switch name {
			case mimeconsts.JSON_CTYPE:
				schema, err := mediaType.Schema.BuildSchema()
				if err != nil {
					return ApiOperation{}, err
				}
				pattern, err := getPatternFromLibopenapiSchema(schema)
				if err != nil {
					return ApiOperation{}, fmt.Errorf("failed to create pattern for requests of the endpoint %q: %w", endpoint.path, err)
				}
				apiOp.jsonRequestBody = pattern
			case mimeconsts.MULTIPART_FORM_DATA:
				//TODO
			default:
				return ApiOperation{}, fmt.Errorf("%w: %s", ErrUnsupportedMediaTypeinSpec, name)
			}
		}
	}

	if op.Responses != nil {
		for codeString, resp := range op.Responses.Codes {

			for name, mediaType := range resp.Content {
				switch name {
				//JSON
				case mimeconsts.JSON_CTYPE:
					if apiOp.jsonResponseBodies == nil {
						apiOp.jsonResponseBodies = map[uint16]core.Pattern{}
					}

					code, err := strconv.Atoi(codeString)
					if err != nil || code < 100 || code >= 600 {
						return ApiOperation{}, fmt.Errorf(
							"invalid HTTP status code encountered for one of the responses of %q: %s", endpoint.path, codeString)
					}

					schema, err := mediaType.Schema.BuildSchema()
					if err != nil {
						return ApiOperation{}, err
					}
					pattern, err := getPatternFromLibopenapiSchema(schema)
					if err != nil {
						return ApiOperation{}, fmt.Errorf(
							"failed to create pattern for one of responses (status %d) of the endpoint %q: %w", code, endpoint.path, err)
					}

					apiOp.jsonResponseBodies[uint16(code)] = pattern
				case mimeconsts.MULTIPART_FORM_DATA:
					//TODO
				default:
					return ApiOperation{}, fmt.Errorf("%w: %s", ErrUnsupportedMediaTypeinSpec, name)
				}
			}
		}
	}

	return apiOp, nil
}

func getPatternFromLibopenapiSchema(schema *base.Schema) (core.Pattern, error) {
	schemaBytes, err := schema.RenderInline()
	if err != nil {
		return nil, err
	}

	schemaBytes, err = openapiutils.ConvertYAMLtoJSON(schemaBytes)
	if err != nil {
		return nil, err
	}
	return core.ConvertJsonSchemaToPattern(string(schemaBytes))
}
