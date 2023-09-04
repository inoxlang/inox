package s3_ns

import (
	"io"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/johannesboyne/gofakes3"
	"github.com/minio/minio-go/v7"
)

type GetObjectResponse struct {
	output      any
	fakeBackend bool
}

func (r *GetObjectResponse) body() io.ReadCloser {
	if r.fakeBackend {
		output := r.output.(*gofakes3.Object)
		return output.Contents
	} else {
		output := r.output.(*minio.Object)
		return output
	}
}

func (r *GetObjectResponse) ReadAll() ([]byte, error) {
	return io.ReadAll(r.body())
}

func (resp *GetObjectResponse) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (resp *GetObjectResponse) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "body":
		return core.WrapReader(resp.body(), nil)
	default:
		method, ok := resp.GetGoMethod(name)
		if !ok {
			panic(core.FormatErrPropertyDoesNotExist(name, resp))
		}
		return method
	}
}

func (*GetObjectResponse) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*GetObjectResponse) PropertyNames(ctx *core.Context) []string {
	return []string{"body"}
}

type PutObjectResponse struct {
	output      any
	fakeBackend bool
}

func (resp *PutObjectResponse) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (resp *PutObjectResponse) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	default:
		method, ok := resp.GetGoMethod(name)
		if !ok {
			panic(core.FormatErrPropertyDoesNotExist(name, resp))
		}
		return method
	}
}

func (*PutObjectResponse) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*PutObjectResponse) PropertyNames(ctx *core.Context) []string {
	return []string{}
}

type GetBucketPolicyResponse struct {
	s string
}

func (resp *GetBucketPolicyResponse) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (resp *GetBucketPolicyResponse) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "body":
	default:
		return nil
	}
	return nil
}

func (*GetBucketPolicyResponse) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*GetBucketPolicyResponse) PropertyNames(ctx *core.Context) []string {
	return []string{}
}
