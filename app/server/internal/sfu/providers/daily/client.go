package daily

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

type room struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type roomsResponse struct {
	Data []room `json:"data"`
}

type meetingTokenResponse struct {
	Token string `json:"token"`
}

type participantsResponse struct {
	Participants []map[string]interface{} `json:"participants"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		baseURL: "https://api.daily.co/v1",
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) CreateMeetingToken(roomName, userName string) (string, error) {
	payload := map[string]interface{}{
		"properties": map[string]interface{}{
			"room_name": roomName,
			"user_name": userName,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("daily encode meeting token request: %w", err)
	}
	req, err := c.newRequest(http.MethodPost, "/meeting-tokens", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	var result meetingTokenResponse
	if err := c.doJSON(req, &result); err != nil {
		return "", err
	}
	if result.Token == "" {
		return "", fmt.Errorf("daily meeting token is empty")
	}
	return result.Token, nil
}

func (c *Client) ListRooms() ([]room, error) {
	req, err := c.newRequest(http.MethodGet, "/rooms", nil)
	if err != nil {
		return nil, err
	}
	var result roomsResponse
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (c *Client) ListParticipants(roomName string) ([]map[string]interface{}, error) {
	req, err := c.newRequest(http.MethodGet, "/rooms/"+roomName+"/presence", nil)
	if err != nil {
		return nil, err
	}
	var result participantsResponse
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}
	return result.Participants, nil
}

func (c *Client) RemoveParticipant(roomName, participantID string) error {
	req, err := c.newRequest(http.MethodPost, "/rooms/"+roomName+"/participants/"+participantID+"/remove", nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, nil)
}

func (c *Client) DeleteRoom(roomName string) error {
	req, err := c.newRequest(http.MethodDelete, "/rooms/"+roomName, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, nil)
}

func (c *Client) newRequest(method, path string, body *bytes.Reader) (*http.Request, error) {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = body
	}
	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("daily build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	return req, nil
}

func (c *Client) doJSON(req *http.Request, target interface{}) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("daily request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("daily request failed: status=%d", resp.StatusCode)
	}
	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("daily decode response: %w", err)
	}
	return nil
}
