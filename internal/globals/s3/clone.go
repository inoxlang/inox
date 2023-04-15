package internal

import core "github.com/inoxlang/inox/internal/core"

func (b *Bucket) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (r *GetObjectResponse) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (r *PutObjectResponse) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (r *GetBucketPolicyResponse) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (i ObjectInfo) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}
