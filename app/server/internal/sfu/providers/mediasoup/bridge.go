package mediasoup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrParticipantNotFound is returned when CloseParticipant gets a 404 from the bridge.
var ErrParticipantNotFound = fmt.Errorf("participant not found")

type BridgeClient struct {
	baseURL string
	client  *http.Client
}

type roomListResponse struct {
	Rooms []string `json:"rooms"`
}

type TransportParams struct {
	ID             string          `json:"id"`
	IceParameters  json.RawMessage `json:"iceParameters"`
	IceCandidates  json.RawMessage `json:"iceCandidates"`
	DtlsParameters json.RawMessage `json:"dtlsParameters"`
	SctpParameters json.RawMessage `json:"sctpParameters"`
}

type ProduceResult struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type ConsumeResult struct {
	ID            string          `json:"id"`
	ProducerID    string          `json:"producerId"`
	Kind          string          `json:"kind"`
	RTPParameters json.RawMessage `json:"rtpParameters"`
}

type ParticipantInfo struct {
	Identity         string `json:"identity"`
	ProducerCount    int    `json:"producerCount"`
	HasSendTransport bool   `json:"hasSendTransport"`
	HasRecvTransport bool   `json:"hasRecvTransport"`
}

func NewBridgeClient(baseURL string) *BridgeClient {
	return &BridgeClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *BridgeClient) CreateRouter(roomID string) error {
	body, err := json.Marshal(map[string]string{"roomId": roomID})
	if err != nil {
		return err
	}
	return b.do(http.MethodPost, "/rooms", bytes.NewReader(body), nil)
}

func (b *BridgeClient) DeleteRouter(roomID string) error {
	return b.do(http.MethodDelete, "/rooms/"+roomID, nil, nil)
}

func (b *BridgeClient) ListRouters() ([]string, error) {
	var result roomListResponse
	if err := b.do(http.MethodGet, "/rooms", nil, &result); err != nil {
		return nil, err
	}
	return result.Rooms, nil
}

func (b *BridgeClient) Health() error {
	return b.do(http.MethodGet, "/health", nil, nil)
}

func (b *BridgeClient) GetRouterCapabilities(roomID string) (json.RawMessage, error) {
	var result struct {
		RTPCapabilities json.RawMessage `json:"rtpCapabilities"`
	}
	if err := b.do(http.MethodGet, "/rooms/"+roomID+"/rtp-capabilities", nil, &result); err != nil {
		return nil, err
	}
	return result.RTPCapabilities, nil
}

func (b *BridgeClient) CreateTransport(roomID, identity, direction string) (*TransportParams, error) {
	body, err := json.Marshal(map[string]string{
		"identity":  identity,
		"direction": direction,
	})
	if err != nil {
		return nil, err
	}
	var result TransportParams
	if err := b.do(http.MethodPost, "/rooms/"+roomID+"/transports", bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type RestartIceResult struct {
	IceParameters json.RawMessage `json:"iceParameters"`
}

func (b *BridgeClient) RestartIce(roomID, transportID string) (json.RawMessage, error) {
	var result RestartIceResult
	if err := b.do(http.MethodPost, "/rooms/"+roomID+"/transports/"+transportID+"/restart-ice", bytes.NewReader([]byte("{}")), &result); err != nil {
		return nil, err
	}
	return result.IceParameters, nil
}

func (b *BridgeClient) ConnectTransport(roomID, transportID string, dtlsParameters json.RawMessage) error {
	body, err := json.Marshal(map[string]interface{}{"dtlsParameters": dtlsParameters})
	if err != nil {
		return err
	}
	return b.do(http.MethodPost, "/rooms/"+roomID+"/transports/"+transportID+"/connect", bytes.NewReader(body), nil)
}

func (b *BridgeClient) Produce(roomID, transportID, kind string, rtpParameters, appData json.RawMessage) (*ProduceResult, error) {
	body, err := json.Marshal(map[string]interface{}{
		"transportId":   transportID,
		"kind":          kind,
		"rtpParameters": rtpParameters,
		"appData":       appData,
	})
	if err != nil {
		return nil, err
	}
	var result ProduceResult
	if err := b.do(http.MethodPost, "/rooms/"+roomID+"/produce", bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (b *BridgeClient) Consume(roomID, transportID, producerID string, rtpCapabilities json.RawMessage) (*ConsumeResult, error) {
	body, err := json.Marshal(map[string]interface{}{
		"transportId":     transportID,
		"producerId":      producerID,
		"rtpCapabilities": rtpCapabilities,
	})
	if err != nil {
		return nil, err
	}
	var result ConsumeResult
	if err := b.do(http.MethodPost, "/rooms/"+roomID+"/consume", bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (b *BridgeClient) ListParticipants(roomID string) ([]ParticipantInfo, error) {
	var result struct {
		Participants []ParticipantInfo `json:"participants"`
	}
	if err := b.do(http.MethodGet, "/rooms/"+roomID+"/participants", nil, &result); err != nil {
		return nil, err
	}
	return result.Participants, nil
}

func (b *BridgeClient) CloseParticipant(roomID, identity string) ([]string, error) {
	var result struct {
		OK               bool     `json:"ok"`
		ClosedProducerID []string `json:"closedProducerIds"`
	}
	err := b.do(http.MethodPost, "/rooms/"+roomID+"/participants/"+url.PathEscape(identity)+"/close", bytes.NewReader([]byte("{}")), &result)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrParticipantNotFound
		}
		return nil, err
	}
	return result.ClosedProducerID, nil
}

func (b *BridgeClient) PauseProducer(roomID, producerID string) error {
	return b.do(http.MethodPost, "/rooms/"+roomID+"/producers/"+producerID+"/pause", bytes.NewReader([]byte("{}")), nil)
}

func (b *BridgeClient) ResumeProducer(roomID, producerID string) error {
	return b.do(http.MethodPost, "/rooms/"+roomID+"/producers/"+producerID+"/resume", bytes.NewReader([]byte("{}")), nil)
}

func (b *BridgeClient) PauseParticipant(roomID, identity string) error {
	return b.do(http.MethodPost, "/rooms/"+roomID+"/participants/"+url.PathEscape(identity)+"/pause", bytes.NewReader([]byte("{}")), nil)
}

func (b *BridgeClient) ResumeParticipant(roomID, identity string) error {
	return b.do(http.MethodPost, "/rooms/"+roomID+"/participants/"+url.PathEscape(identity)+"/resume", bytes.NewReader([]byte("{}")), nil)
}

func (b *BridgeClient) do(method, path string, body io.Reader, out interface{}) error {
	req, err := http.NewRequest(method, b.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("mediasoup bridge: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("mediasoup bridge: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("mediasoup bridge: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("mediasoup bridge: %s %s status=%d: %s", method, path, resp.StatusCode, string(data))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("mediasoup bridge: decode response: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status=404")
}
