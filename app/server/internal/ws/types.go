package ws

import "encoding/json"

// Message 是 websocket 的线路协议格式。
// id 用于客户端-服务端请求-应答关联，推送消息不含 id。
type Message struct {
	ID    string          `json:"id,omitempty"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// ACK 是对客户端请求的应答（request.id 非空时发送）。
type ACK struct {
	ID    string      `json:"id"`
	Event string      `json:"event"`
	Data  interface{} `json:"data,omitempty"`
	Error *ACKError   `json:"error,omitempty"`
}

// ACKError 是应答中携带的业务错误。
type ACKError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
