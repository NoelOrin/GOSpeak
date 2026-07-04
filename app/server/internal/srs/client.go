package srs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
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

func (c *Client) DeleteRoom(room string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/api/v1/streams/"+url.PathEscape(room), nil)
	if err != nil {
		return fmt.Errorf("srs build delete room request: %w", err)
	}
	return c.doCodeRequest(req, "delete room")
}

func (c *Client) RemoveParticipant(room, identity string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/api/v1/streams/"+url.PathEscape(room)+"/clients/"+url.PathEscape(identity), nil)
	if err != nil {
		return fmt.Errorf("srs build remove participant request: %w", err)
	}
	return c.doCodeRequest(req, "kick participant")
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
