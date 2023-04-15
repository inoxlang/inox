package lsp

//
//import (
//	"context"
//	"fmt"
//
//	"github.com/inox-project/inox/internal/lsp/jsonrpc"
//	"github.com/inox-project/inox/internal/lsp/lsp/defines"
//)
//
//type Methods struct {
//	onInitialize func(ctx context.Context, req *defines.InitializeParams) (defines.InitializeResult, *defines.InitializeError)
//}
//
//func (s *Methods) OnInitialize(f func(ctx context.Context, req *defines.InitializeParams) (result defines.InitializeResult, err *defines.InitializeError)) {
//	s.onInitialize = f
//}
//
//// initialize
//func (s *Methods) initialize(ctx context.Context, req interface{}) (interface{}, error) {
//	params := req.(*defines.InitializeParams)
//	logs.Println(params)
//	if s.onInitialize != nil {
//		res, err := s.onInitialize(ctx, params)
//		e := wrapErrorToRespError(err, 0)
//		return res, e
//	}
//	return nil, nil
//}
//
//func (s *Methods) InitializeMethodInfo() *jsonrpc.MethodInfo {
//	if s.onInitialize == nil{
//		return nil
//	}
//	return &jsonrpc.MethodInfo{
//		Name: "initialize",
//		NewRequest: func() interface{} {
//			return &defines.InitializeParams{}
//		},
//		Handler: s.initialize,
//	}
//}
//
//func (s *Methods) GetMethods() []*jsonrpc.MethodInfo {
//	return []*jsonrpc.MethodInfo{
//		s.InitializeMethodInfo(),
//	}
//}
