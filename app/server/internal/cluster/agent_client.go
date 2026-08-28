package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrClusterNodeNotFound is returned when the Agent no longer knows the node.
// The worker must re-register before heartbeats can succeed again.
var ErrClusterNodeNotFound = errors.New("cluster node not found")

type agentHTTPError struct {
	status int
	body   string
}

func (e *agentHTTPError) Error() string {
	return fmt.Sprintf("agent client status=%d body=%s", e.status, strings.TrimSpace(e.body))
}

// AgentClient 是 Worker 侧访问 Agent 控制面 HTTP API 的轻量客户端。
type AgentClient struct {
	baseURL    string
	token      string
	nodeSecret string
	http       *http.Client
}

// NewAgentClient 创建 AgentClient。
func NewAgentClient(baseURL, token, nodeSecret string) *AgentClient {
	return &AgentClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:      strings.TrimSpace(token),
		nodeSecret: strings.TrimSpace(nodeSecret),
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Register 向 Agent 注册当前节点。
func (c *AgentClient) Register(ctx context.Context, req RegisterRequest) error {
	req.NodeSecret = c.nodeSecret
	return c.do(ctx, "/api/v1/cluster/nodes/register", req)
}

// Heartbeat 上报节点运行时状态；附带 node_secret 供 Agent 校验身份。
func (c *AgentClient) Heartbeat(ctx context.Context, report HeartbeatReport) error {
	report.NodeSecret = c.nodeSecret
	return c.do(ctx, "/api/v1/cluster/nodes/heartbeat", report)
}

// Deregister 注销当前节点。
func (c *AgentClient) Deregister(ctx context.Context, nodeID string) error {
	return c.do(ctx, "/api/v1/cluster/nodes/deregister", map[string]string{
		"node_id": nodeID, "node_secret": c.nodeSecret,
	})
}

// do retries transient network/5xx/429 failures with short backoff.
func (c *AgentClient) do(ctx context.Context, path string, payload interface{}) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(500*(1<<attempt)) * time.Millisecond // 1s, 2s
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		err := c.doOnce(ctx, path, payload)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryableAgentError(err) {
			return err
		}
	}
	return lastErr
}

func retryableAgentError(err error) bool {
	if errors.Is(err, ErrClusterNodeNotFound) {
		return false
	}
	var httpErr *agentHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.status == http.StatusTooManyRequests || httpErr.status >= http.StatusInternalServerError
	}
	return true
}

func (c *AgentClient) doOnce(ctx context.Context, path string, payload interface{}) error {
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
		return &agentHTTPError{status: resp.StatusCode, body: string(raw)}
	}

	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("agent client parse %s: %w", path, err)
	}
	if envelope.Code != 0 {
		if envelope.Code == 3001 {
			return ErrClusterNodeNotFound
		}
		return fmt.Errorf("agent client %s code=%d msg=%s", path, envelope.Code, envelope.Msg)
	}
	return nil
}
