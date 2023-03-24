package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	s3_symbolic "github.com/inox-project/inox/internal/globals/s3/symbolic"
)

func (b *Bucket) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.Bucket{}, nil
}

func (resp *GetObjectResponse) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.GetObjectResponse{}, nil
}

func (resp *PutObjectResponse) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.PutObjectResponse{}, nil
}

func (resp *GetBucketPolicyResponse) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.GetBucketPolicyResponse{}, nil
}

func (i *ObjectInfo) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &s3_symbolic.ObjectInfo{}, nil
}
