package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	s3_symbolic "github.com/inoxlang/inox/internal/globals/s3/symbolic"
)

func (b *Bucket) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.Bucket{}, nil
}

func (resp *GetObjectResponse) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.GetObjectResponse{}, nil
}

func (resp *PutObjectResponse) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.PutObjectResponse{}, nil
}

func (resp *GetBucketPolicyResponse) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.GetBucketPolicyResponse{}, nil
}

func (i *ObjectInfo) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.ObjectInfo{}, nil
}
