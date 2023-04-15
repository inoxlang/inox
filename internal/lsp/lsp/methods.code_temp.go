package lsp

const structItemTemp = `	on%s func(ctx context.Context, req *%s) (*%s, %s)`

const noRespStructItemTemp = `	on%s func(ctx context.Context, req *%s) %s`

const structTemp = `
type Methods struct {
    Opt Options
%s
}
`

const methodsTemp = `
func (m *Methods) On%s(f func(ctx context.Context, req *%s) (result *%s, err %s)) {
	m.on%s = f
}
`

const noRespMethodsTemp = `
func (m *Methods) On%s(f func(ctx context.Context, req *%s) (err %s)) {
	m.on%s = f
}
`

const jsonrpcHandlerTemp = `
func (m *Methods) %s(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*%s)
	if m.on%s != nil {
		res, err := m.on%s(ctx, params)
		e := wrapErrorToRespError(err, %s)
		return res, e
	}
%s
}
`

const builtinTemp = `
	res, err := m.builtin%s(ctx, params)
	e := wrapErrorToRespError(err, %s)
	return res, e
`

const noRespBuiltinTemp = `
	err := m.builtin%s(ctx, params)
	e := wrapErrorToRespError(err, %s)
	return nil, e
`
const noBuiltinTemp = `    return nil, nil`

const noRespJsonrpcHandlerTemp = `
func (m *Methods) %s(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*%s)
	if m.on%s != nil {
		err := m.on%s(ctx, params)
		e := wrapErrorToRespError(err, %s)
		return nil, e
	}
%s
}
`

const methodInfoDefaultTemp = `
    if m.on%s == nil{
		return nil
	}`

const methodsInfoTemp = `
func (m *Methods) %sMethodInfo() *jsonrpc.MethodInfo {
%s	
    return &jsonrpc.MethodInfo{
		Name: "%s",
		NewRequest: func() interface{} {
			return %s
		},
		Handler: m.%s,
	}
}
`
const getInfoItemTemp = `	    m.%sMethodInfo(),`

const getInfoTemp = `
func (m *Methods) GetMethods() []*jsonrpc.MethodInfo {
	return []*jsonrpc.MethodInfo{
%s
	}
}`
