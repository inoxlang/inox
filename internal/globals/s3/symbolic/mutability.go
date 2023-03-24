package internal

func (b *Bucket) IsMutable() bool {
	return true
}

func (r *GetObjectResponse) IsMutable() bool {
	return true
}

func (r *PutObjectResponse) IsMutable() bool {
	return true
}

func (r *GetBucketPolicyResponse) IsMutable() bool {
	return false
}

func (i *ObjectInfo) IsMutable() bool {
	return false
}
