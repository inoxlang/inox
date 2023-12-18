package jsonrpc

import "context"

const (
	CANCEL_REQUEST_METHOD = "$/cancelRequest"
)

// $/cancelRequest
type cancelParams struct {
	ID interface{} `json:"id"`
}

func cancelRequest(ctx context.Context, req interface{}) (interface{}, error) {
	params := req.(*cancelParams)
	if params.ID == nil {
		return nil, nil
	}
	session := GetSession(ctx)
	session.cancelJob(params.ID)
	return nil, nil
}

func CancelRequest() MethodInfo {
	return MethodInfo{
		Name: CANCEL_REQUEST_METHOD,
		NewRequest: func() interface{} {
			return &cancelParams{}
		},
		Handler: cancelRequest,
	}
}

// $/progress
// TODO not implemented yet
