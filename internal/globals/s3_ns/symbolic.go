package s3_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	s3_symbolic "github.com/inoxlang/inox/internal/globals/s3_ns/symbolic"
)

func (b *Bucket) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &s3_symbolic.Bucket{}, nil
}

func (resp *GetObjectResponse) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &s3_symbolic.GetObjectResponse{}, nil
}

func (resp *PutObjectResponse) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &s3_symbolic.PutObjectResponse{}, nil
}

func (resp *GetBucketPolicyResponse) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &s3_symbolic.GetBucketPolicyResponse{}, nil
}

func (i *ObjectInfo) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &s3_symbolic.ObjectInfo{}, nil
}
