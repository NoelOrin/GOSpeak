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
	AutoDiscover       bool                `json:"autoDiscover,omitempty"`
}

type SessionDescription struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

type TrackSpec struct {
	Location                 string `json:"location"`
	MID                      string `json:"mid,omitempty"`
	SessionID                string `json:"sessionId,omitempty"`
	TrackName                string `json:"trackName,omitempty"`
	BidirectionalMediaStream bool   `json:"bidirectionalMediaStream,omitempty"`
	Kind                     string `json:"kind,omitempty"`
}

type TracksResponse struct {
	SessionDescription            *SessionDescription `json:"sessionDescription,omitempty"`
	Tracks                        []TrackResult       `json:"tracks,omitempty"`
	RequiresImmediateRenegotiation bool               `json:"requiresImmediateRenegotiation,omitempty"`
	ErrorCode                     string              `json:"errorCode,omitempty"`
	ErrorDescription              string              `json:"errorDescription,omitempty"`
}

type TrackResult struct {
	TrackName        string `json:"trackName,omitempty"`
	MID              string `json:"mid,omitempty"`
	Location         string `json:"location"`
	SessionID        string `json:"sessionId,omitempty"`
	ErrorCode        string `json:"errorCode,omitempty"`
	ErrorDescription string `json:"errorDescription,omitempty"`
}

type CloseTrackRequest struct {
	Tracks             []CloseTrackSpec    `json:"tracks,omitempty"`
	TrackNames         []string            `json:"trackNames,omitempty"`
	SessionDescription *SessionDescription `json:"sessionDescription,omitempty"`
	Force              bool                `json:"force,omitempty"`
}

type CloseTrackSpec struct {
	MID       string `json:"mid,omitempty"`
	TrackName string `json:"trackName,omitempty"`
}

type CloseTrackResponse struct {
	Tracks                        []CloseTrackResult  `json:"tracks,omitempty"`
	SessionDescription            *SessionDescription `json:"sessionDescription,omitempty"`
	RequiresImmediateRenegotiation bool               `json:"requiresImmediateRenegotiation,omitempty"`
}

type CloseTrackResult struct {
	MID       string `json:"mid,omitempty"`
	TrackName string `json:"trackName,omitempty"`
}

type RenegotiateRequest struct {
	SessionDescription SessionDescription `json:"sessionDescription"`
}

type APIError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
