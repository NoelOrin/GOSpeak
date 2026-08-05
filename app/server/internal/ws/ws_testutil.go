package ws

import (
	"context"

	"GOSpeak/internal/pkg"
)

// NewTestClient 创建测试用 Client，conn 可为 nil。
// 仅用于 Fanout、Handler 等不需要 WS 通信的测试场景。
func NewTestClient(id string, claims *pkg.Claims) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		id:      id,
		claims:  claims,
		ctx:     ctx,
		cancel:  cancel,
		writeCh: make(chan []byte, 64),
		closed:  make(chan struct{}),
		state:   ConnStateNew,
	}
}
