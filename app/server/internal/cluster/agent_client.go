package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AgentClient 是 Worker 侧访问 Agent 控制面 HTTP API 的轻量客户端。
type AgentClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewAgentClient 创建 AgentClient。
func NewAgentClient(baseURL, token string) *AgentClient {
	return &AgentClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Register 向 Agent 注册当前节点。
func (c *AgentClient) Register(ctx context.Context, req RegisterRequest) error {
	return c.do(ctx, "/api/v1/cluster/nodes/register", req)
}

// Heartbeat 上报节点运行时状态。
func (c *AgentClient) Heartbeat(ctx context.Context, report HeartbeatReport) error {
	return c.do(ctx, "/api/v1/cluster/nodes/heartbeat", report)
}

// Deregister 注销当前节点。
func (c *AgentClient) Deregister(ctx context.Context, nodeID string) error {
	return c.do(ctx, "/api/v1/cluster/nodes/deregister", map[string]string{
		"node_id": nodeID,
	})
}

func (c *AgentClient) do(ctx context.Context, path string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("agent client marshal %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("agent client request %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("agent client call %s: %w", path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("agent client read %s: %w", path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("agent client %s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("agent client parse %s: %w", path, err)
	}
	if envelope.Code != 0 {
		return fmt.Errorf("agent client %s code=%d msg=%s", path, envelope.Code, envelope.Msg)
	}
	return nil
}
