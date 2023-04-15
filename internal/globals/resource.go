package internal

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	_fs "github.com/inoxlang/inox/internal/globals/fs"
	_http "github.com/inoxlang/inox/internal/globals/http"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrNilResourceArgument                 = errors.New("resource argument is nil")
	ErrFailedToImmediatelyAcquireResource  = errors.New("failed to immediately acquire resource")
	ErrResourceHasHardcodedUrlMetaProperty = errors.New("resource has hardcoded _url_ metaproperty")
	ErrInvalidResourceContent              = errors.New("invalid resource's content")
	ErrContentTypeParserNotFound           = errors.New("parser not found for content type")
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
	res, _, err = __readResource(ctx, resource, args...)
	return
}

func __readResource(ctx *core.Context, resource core.ResourceName, args ...core.Value) (res core.Value, contentType core.Mimetype, err error) {
	doParse := true
	validateRaw := false
	var b []byte

	var otherArgs []core.Value

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Mimetype:
			if contentType != "" {
				err = core.FmtErrXProvidedAtLeastTwice("content type")
				return
			}
			contentType = v
		case core.Option:
			if v.Name == "raw" && v.Value == core.True {
				doParse = false
			} else {
				otherArgs = append(otherArgs, v)
			}
		default:
			otherArgs = append(otherArgs, v)
			//err = fmt.Errorf("read resource: invalid argument %#v", arg)
		}
	}

	if resource == nil {
		err = ErrNilResourceArgument
		return
	}

	start := time.Now()

	switch resrc := resource.(type) {
	case core.URL:
		resp, respErr := _http.HttpGet(ctx, resrc, otherArgs...)

		if resp != nil {
			defer resp.Body(ctx).Close()
		}
		if respErr != nil {
			err = fmt.Errorf("read: http network error: %s", respErr.Error())
			return
		} else if resp.StatusCode(ctx) >= 400 {
			err = fmt.Errorf("read: http: code %d: %s", resp.StatusCode(ctx), resp.Status(ctx))
			return
		} else {
			b, respErr = io.ReadAll(resp.Body(ctx))
			if respErr != nil {
				err = fmt.Errorf("read: http: body: %s", respErr.Error())
				return
			}

			respContentType, err := _http.Mime_(ctx, core.Str(resp.ContentType(ctx)))

			if err == nil && contentType == "" {
				contentType = respContentType
			}

		}
	case core.Path:
		if len(otherArgs) != 0 {
			err = fmt.Errorf("unused args have been provided")
			return
		}
		if resrc.IsDirPath() {
			_res, lsErr := _fs.ListFiles(ctx, resrc)
			if lsErr != nil {
				err = lsErr
				return
			}

			res = core.ConvertReturnValue(reflect.ValueOf(_res))
			return
		} else {
			var _err error
			b, _err = _fs.ReadEntireFile(ctx, resrc)
			if _err != nil {
				err = _err
				return
			}

			t, ok := core.FILE_EXTENSION_TO_MIMETYPE[filepath.Ext(string(resrc))]
			if ok {
				contentType = t
			}

		}
	default:
		err = fmt.Errorf("resources of type %T not supported yet", resrc)
		return
	}

	ctx.Log("fetching", resource, "took", time.Since(start).Milliseconds(), "ms")

	ct := contentType.WithoutParams()
	switch ct {
	case core.PLAIN_TEXT_CTYPE:
		res = core.Str(b)
	case core.APP_OCTET_STREAM_CTYPE:
		res = core.NewByteSlice(b, false, "")
	default:
		parser, ok := core.GetParser(ct)

		if doParse {
			if !ok {
				res = nil
				contentType = ""
				err = fmt.Errorf("%w (%s)", ErrContentTypeParserNotFound, ct)
				return
			}

			res, err = parser.Parse(ctx, utils.BytesAsString(b))
		} else if validateRaw {
			if !ok {
				res = nil
				contentType = ""
				err = fmt.Errorf("%w (%s)", ErrContentTypeParserNotFound, ct)
				return
			}

			if !parser.Validate(ctx, utils.BytesAsString(b)) {
				res = nil
				contentType = ""
				err = ErrInvalidResourceContent
				return
			}
			res = core.NewByteSlice(b, false, ct)
		} else {
			res = core.NewByteSlice(b, false, ct)
		}
	}

	return
}

// _getResource is a high level function reading/fetching & acquiring a resource such as a file or a http resource.
func _getResource(ctx *core.Context, resource core.ResourceName, args ...core.Value) (res core.Value, err error) {

	if ok, err := ctx.TryAcquireResource(resource); !ok {
		return core.Nil, ErrFailedToImmediatelyAcquireResource
	} else if err != nil {
		return nil, err
	}

	releaseResource := true
	defer func() {
		if releaseResource {
			ctx.ReleaseResource(resource)
		}
	}()

	resp, _, readErr := __readResource(ctx, resource, args...)

	if readErr != nil {
		return core.Nil, readErr
	}

	url := resource.(core.URL)

	switch resp := resp.(type) {
	case *core.Object:
		if _, error := core.UrlOf(ctx, resp); error == nil {
			resp = nil
			err = ErrResourceHasHardcodedUrlMetaProperty
			return
		}
		releaseResource = false
		resp.SetURLOnce(ctx, url)
		res = resp
		return
	default:
		err = fmt.Errorf("failed to get %s: type of value is not supported yet: %T", resource, resp)
		return
	}
}

// _createResource creates a resource with its name and an (optional) content.
func _createResource(ctx *core.Context, resource core.ResourceName, args ...core.Readable) (core.Value, error) {
	var content *core.Reader

	for _, arg := range args {
		if content != nil {
			return nil, core.FmtErrXProvidedAtLeastTwice("content")
		}
		content = arg.Reader()
	}

	if resource == nil {
		return nil, ErrNilResourceArgument
	}

	switch res := resource.(type) {
	case core.URL:
		resp, err := _http.HttpPost(ctx, res, content)
		if resp != nil {
			defer resp.Body(ctx).Close()
		}

		if err != nil {
			io.ReadAll(resp.Body(ctx))
			return nil, fmt.Errorf("create: http: %s", err.Error())
		} else {
			contentType := resp.ContentType(ctx)
			b, err := io.ReadAll(resp.Body(ctx))
			if err != nil {
				return nil, fmt.Errorf("create: http: body: %s", err.Error())
			}

			switch contentType {
			case core.JSON_CTYPE, core.HTML_CTYPE, core.PLAIN_TEXT_CTYPE:
				return core.Str(b), nil
			}
			return &core.ByteSlice{Bytes: b, IsDataMutable: true}, nil
		}
	case core.Path:
		if res.IsDirPath() {
			return nil, _fs.Mkdir(ctx, res)
		} else {
			if content != nil {
				return nil, _fs.Mkfile(ctx, res, content)
			}
			return nil, _fs.Mkfile(ctx, res)
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
				return nil, core.FmtErrXProvidedAtLeastTwice("mod")
			}

			switch v {
			case "append", "replace":
				mode = v
			default:
				return nil, fmt.Errorf("invalid mode '%s'", v)
			}
		case core.Readable:
			if content != nil {
				return nil, core.FmtErrXProvidedAtLeastTwice("content")
			}
			content = v.Reader()
		default:
			return nil, fmt.Errorf("invalid argument e %#v", arg)
		}
	}

	if resource == nil {
		return nil, ErrNilResourceArgument
	}

	switch res := resource.(type) {
	case core.URL:

		if mode != "" {
			return nil, errors.New("update: http does not support append mode yet")
		}

		resp, err := _http.HttpPatch(ctx, res, content)

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
			case core.JSON_CTYPE, core.HTML_CTYPE, core.PLAIN_TEXT_CTYPE:
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
				return nil, _fs.AppendToFile(ctx, resource, content)
			case "replace":
				return nil, _fs.ReplaceFileContent(ctx, resource, content)
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
		resp, err := _http.HttpDelete(ctx, res)
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
			case core.JSON_CTYPE, core.HTML_CTYPE, core.PLAIN_TEXT_CTYPE:
				//TODO: return checked strings ?
				return core.Str(b), nil
			}
			return &core.ByteSlice{Bytes: b, IsDataMutable: true}, nil
		}
	case core.Path:
		return nil, _fs.Remove(ctx, res)
	default:
		return nil, fmt.Errorf("resources of type %T not supported yet", res)
	}
}
