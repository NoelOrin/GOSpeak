package agora

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var ErrRESTCredentialsMissing = errors.New("agora customer id and secret are required for REST API")

type RESTClient struct {
	appID          string
	customerID     string
	customerSecret string
	client         *http.Client
}

type channelListResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Channels []string `json:"channels"`
	} `json:"data"`
}

type channelUserResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Users []string `json:"users"`
	} `json:"data"`
}

func NewRESTClient(appID, customerID, customerSecret string) *RESTClient {
	return &RESTClient{
		appID:          appID,
		customerID:     customerID,
		customerSecret: customerSecret,
		client:         &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *RESTClient) ListRooms() ([]string, error) {
	var result channelListResponse
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("https://api.agora.io/dev/v1/channel/%s", c.appID), &result); err != nil {
		return nil, fmt.Errorf("agora list rooms: %w", err)
	}
	return result.Data.Channels, nil
}

func (c *RESTClient) GetChannelUsers(channelName string) ([]string, error) {
	var result channelUserResponse
	endpoint := fmt.Sprintf("https://api.agora.io/dev/v1/channel/user/%s/%s", c.appID, url.PathEscape(channelName))
	if err := c.doJSON(http.MethodGet, endpoint, &result); err != nil {
		return nil, fmt.Errorf("agora get channel users: %w", err)
	}
	return result.Data.Users, nil
}

func (c *RESTClient) DeleteChannel(channelName string) error {
	endpoint := fmt.Sprintf("https://api.agora.io/dev/v1/channel/%s/%s", c.appID, url.PathEscape(channelName))
	if err := c.doJSON(http.MethodDelete, endpoint, nil); err != nil {
		return fmt.Errorf("agora delete channel: %w", err)
	}
	return nil
}

func (c *RESTClient) doJSON(method, endpoint string, out interface{}) error {
	if c.customerID == "" || c.customerSecret == "" {
		return ErrRESTCredentialsMissing
	}

	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.customerID, c.customerSecret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
