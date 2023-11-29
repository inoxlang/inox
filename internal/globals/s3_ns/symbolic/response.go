package s3_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

type GetObjectResponse struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *GetObjectResponse) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*GetObjectResponse)
	return ok
}

func (resp *GetObjectResponse) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *GetObjectResponse) Prop(name string) symbolic.Value {
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

func (r *GetObjectResponse) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("get-object-response")
}

func (r *GetObjectResponse) WidestOfType() symbolic.Value {
	return &GetObjectResponse{}
}

type PutObjectResponse struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *PutObjectResponse) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*PutObjectResponse)
	return ok
}

func (resp *PutObjectResponse) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *PutObjectResponse) Prop(name string) symbolic.Value {
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

func (r *PutObjectResponse) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("put-object-response")
}

func (r *PutObjectResponse) WidestOfType() symbolic.Value {
	return &PutObjectResponse{}
}

type GetBucketPolicyResponse struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *GetBucketPolicyResponse) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*GetBucketPolicyResponse)
	return ok
}

func (resp *GetBucketPolicyResponse) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *GetBucketPolicyResponse) Prop(name string) symbolic.Value {
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

func (r *GetBucketPolicyResponse) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("get-bucket-policy-response")
}

func (r *GetBucketPolicyResponse) WidestOfType() symbolic.Value {
	return &GetBucketPolicyResponse{}
}
