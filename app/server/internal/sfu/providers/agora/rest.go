package agora

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var ErrRESTCredentialsMissing = errors.New("agora customer id and secret are required for REST API")

type RESTClient struct {
	appID          string
	customerID     string
	customerSecret string
	baseURL        string
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

// CreateKickingRuleRequest creates a temporary rule that revokes privileges.
// privileges:
//   - join_channel  => force leave / block rejoin (kick)
//   - publish_audio / publish_video => force stop publishing (mute)
type CreateKickingRuleRequest struct {
	AppID         string   `json:"appid"`
	CName         string   `json:"cname"`
	UID           string   `json:"uid,omitempty"`
	IP            string   `json:"ip,omitempty"`
	Time          int      `json:"time"`
	TimeInSeconds int      `json:"time_in_seconds"`
	Privileges    []string `json:"privileges"`
}

type kickingRuleResponse struct {
	Status  string `json:"status"`
	ID      int    `json:"id"`
	Success bool   `json:"success"`
	Data    struct {
		ID int `json:"id"`
	} `json:"data"`
}

type kickingRuleListResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Rules []struct {
			ID         int      `json:"id"`
			CName      string   `json:"cname"`
			UID        int      `json:"uid"`
			UIDStr     string   `json:"uid_str"`
			Privileges []string `json:"privileges"`
		} `json:"rules"`
	} `json:"data"`
	// Some responses flatten rules at top-level.
	Rules []struct {
		ID         int      `json:"id"`
		CName      string   `json:"cname"`
		UID        int      `json:"uid"`
		UIDStr     string   `json:"uid_str"`
		Privileges []string `json:"privileges"`
	} `json:"rules"`
}

func NewRESTClient(appID, customerID, customerSecret string) *RESTClient {
	return &RESTClient{
		appID:          appID,
		customerID:     customerID,
		customerSecret: customerSecret,
		baseURL:        "https://api.agora.io",
		client:         &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *RESTClient) ListRooms() ([]string, error) {
	var result channelListResponse
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/dev/v1/channel/%s", c.appID), nil, &result); err != nil {
		return nil, fmt.Errorf("agora list rooms: %w", err)
	}
	return result.Data.Channels, nil
}

func (c *RESTClient) GetChannelUsers(channelName string) ([]string, error) {
	var result channelUserResponse
	endpoint := fmt.Sprintf("/dev/v1/channel/user/%s/%s", c.appID, url.PathEscape(channelName))
	if err := c.doJSON(http.MethodGet, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("agora get channel users: %w", err)
	}
	return result.Data.Users, nil
}

func (c *RESTClient) DeleteChannel(channelName string) error {
	endpoint := fmt.Sprintf("/dev/v1/channel/%s/%s", c.appID, url.PathEscape(channelName))
	if err := c.doJSON(http.MethodDelete, endpoint, nil, nil); err != nil {
		return fmt.Errorf("agora delete channel: %w", err)
	}
	return nil
}

// CreateKickingRule creates a privilege rule. Returns rule id (required for unmute).
func (c *RESTClient) CreateKickingRule(channelName, identity string, ttlSeconds int, privileges []string) (int, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 60
	}
	if len(privileges) == 0 {
		privileges = []string{"join_channel"}
	}
	body := CreateKickingRuleRequest{
		AppID:         c.appID,
		CName:         channelName,
		UID:           identity,
		Time:          0,
		TimeInSeconds: ttlSeconds,
		Privileges:    privileges,
	}
	var result kickingRuleResponse
	if err := c.doJSON(http.MethodPost, "/dev/v1/kicking-rule", body, &result); err != nil {
		return 0, fmt.Errorf("agora create kicking rule: %w", err)
	}
	if result.ID != 0 {
		return result.ID, nil
	}
	if result.Data.ID != 0 {
		return result.Data.ID, nil
	}
	return 0, fmt.Errorf("agora create kicking rule: empty rule id in response")
}

// DeleteKickingRule removes a previously created rule (used for unmute).
func (c *RESTClient) DeleteKickingRule(ruleID int) error {
	if ruleID <= 0 {
		return nil
	}
	// Agora docs: DELETE with JSON body {"appid","id"} is accepted by some versions;
	// path form also exists. Prefer path, fall back to body delete on failure.
	endpoint := fmt.Sprintf("/dev/v1/kicking-rule/%d", ruleID)
	if err := c.doJSON(http.MethodDelete, endpoint, nil, nil); err == nil {
		return nil
	}
	body := map[string]interface{}{
		"appid": c.appID,
		"id":    ruleID,
	}
	if err := c.doJSON(http.MethodDelete, "/dev/v1/kicking-rule", body, nil); err != nil {
		return fmt.Errorf("agora delete kicking rule: %w", err)
	}
	return nil
}

// FindKickingRuleIDs lists rule ids matching channel+identity (best-effort recovery).
func (c *RESTClient) FindKickingRuleIDs(channelName, identity string) ([]int, error) {
	var result kickingRuleListResponse
	endpoint := fmt.Sprintf("/dev/v1/kicking-rule?appid=%s", url.QueryEscape(c.appID))
	if err := c.doJSON(http.MethodGet, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("agora list kicking rules: %w", err)
	}
	rules := result.Data.Rules
	if len(rules) == 0 {
		rules = result.Rules
	}
	out := make([]int, 0)
	for _, r := range rules {
		if r.ID == 0 {
			continue
		}
		if channelName != "" && r.CName != "" && r.CName != channelName {
			continue
		}
		uidMatch := false
		if identity != "" {
			if r.UIDStr != "" && r.UIDStr == identity {
				uidMatch = true
			}
			if !uidMatch && fmt.Sprintf("%d", r.UID) == identity {
				uidMatch = true
			}
		} else {
			uidMatch = true
		}
		if uidMatch {
			out = append(out, r.ID)
		}
	}
	return out, nil
}

func (c *RESTClient) doJSON(method, endpoint string, payload interface{}, out interface{}) error {
	if c.customerID == "" || c.customerSecret == "" {
		return ErrRESTCredentialsMissing
	}

	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, reader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.customerID, c.customerSecret)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if len(body) > 0 {
			return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
