package jsonrpc

import (
	"encoding/json"
	"fmt"
)

type BaseMessage struct {
	Jsonrpc string `json:"jsonrpc"`
}

type RequestMessage struct {
	BaseMessage
	ID     interface{}     `json:"id"` // may be int or string
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"` // params, is some struct or slice
}

type NotificationMessage struct {
	BaseMessage
	Method string          `json:"method"` // starts with "/$", server build-in methods.
	Params json.RawMessage `json:"params"` // params, is some struct or slice
}

type ResponseMessage struct {
	BaseMessage
	ID     interface{}     `json:"id"` // may be int or string
	Result interface{}    `json:"result"`
	Error  *ResponseError `json:"error"`
}

type ResponseError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type CancelParams struct {
	ID interface{} `json:"id"` // the method id should be canceled
}

type ProgressParams struct {
	Token interface{} `json:"token"` // int or string
	/**
	 * The progress data.
	 */
	Value interface{} `json:"value"`
}

func (r ResponseError) Error() string {
	return fmt.Sprintf("code: %d, message: %s, data: %v", r.Code, r.Message, r.Data)
}

type BuildInError = ResponseError

const ParseErrorCode = -32700
const InvalidRequestCode = -32600
const MethodNotFoundCode = -32601
const InvalidParamsCode = -32602
const InternalErrorCode = -32603
const jsonrpcReservedErrorRangeStartCode = -32099
const serverErrorStartCode = jsonrpcReservedErrorRangeStartCode
const ServerNotInitializedCode = -32002
const UnknownErrorCodeCode = -32001
const jsonrpcReservedErrorRangeEnCode = -32000
const serverErrorEndCode = jsonrpcReservedErrorRangeEnCode
const lspReservedErrorRangeStartCode = -32899
const ContentModifiedCode = -32801
const RequestCancelledCode = -32800
const lspReservedErrorRangeEndCode = -32800

var ParseError = BuildInError{
	Code:    ParseErrorCode,
	Message: "ParseError",
}
var InvalidRequest = BuildInError{
	Code:    InvalidRequestCode,
	Message: "InvalidRequest",
}
var MethodNotFound = BuildInError{
	Code:    MethodNotFoundCode,
	Message: "MethodNotFound",
}
var InvalidParams = BuildInError{
	Code:    InvalidParamsCode,
	Message: "InvalidParams",
}
var InternalError = BuildInError{
	Code:    InternalErrorCode,
	Message: "InternalError",
}
var ServerNotInitialized = BuildInError{
	Code:    ServerNotInitializedCode,
	Message: "ServerNotInitialized",
}
var UnknownErrorCode = BuildInError{
	Code:    UnknownErrorCodeCode,
	Message: "UnknownErrorCode",
}
var ContentModified = BuildInError{
	Code:    ContentModifiedCode,
	Message: "ContentModified",
}
var RequestCancelled = BuildInError{
	Code:    RequestCancelledCode,
	Message: "RequestCancelled",
}
