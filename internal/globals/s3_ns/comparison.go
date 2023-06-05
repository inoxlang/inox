package s3_ns

import "github.com/inoxlang/inox/internal/core"

func (b *Bucket) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherBucket, ok := other.(*Bucket)
	return ok && b == otherBucket
}

func (r *GetObjectResponse) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherResponse, ok := other.(*GetObjectResponse)
	return ok && r == otherResponse
}

func (r *PutObjectResponse) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherResponse, ok := other.(*PutObjectResponse)
	return ok && r == otherResponse
}

func (r *GetBucketPolicyResponse) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherResponse, ok := other.(*GetBucketPolicyResponse)
	return ok && r == otherResponse
}

func (i *ObjectInfo) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherInfo, ok := other.(*ObjectInfo)
	return ok && i == otherInfo
}
