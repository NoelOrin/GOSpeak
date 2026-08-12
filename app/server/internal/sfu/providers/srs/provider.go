package srs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

type Service struct {
	client     *Client
	secret     string
	host       string
	publicHost string
	whipURL    string
	registry   pkg.RoomRegistry
	resolver   pkg.StreamRoomResolver
	muteRules  sfu.MuteRuleStore
}

func (s *Service) SetRoomRegistry(r pkg.RoomRegistry) {
	s.registry = r
}

// SetStreamRoomResolver 注入 stream→room 反查（signal.Hub 实现）。
func (s *Service) SetStreamRoomResolver(r pkg.StreamRoomResolver) {
	s.resolver = r
}

// SetMuteRuleStore 注入跨实例禁推黑名单（nats → memory），实现 sfu.MuteRuleStoreSetter。
func (s *Service) SetMuteRuleStore(store sfu.MuteRuleStore) {
	if store == nil {
		store = sfu.NewMemoryMuteRuleStore()
	}
	s.muteRules = store
}

func NewService(cfg *config.Config) *Service {
	host := strings.TrimSpace(cfg.SRSHost)
	apiPort := strings.TrimSpace(cfg.SRSApiPort)
	if host == "" {
		host = "localhost"
	}
	if apiPort == "" {
		apiPort = "1985"
	}
	whipURL := strings.TrimSpace(cfg.SRSWHIPURL)
	if whipURL == "" {
		whipURL = "/rtc/v1/whip/"
	}
	baseURL := fmt.Sprintf("http://%s:%s", host, apiPort)
	return &Service{
		client:     NewClient(baseURL),
		secret:     cfg.SRSSecret,
		host:       baseURL,
		publicHost: strings.TrimSpace(cfg.SRSPublicHost),
		whipURL:    whipURL,
	}
}

// Close 释放 HTTP 空闲连接。
func (s *Service) Close() error {
	if s.client != nil && s.client.http != nil {
		s.client.http.CloseIdleConnections()
	}
	return nil
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	return GenerateToken(room, identity, s.secret)
}

func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.resolver != nil {
		return s.listRoomsFromSRS()
	}
	// 降级：resolver 未注入时保持 registry 聚合。
	if s.registry == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "srs room registry not configured")
	}
	rooms := s.registry.Rooms()
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for _, name := range rooms {
		streams := s.registry.Streams(name)
		out = append(out, sfu.RoomSummary{Name: name, MemberCount: len(streams)})
	}
	return out, nil
}

// listRoomsFromSRS 直接查 SRS /api/v1/streams/，用 resolver 反查 room 聚合。
func (s *Service) listRoomsFromSRS() ([]sfu.RoomSummary, error) {
	streams, err := s.client.ListStreams()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	rooms := make(map[string]int)
	for _, stream := range streams {
		if !strings.HasPrefix(stream, streamNamePrefix) {
			continue
		}
		room, ok := s.resolver.RoomForStream(stream)
		if !ok || room == "" {
			continue
		}
		rooms[room]++
	}
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for room, count := range rooms {
		out = append(out, sfu.RoomSummary{Name: room, MemberCount: count})
	}
	return out, nil
}

func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	if s.resolver != nil {
		return s.listParticipantsFromSRS(room)
	}
	// 降级：resolver 未注入时保持旧 registry 路径。
	var streams []string
	if s.registry != nil {
		streams = s.registry.Streams(room)
	}
	if len(streams) == 0 {
		return []sfu.ParticipantSummary{}, nil
	}
	participants, err := s.client.ListParticipantsByStreams(streams)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	out := make([]sfu.ParticipantSummary, 0, len(participants))
	seen := make(map[string]struct{}, len(participants))
	for _, p := range participants {
		stream, _ := p["stream"].(string)
		identity := ""
		if s.registry != nil && stream != "" {
			if id, ok := s.registry.IdentityForStream(room, stream); ok {
				identity = id
			}
		}
		if identity == "" {
			if id, ok := p["id"].(string); ok {
				identity = id
			}
		}
		if identity == "" {
			continue
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		out = append(out, sfu.ParticipantSummary{Identity: identity})
	}
	return out, nil
}

// listParticipantsFromSRS 从 /api/v1/streams/ 拉全部活跃推流（不含播放连接），
// 用 resolver 反查 room 过滤，identity 经 registry 解析；解析不到的流跳过，
// 避免 SRS 残留/内部流污染统计。
func (s *Service) listParticipantsFromSRS(room string) ([]sfu.ParticipantSummary, error) {
	streams, err := s.client.ListStreams()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	seen := make(map[string]struct{}, len(streams))
	out := make([]sfu.ParticipantSummary, 0, len(streams))
	for _, stream := range streams {
		if !strings.HasPrefix(stream, streamNamePrefix) {
			continue
		}
		candidate, ok := s.resolver.RoomForStream(stream)
		if !ok || !roomMatches(candidate, room) {
			continue
		}
		identity := ""
		if s.registry != nil {
			if id, ok := s.registry.IdentityForStream(candidate, stream); ok {
				identity = id
			}
		}
		if identity == "" {
			continue // 无法识别的流不计入
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		out = append(out, sfu.ParticipantSummary{Identity: identity})
	}
	return out, nil
}

// roomMatches 支持复合键（domainUUID:roomName）精确匹配与纯逻辑名后缀匹配。
func roomMatches(candidate, room string) bool {
	if candidate == room {
		return true
	}
	if !strings.Contains(room, ":") {
		if i := strings.LastIndex(candidate, ":"); i >= 0 && candidate[i+1:] == room {
			return true
		}
	}
	return false
}

const publishBlockRuleID = 1

// PublishBlockKey 是 SRS 禁推黑名单的 MuteRuleStore key（provider 与 callback handler 共用）。
func PublishBlockKey(stream string) string {
	return "srs_pub_block:" + stream
}

func (s *Service) ruleStore() sfu.MuteRuleStore {
	if s.muteRules != nil {
		return s.muteRules
	}
	return sfu.NewMemoryMuteRuleStore()
}

// MuteParticipantTimed 实现 sfu.TimedMuteProvider（Discord 式，不踢流）：
// muted=true 写禁推黑名单——当前推流保留（订阅端静音由 member:muted 事件驱动，成员仍能听），
// 一旦断流/重连，SRS on_publish 回调会拒绝该 stream 重新发布；
// muted=false 移除黑名单，允许重新发布。ttlSeconds>0 时黑名单带 TTL。
func (s *Service) MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error {
	stream := s.resolveStream(room, identity)
	return s.applyPublishBlock(stream, muted, ttlSeconds)
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return s.MuteParticipantTimed(room, identity, trackSid, muted, 0)
}

func (s *Service) resolveStream(room, identity string) string {
	if s.registry != nil {
		if st, ok := s.registry.StreamForIdentity(room, identity); ok {
			return st
		}
	}
	return GenerateStreamName(room, identity)
}

func (s *Service) applyPublishBlock(stream string, muted bool, ttlSeconds int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if muted {
		var ttl time.Duration
		if ttlSeconds > 0 {
			ttl = time.Duration(ttlSeconds) * time.Second
		}
		return s.ruleStore().Save(ctx, PublishBlockKey(stream), publishBlockRuleID, ttl)
	}
	return s.ruleStore().Delete(ctx, PublishBlockKey(stream))
}

func (s *Service) RemoveParticipant(room, identity string) error {
	stream := ""
	if s.registry != nil {
		if st, ok := s.registry.StreamForIdentity(room, identity); ok {
			stream = st
		}
	}
	if stream == "" {
		stream = GenerateStreamName(room, identity)
	}
	kicked, remaining, err := s.client.KickByStreams([]string{stream})
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	if kicked == 0 && remaining == 0 {
		return pkg.NewAppError(pkg.NOT_FOUND, "srs participant not found")
	}
	return nil
}

func (s *Service) DeleteRoom(room string) error {
	if s.resolver != nil {
		return s.deleteRoomFromSRS(room)
	}
	// 降级：resolver 未注入时保持旧 registry 路径。
	if s.registry != nil {
		streams := s.registry.Streams(room)
		kicked, remaining, err := s.client.KickByStreams(streams)
		if err != nil {
			return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
		}
		if kicked == 0 && remaining == 0 && len(streams) == 0 {
			return pkg.NewAppError(pkg.NOT_FOUND, "srs room not found or empty")
		}
		s.registry.ClearRoom(room)
		return nil
	}
	if err := s.client.DeleteRoom(room); err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	return nil
}

// deleteRoomFromSRS 从 /api/v1/streams/ 拉全部流，用 resolver 反查属于该 room 的流，
// 通过 SRS clients API 踢掉（SRS 无删流原语，DELETE /api/v1/streams/{name} 返 2048），
// 最后清理本地聚合登记（ClearRoom 会同步清理 membership KV）。
func (s *Service) deleteRoomFromSRS(room string) error {
	streams, err := s.client.ListStreams()
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	targets := make([]string, 0, len(streams))
	for _, stream := range streams {
		if !strings.HasPrefix(stream, streamNamePrefix) {
			continue
		}
		candidate, ok := s.resolver.RoomForStream(stream)
		if !ok || !roomMatches(candidate, room) {
			continue
		}
		targets = append(targets, stream)
	}
	kicked, remaining, err := s.client.KickByStreams(targets)
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	if kicked == 0 && remaining == 0 && len(targets) == 0 {
		return pkg.NewAppError(pkg.NOT_FOUND, "srs room not found or empty")
	}
	if remaining > 0 && kicked == 0 {
		return pkg.NewAppError(pkg.SFU_ERROR, "srs delete room partial failure")
	}
	if s.registry != nil {
		s.registry.ClearRoom(room)
	}
	return nil
}

func (s *Service) GetHost() string {
	if s.publicHost != "" {
		return s.publicHost
	}
	return s.host
}

func (s *Service) ProviderName() string {
	return "srs"
}

func (s *Service) Capabilities() sfu.Capabilities {
	return sfu.CapabilitiesFor("srs")
}

func (s *Service) StreamName(room, identity string) string {
	return GenerateStreamName(room, identity)
}

func (s *Service) StreamInfo(room, identity string) (stream, token string, err error) {
	stream = GenerateStreamName(room, identity)
	t, err := GenerateStreamToken(stream, s.secret)
	if err != nil {
		return stream, "", pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, "generate stream token: "+err.Error())
	}
	return stream, t, nil
}

func (s *Service) ClientInfo() map[string]interface{} {
	return map[string]interface{}{
		"whipUrl": s.whipURL,
	}
}
