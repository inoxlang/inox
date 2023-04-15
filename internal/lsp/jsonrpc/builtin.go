package jsonrpc

import "context"

// $/cancelRequest
type cancelParams struct {
	ID interface{} `json:"id"`
}

func cancelRequest(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*cancelParams)
	if params.ID == nil {
		return nil, nil
	}
	session := getSession(ctx)
	session.cancelJob(params.ID)
	return nil, nil
}

func CancelRequest() MethodInfo {
	return MethodInfo{
		Name: "$/cancelRequest",
		NewRequest: func() interface{} {
			return &cancelParams{}
		},
		Handler: cancelRequest,
	}
}

// $/progress
// TODO not implemented yet
