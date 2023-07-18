package s3_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

type GetObjectResponse struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *GetObjectResponse) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*GetObjectResponse)
	return ok
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

func (r *GetObjectResponse) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%get-object-response")))
	return
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

func (r *PutObjectResponse) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%put-object-response")))
	return
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

func (r *GetBucketPolicyResponse) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%get-bucket-policy-response")))
	return
}

func (r *GetBucketPolicyResponse) WidestOfType() symbolic.SymbolicValue {
	return &GetBucketPolicyResponse{}
}
