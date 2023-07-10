package s3_ns

import "github.com/inoxlang/inox/internal/core"

func (b *Bucket) Clone(clones map[uintptr]map[int]core.Value, depth int) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (r *GetObjectResponse) Clone(clones map[uintptr]map[int]core.Value, depth int) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (r *PutObjectResponse) Clone(clones map[uintptr]map[int]core.Value, depth int) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (r *GetBucketPolicyResponse) Clone(clones map[uintptr]map[int]core.Value, depth int) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (i ObjectInfo) Clone(clones map[uintptr]map[int]core.Value, depth int) (core.Value, error) {
	return nil, core.ErrNotClonable
}
