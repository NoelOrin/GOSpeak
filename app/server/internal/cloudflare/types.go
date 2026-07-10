package cloudflare

type NewSessionRequest struct {
	Thirdparty    bool   `json:"thirdparty,omitempty"`
	CorrelationID string `json:"correlationId,omitempty"`
}

type NewSessionResponse struct {
	SessionID string `json:"sessionId"`
}

type SessionInfo struct {
	SessionID   string      `json:"sessionId"`
	AppID       string      `json:"appId"`
	RequesterIP string      `json:"requesterIp,omitempty"`
	IceServers  []IceServer `json:"iceServers,omitempty"`
}

type IceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type TrackRequest struct {
	SessionDescription *SessionDescription `json:"sessionDescription,omitempty"`
	Tracks             []TrackSpec         `json:"tracks,omitempty"`
}

type SessionDescription struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

type TrackSpec struct {
	Location string `json:"location"`
	MID      string `json:"mid,omitempty"`
}

type TracksResponse struct {
	SessionDescription *SessionDescription `json:"sessionDescription,omitempty"`
	Tracks             []TrackResult       `json:"tracks,omitempty"`
}

type TrackResult struct {
	TrackName string `json:"trackName,omitempty"`
	MID       string `json:"mid,omitempty"`
	Location  string `json:"location"`
	SessionID string `json:"sessionId,omitempty"`
}

type CloseTrackRequest struct {
	TrackNames []string `json:"trackNames"`
}

type CloseTrackResponse struct {
	Tracks []CloseTrackResult `json:"tracks,omitempty"`
}

type CloseTrackResult struct {
	TrackName string `json:"trackName"`
}

type APIError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
