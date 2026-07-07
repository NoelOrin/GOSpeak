package srs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

type apiResponse struct {
	Code int `json:"code"`
}

type streamsResponse struct {
	Code    int `json:"code"`
	Streams []struct {
		Name string `json:"name"`
	} `json:"streams"`
}

type clientsResponseClient struct {
	ID      string `json:"client"`
	Stream  string `json:"stream"`
	IP      string `json:"ip"`
	Vhost   string `json:"vhost"`
	PageURL string `json:"pageUrl"`
}

type clientsResponse struct {
	Code    int                    `json:"code"`
	Clients []clientsResponseClient `json:"clients"`
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) ListRooms() ([]string, error) {
	resp, err := c.http.Get(c.baseURL + "/api/v1/streams")
	if err != nil {
		return nil, fmt.Errorf("srs list streams: %w", err)
	}
	defer resp.Body.Close()

	var result streamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("srs decode streams: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("srs api error: code=%d", result.Code)
	}

	rooms := make([]string, 0, len(result.Streams))
	for _, stream := range result.Streams {
		if stream.Name == "" {
			continue
		}
		rooms = append(rooms, stream.Name)
	}
	return rooms, nil
}

func (c *Client) fetchClients() ([]clientsResponseClient, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v1/clients/", nil)
	if err != nil {
		return nil, fmt.Errorf("srs build list clients request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("srs list clients: %w", err)
	}
	defer resp.Body.Close()

	var result clientsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("srs decode clients: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("srs api error: code=%d", result.Code)
	}
	return result.Clients, nil
}

func (c *Client) ListParticipants(room string) ([]map[string]interface{}, error) {
	clients, err := c.fetchClients()
	if err != nil {
		return nil, err
	}

	out := make([]map[string]interface{}, 0, len(clients))
	for _, cl := range clients {
		if room != "" && cl.Stream != room {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":     cl.ID,
			"stream": cl.Stream,
			"ip":     cl.IP,
			"vhost":  cl.Vhost,
		})
	}
	return out, nil
}

// KickByStreams 踢出 SRS 上推流到任意给定 stream 的所有 client。
// SRS 无删 stream 原语（DELETE /api/v1/streams/{name} 返 2048），
// 删除 room 语义靠 kick client 实现：先 list /api/v1/clients/ 过滤 stream ∈ targets，
// 逐条 DELETE /api/v1/clients/{id}。返 kicked/remaining 计数。
func (c *Client) KickByStreams(targets []string) (kicked, remaining int, err error) {
	if len(targets) == 0 {
		return 0, 0, nil
	}
	want := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		want[t] = struct{}{}
	}

	clients, err := c.fetchClients()
	if err != nil {
		return 0, 0, err
	}

	var toKick []string
	for _, cl := range clients {
		if _, ok := want[cl.Stream]; ok && cl.ID != "" {
			toKick = append(toKick, cl.ID)
		}
	}
	if len(toKick) == 0 {
		return 0, 0, nil
	}

	for _, id := range toKick {
		req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/api/v1/clients/"+url.PathEscape(id), nil)
		if err != nil {
			remaining++
			continue
		}
		if err := c.doCodeRequest(req, "kick participant"); err != nil {
			remaining++
			continue
		}
		kicked++
	}

	if remaining > 0 {
		return kicked, remaining, fmt.Errorf("srs kick partial failure: kicked=%d remaining=%d", kicked, remaining)
	}
	return kicked, remaining, nil
}

func (c *Client) DeleteRoom(room string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/api/v1/streams/"+url.PathEscape(room), nil)
	if err != nil {
		return fmt.Errorf("srs build delete room request: %w", err)
	}
	return c.doCodeRequest(req, "delete room")
}

func (c *Client) doCodeRequest(req *http.Request, action string) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("srs %s: %w", action, err)
	}
	defer resp.Body.Close()

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("srs decode %s response: %w", action, err)
	}
	if result.Code != 0 {
		return fmt.Errorf("srs api error: code=%d", result.Code)
	}
	return nil
}