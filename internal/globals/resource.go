package internal

import (
	"errors"
	"fmt"
	"io"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/mimeconsts"
)

var (
	ErrNilResourceArgument                = errors.New("resource argument is nil")
	ErrFailedToImmediatelyAcquireResource = errors.New("failed to immediately acquire resource")
)

// fetching:
//   - url -> http.get <url>
//   - file path -> file's content is read
//   - dir path -> fs.ls <path>
//

// _readResource is a high level function reading/fetching a resource such as a file or a http resource.
//
// the resource is parsed according to its content type, the content type can be overriden by passing a mimetype value.
// Parsing is supported for the following content types: application/json, text/html, text/plain.
//
// The content type is found in this way:
//   - url -> content-type header
//   - file path -> file's extension
func _readResource(ctx *core.Context, resource core.ResourceName, args ...core.Value) (res core.Value, err error) {
	if resource == nil {
		err = ErrNilResourceArgument
		return
	}

	switch resrc := resource.(type) {
	case core.URL:
		return http_ns.HttpRead(ctx, resrc, args...)
	case core.Path:
		return fs_ns.Read(ctx, resrc, args...)
	default:
		err = fmt.Errorf("resources of type %T not supported yet", resrc)
		return
	}
}

// _getResource is a high level function reading/fetching & acquiring a resource such as a file or a http resource.
func _getResource(ctx *core.Context, resource core.ResourceName, args ...core.Value) (res core.Value, err error) {

	// if ok, err := ctx.TryAcquireResource(resource); !ok {
	// 	return core.Nil, ErrFailedToImmediatelyAcquireResource
	// } else if err != nil {
	// 	return nil, err
	// }

	// releaseResource := true
	// defer func() {
	// 	if releaseResource {
	// 		ctx.ReleaseResource(resource)
	// 	}
	// }()

	resp, readErr := _readResource(ctx, resource, args...)

	if readErr != nil {
		return core.Nil, readErr
	}

	url := resource.(core.URL)

	switch resp := resp.(type) {
	case *core.Object:
		if _, error := core.UrlOf(ctx, resp); error == nil {
			resp = nil
			err = core.ErrResourceHasHardcodedUrlMetaProperty
			return
		}
		//releaseResource = false
		resp.SetURLOnce(ctx, url)
		res = resp
		return
	default:
		err = fmt.Errorf("failed to get %s: type of value is not supported yet: %T", resource, resp)
		return
	}
}

// _createResource creates a resource with its name and an (optional) content.
func _createResource(ctx *core.Context, resource core.ResourceName, args ...core.Value) (core.Value, error) {

	if resource == nil {
		return nil, ErrNilResourceArgument
	}

	switch res := resource.(type) {
	case core.URL:
		args = append([]core.Value{res}, args...)
		resp, err := http_ns.HttpPost(ctx, args...)
		if resp != nil {
			defer resp.Body(ctx).Close()
		}

		if err != nil {
			io.ReadAll(resp.Body(ctx))
			return nil, fmt.Errorf("create: http: %s", err.Error())
		} else {
			return resp, nil
		}
	case core.Path:
		var content *core.Reader

		for _, arg := range args {
			if content != nil {
				return nil, commonfmt.FmtErrXProvidedAtLeastTwice("content")
			}
			content = arg.(core.Readable).Reader()
		}

		if res.IsDirPath() {
			return nil, fs_ns.Mkdir(ctx, res, nil)
		} else {
			if content != nil {
				return nil, fs_ns.Mkfile(ctx, res, content)
			}
			return nil, fs_ns.Mkfile(ctx, res)
		}
	default:
		return nil, fmt.Errorf("resources of type %T not supported yet", res)
	}
}

func _updateResource(ctx *core.Context, resource core.ResourceName, args ...core.Value) (core.Value, error) {
	var content *core.Reader
	var mode core.Identifier

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Identifier:
			if mode != "" {
				return nil, commonfmt.FmtErrXProvidedAtLeastTwice("mod")
			}

			switch v {
			case "append", "replace":
				mode = v
			default:
				return nil, fmt.Errorf("invalid mode '%s'", v)
			}
		case core.Readable:
			if content != nil {
				return nil, commonfmt.FmtErrXProvidedAtLeastTwice("content")
			}
			content = v.Reader()
		default:
			return nil, fmt.Errorf("invalid argument e %#v", arg)
		}
	}

	if resource == nil {
		return nil, ErrNilResourceArgument
	}

	if mode == "" {
		mode = "replace"
	}

	switch res := resource.(type) {
	case core.URL:

		if mode != "replace" {
			return nil, errors.New("update: http only supports replace mode for now")
		}

		resp, err := http_ns.HttpPatch(ctx, res, content)

		if resp != nil {
			defer resp.Body(ctx).Close()
		}

		if err != nil {
			return nil, fmt.Errorf("update: http: %s", err.Error())
		} else {
			contentType := resp.ContentType(ctx)
			b, err := io.ReadAll(resp.Body(ctx))
			if err != nil {
				return nil, fmt.Errorf("update: http: body: %s", err.Error())
			}

			switch contentType {
			case mimeconsts.JSON_CTYPE, mimeconsts.HTML_CTYPE, mimeconsts.PLAIN_TEXT_CTYPE:
				return core.Str(b), nil
			}
			return &core.ByteSlice{Bytes: b, IsDataMutable: true}, nil
		}
	case core.Path:
		if res.IsDirPath() {
			return nil, errors.New("update: directories not supported")
		} else {

			switch mode {
			case "append":
				return nil, fs_ns.AppendToFile(ctx, resource, content)
			case "replace":
				return nil, fs_ns.ReplaceFileContent(ctx, resource, content)
			default:
				panic(core.ErrUnreachable)
			}
		}
	default:
		return nil, fmt.Errorf("resources of type %T not supported yet", res)
	}
}

// _deleteResource deletes a resource such as a file or an HTTP resource.
func _deleteResource(ctx *core.Context, resource core.ResourceName, args ...core.Value) (core.Value, error) {

	for _, arg := range args {
		switch arg.(type) {
		default:
			return nil, fmt.Errorf("invalid argument %#v", arg)
		}
	}

	if resource == nil {
		return nil, ErrNilResourceArgument
	}

	switch res := resource.(type) {
	case core.URL:
		resp, err := http_ns.HttpDelete(ctx, res)
		if resp != nil {
			defer resp.Body(ctx).Close()
		}

		if err != nil {
			return nil, fmt.Errorf("delete: http: %s", err.Error())
		} else {
			contentType := resp.ContentType(ctx)
			b, err := io.ReadAll(resp.Body(ctx))
			if err != nil {
				return nil, fmt.Errorf("delete: http: body: %s", err.Error())
			}

			switch contentType {
			case mimeconsts.JSON_CTYPE, mimeconsts.HTML_CTYPE, mimeconsts.PLAIN_TEXT_CTYPE:
				//TODO: return checked strings ?
				return core.Str(b), nil
			}
			return &core.ByteSlice{Bytes: b, IsDataMutable: true}, nil
		}
	case core.Path:
		return nil, fs_ns.Remove(ctx, res)
	default:
		return nil, fmt.Errorf("resources of type %T not supported yet", res)
	}
}
