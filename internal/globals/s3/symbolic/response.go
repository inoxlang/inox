package internal

import symbolic "github.com/inoxlang/inox/internal/core/symbolic"

type GetObjectResponse struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *GetObjectResponse) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*GetObjectResponse)
	return ok
}

func (r GetObjectResponse) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &GetObjectResponse{}
}

func (resp *GetObjectResponse) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *GetObjectResponse) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "body":
		return &symbolic.Reader{}
	default:
		return symbolic.GetGoMethodOrPanic(name, resp)
	}
}

func (*GetObjectResponse) PropertyNames() []string {
	return []string{"body"}
}

func (r *GetObjectResponse) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *GetObjectResponse) IsWidenable() bool {
	return false
}

func (r *GetObjectResponse) String() string {
	return "%get-object-response"
}

func (r *GetObjectResponse) WidestOfType() symbolic.SymbolicValue {
	return &GetObjectResponse{}
}

type PutObjectResponse struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *PutObjectResponse) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*PutObjectResponse)
	return ok
}

func (r PutObjectResponse) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &PutObjectResponse{}
}

func (resp *PutObjectResponse) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *PutObjectResponse) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "body":
		return &symbolic.Reader{}
	default:
		return symbolic.GetGoMethodOrPanic(name, resp)
	}
}

func (*PutObjectResponse) PropertyNames() []string {
	return []string{"body"}
}

func (r *PutObjectResponse) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *PutObjectResponse) IsWidenable() bool {
	return false
}

func (r *PutObjectResponse) String() string {
	return "%put-object-response"
}

func (r *PutObjectResponse) WidestOfType() symbolic.SymbolicValue {
	return &PutObjectResponse{}
}

type GetBucketPolicyResponse struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *GetBucketPolicyResponse) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*GetBucketPolicyResponse)
	return ok
}

func (r GetBucketPolicyResponse) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &GetBucketPolicyResponse{}
}

func (resp *GetBucketPolicyResponse) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *GetBucketPolicyResponse) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "body":
		return &symbolic.Reader{}
	default:
		return symbolic.GetGoMethodOrPanic(name, resp)
	}
}

func (*GetBucketPolicyResponse) PropertyNames() []string {
	return []string{"body"}
}

func (r *GetBucketPolicyResponse) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *GetBucketPolicyResponse) IsWidenable() bool {
	return false
}

func (r *GetBucketPolicyResponse) String() string {
	return "%get-bucket-policy-response"
}

func (r *GetBucketPolicyResponse) WidestOfType() symbolic.SymbolicValue {
	return &GetBucketPolicyResponse{}
}
