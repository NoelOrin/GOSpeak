package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://rtc.live.cloudflare.com/v1"

type Client struct {
	appID      string
	appSecret  string
	baseURL    string
	httpClient *http.Client
}

func NewClient(appID, appSecret string) *Client {
	return &Client{
		appID:     appID,
		appSecret: appSecret,
		baseURL:   defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateSession creates an empty Cloudflare Realtime session.
// Official API accepts thirdparty/correlationId only as query params; the request body
// must stay empty or Cloudflare validates it as a sessionDescription payload.
func (c *Client) CreateSession(req *NewSessionRequest) (*NewSessionResponse, error) {
	path := fmt.Sprintf("/apps/%s/sessions/new", c.appID)
	if req != nil {
		q := url.Values{}
		if req.Thirdparty {
			q.Set("thirdparty", "true")
		}
		if req.CorrelationID != "" {
			q.Set("correlationId", req.CorrelationID)
		}
		if encoded := q.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}

	var result NewSessionResponse
	if err := c.doJSON(http.MethodPost, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetSession(sessionID string) (*SessionInfo, error) {
	var result SessionInfo
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/apps/%s/sessions/%s", c.appID, sessionID), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) AddTracks(sessionID string, req *TrackRequest) (*TracksResponse, error) {
	var result TracksResponse
	if err := c.doJSON(http.MethodPost, fmt.Sprintf("/apps/%s/sessions/%s/tracks/new", c.appID, sessionID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Renegotiate(sessionID string, req *RenegotiateRequest) error {
	if req == nil {
		return fmt.Errorf("cloudflare renegotiate: request is nil")
	}
	return c.doJSON(http.MethodPut, fmt.Sprintf("/apps/%s/sessions/%s/renegotiate", c.appID, sessionID), req, nil)
}

func (c *Client) CloseTracks(sessionID string, req *CloseTrackRequest) (*CloseTrackResponse, error) {
	var result CloseTrackResponse
	if err := c.doJSON(http.MethodPut, fmt.Sprintf("/apps/%s/sessions/%s/tracks/close", c.appID, sessionID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteSession terminates a session and closes all of its tracks. Used for
// participant kick and room deletion, where Cloudflare has no room/participant
// concept and the whole session must be torn down.
func (c *Client) DeleteSession(sessionID string) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/apps/%s/sessions/%s", c.appID, sessionID), nil, nil)
}

func (c *Client) doJSON(method, path string, body, target interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cloudflare marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("cloudflare build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.appSecret != "" {
		req.Header.Set("Authorization", "Bearer "+c.appSecret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudflare API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("cloudflare decode response: %w", err)
	}
	return nil
}
