package s3_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	s3_symbolic "github.com/inoxlang/inox/internal/globals/s3_ns/symbolic"
)

const (
	NAMESPACE_NAME = "s3"
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		S3Get, func(ctx *symbolic.Context, u *symbolic.URL) (*s3_symbolic.GetObjectResponse, *symbolic.Error) {
			return &s3_symbolic.GetObjectResponse{}, nil
		},
		S3List, func(ctx *symbolic.Context, u *symbolic.URL) (*symbolic.List, *symbolic.Error) {
			return symbolic.NewListOf(&s3_symbolic.ObjectInfo{}), nil
		},
		S3put, func(ctx *symbolic.Context, u *symbolic.URL, reader symbolic.Readable) (*s3_symbolic.GetObjectResponse, *symbolic.Error) {
			return &s3_symbolic.GetObjectResponse{}, nil
		},
		S3Delete, func(ctx *symbolic.Context, u *symbolic.URL, reader symbolic.Readable) *symbolic.Error {
			return nil
		},
		S3GetBucketPolicy, func(ctx *symbolic.Context, u *symbolic.URL) (*s3_symbolic.GetBucketPolicyResponse, *symbolic.Error) {
			return &s3_symbolic.GetBucketPolicyResponse{}, nil
		},
		S3SetBucketPolicy, func(ctx *symbolic.Context, u *symbolic.URL, policy symbolic.SymbolicValue) *symbolic.Error {
			return nil
		},
		S3RemoveBucketPolicy, func(ctx *symbolic.Context, u *symbolic.URL) *symbolic.Error {
			return nil
		},
	})
}

func NewS3namespace() *core.Namespace {
	return core.NewNamespace(NAMESPACE_NAME, map[string]core.Value{
		"get":                  core.ValOf(S3Get),
		"ls":                   core.ValOf(S3List),
		"put":                  core.ValOf(S3put),
		"delete":               core.ValOf(S3Delete),
		"get_bucket_policy":    core.ValOf(S3GetBucketPolicy),
		"set_bucket_policy":    core.ValOf(S3SetBucketPolicy),
		"remove_bucket_policy": core.ValOf(S3RemoveBucketPolicy),
	})
}
