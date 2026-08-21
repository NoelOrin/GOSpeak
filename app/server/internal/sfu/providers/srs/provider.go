package srs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

// errDeleteRoomPartial 标记删房期间的媒体层部分失败或 TOCTOU 残留：
// registry 不能清理，也不应回退到 registry 分支重试（重试可能误清新流登记）。
var errDeleteRoomPartial = errors.New("srs delete room partial failure")

type Service struct {
	client      *Client
	secret      string
	host        string
	publicHost  string
	whipURL     string
	mu          sync.RWMutex
	registry    pkg.RoomRegistry
	resolver    pkg.StreamRoomResolver
	muteMu      sync.Mutex
	muteRules   sfu.MuteRuleStore
	cacheMu     sync.Mutex
	streamCache map[string]streamRoomEntry
}

type streamRoomEntry struct {
	room  string
	found bool
	at    time.Time
}

// streamRoomCacheTTL 控制 stream→room 反查缓存窗口；缓存只为收敛 N+1，允许短暂陈旧。
const streamRoomCacheTTL = 5 * time.Second

func (s *Service) SetRoomRegistry(r pkg.RoomRegistry) {
	s.mu.Lock()
	s.registry = r
	s.mu.Unlock()
}

// SetStreamRoomResolver 注入 stream→room 反查（signal.Hub 实现）。
func (s *Service) SetStreamRoomResolver(r pkg.StreamRoomResolver) {
	s.mu.Lock()
	s.resolver = r
	s.mu.Unlock()
}

// SetMuteRuleStore 注入跨实例禁推黑名单（nats KV），实现 sfu.MuteRuleStoreSetter。
// 设计上用于启动期注入；nil 输入回退到默认内存 store。即使运行期发生替换，
// muteMu 也保证替换与 withMuteStore 内的 Save/Delete/Get 互斥，不会出现
// “写入旧 store 后读取新 store”的中间状态。
func (s *Service) SetMuteRuleStore(store sfu.MuteRuleStore) {
	s.muteMu.Lock()
	defer s.muteMu.Unlock()
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
	svc := &Service{
		client:      NewClient(baseURL),
		secret:      cfg.SRSSecret,
		host:        baseURL,
		publicHost:  strings.TrimSpace(cfg.SRSPublicHost),
		whipURL:     whipURL,
		muteRules:   nil,
		streamCache: make(map[string]streamRoomEntry),
	}
	activeServices.Store(svc, struct{}{})
	return svc
}

func init() {
	// Periodically evict expired stream→room cache entries to prevent unbounded growth.
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			// We need a reference to the service to access its streamCache.
			// This is handled by the resolveStreamRoom lazy eviction on access.
			// For a global cleanup, we iterate over a package-level registry.
			cleanupStreamCaches()
		}
	}()
}

// cleanupStreamCaches removes expired entries from all known Service instances.
// In practice there is one Service per process; this is a safety net.
var activeServices sync.Map // *Service -> struct{}

func cleanupStreamCaches() {
	now := time.Now()
	activeServices.Range(func(_, v interface{}) bool {
		if svc, ok := v.(*Service); ok {
			svc.cacheMu.Lock()
			for k, entry := range svc.streamCache {
				if now.Sub(entry.at) >= streamRoomCacheTTL {
					delete(svc.streamCache, k)
				}
			}
			svc.cacheMu.Unlock()
		}
		return true
	})
}

// Close 释放 HTTP 空闲连接并从清理注册表中移除。
func (s *Service) Close() error {
	activeServices.Delete(s)
	if s.client != nil && s.client.http != nil {
		s.client.http.CloseIdleConnections()
	}
	return nil
}

func (s *Service) GenerateToken(room, identity string) (string, error) {
	return GenerateToken(room, identity, s.secret)
}

func (s *Service) ListRooms() ([]sfu.RoomSummary, error) {
	if s.resolverRef() != nil {
		rooms, err := s.listRoomsFromSRS()
		if err == nil {
			return rooms, nil
		}
		logger.Warnf("[srs] list rooms from SRS API failed, fallback to registry: %v", err)
	}
	// 降级：resolver 未注入时保持 registry 聚合。
	registry := s.registryRef()
	if registry == nil {
		return nil, pkg.NewAppError(pkg.SFU_NOT_CONFIGURED, "srs room registry not configured")
	}
	rooms := registry.Rooms()
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for _, name := range rooms {
		streams := registry.Streams(name)
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
	// room -> identity 集合，按 identity 去重后再计数（与 ListParticipants 口径一致）。
	rooms := make(map[string]map[string]struct{})
	for _, rs := range s.collectRoomStreams(streams, "") {
		identity := ""
		if registry := s.registryRef(); registry != nil {
			if id, ok := registry.IdentityForStream(rs.roomKey, rs.stream); ok {
				identity = id
			}
		}
		if identity == "" {
			identity = strings.TrimPrefix(rs.stream, streamNamePrefix)
		}
		if rooms[rs.roomKey] == nil {
			rooms[rs.roomKey] = make(map[string]struct{})
		}
		rooms[rs.roomKey][identity] = struct{}{}
	}
	out := make([]sfu.RoomSummary, 0, len(rooms))
	for room, members := range rooms {
		out = append(out, sfu.RoomSummary{Name: room, MemberCount: len(members)})
	}
	return out, nil
}

func (s *Service) ListParticipants(room string) ([]sfu.ParticipantSummary, error) {
	if s.resolverRef() != nil {
		parts, err := s.listParticipantsFromSRS(room)
		if err == nil {
			return parts, nil
		}
		logger.Warnf("[srs] list participants from SRS API failed, fallback to registry: %v", err)
	}
	// 降级：resolver 未注入时保持旧 registry 路径。
	var streams []string
	registry := s.registryRef()
	if registry != nil {
		streams = registry.Streams(room)
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
		if registry != nil && stream != "" {
			if id, ok := registry.IdentityForStream(room, stream); ok {
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
// 用 resolver 反查 room 过滤，identity 经 registry 解析；解析不到的孤儿流
// 以 stream 名兜底计入，保证活跃发布者不被静默丢失（对齐 ListLevel=hard）。
func (s *Service) listParticipantsFromSRS(room string) ([]sfu.ParticipantSummary, error) {
	streams, err := s.client.ListStreams()
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
	}
	seen := make(map[string]struct{}, len(streams))
	out := make([]sfu.ParticipantSummary, 0, len(streams))
	for _, rs := range s.collectRoomStreams(streams, room) {
		identity := ""
		if registry := s.registryRef(); registry != nil {
			if id, ok := registry.IdentityForStream(rs.roomKey, rs.stream); ok {
				identity = id
			}
		}
		if identity == "" {
			// 登记丢失的孤儿流仍计入参与者：保证 ListLevel=hard 的口径是
			// "返回真实活跃发布者"，而不是静默丢失。
			identity = strings.TrimPrefix(rs.stream, streamNamePrefix)
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		out = append(out, sfu.ParticipantSummary{Identity: identity})
	}
	return out, nil
}

// roomStream 记录属于某 room 的 SRS 业务流及其反查出的房间键。
type roomStream struct {
	stream  string
	roomKey string
}

// collectRoomStreams 从 SRS 全量流中筛选属于 room 的流（统一三处调用点的过滤逻辑）。
// room 为空时只做前缀与反查过滤，供 listRoomsFromSRS 聚合全部房间。
func (s *Service) collectRoomStreams(streams []string, room string) []roomStream {
	out := make([]roomStream, 0, len(streams))
	for _, stream := range streams {
		if !strings.HasPrefix(stream, streamNamePrefix) {
			continue
		}
		candidate, ok := s.resolveStreamRoom(stream)
		if !ok || candidate == "" {
			continue
		}
		if room != "" && !roomMatches(candidate, room) {
			continue
		}
		out = append(out, roomStream{stream: stream, roomKey: candidate})
	}
	return out
}

// resolveStreamRoom 优先读短 TTL 缓存，miss 时经 resolver 反查并回填。
func (s *Service) resolveStreamRoom(stream string) (string, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if s.streamCache == nil {
		s.streamCache = make(map[string]streamRoomEntry)
	}
	if entry, ok := s.streamCache[stream]; ok {
		if time.Since(entry.at) < streamRoomCacheTTL {
			return entry.room, entry.found
		}
		// 过期命中立即淘汰：历史唯一 stream 数不随不再查询的流持续增长。
		delete(s.streamCache, stream)
	}
	resolver := s.resolverRef()
	if resolver == nil {
		return "", false
	}
	room, ok := resolver.RoomForStream(stream)
	s.streamCache[stream] = streamRoomEntry{room: room, found: ok, at: time.Now()}
	return room, ok
}

func (s *Service) registryRef() pkg.RoomRegistry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.registry
}

func (s *Service) resolverRef() pkg.StreamRoomResolver {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resolver
}

// roomMatches 判断 stream 反查出的房间键是否属于目标房间。
// 房间键由 pkg.RoomKey 生成：域房为 "domainUUID:roomName" 复合键，平台房为裸名。
// 只做精确匹配：后缀回退会让裸名请求命中任意域的复合键，导致跨租户枚举/误踢。
func roomMatches(candidate, room string) bool {
	return candidate == room
}

// publishBlockRuleID 是 SRS 禁推黑名单的占位 ruleID：SRS 侧只有"存在/不存在"两种状态，
// 不需要真实 rule 标识；值恒为 1。
const publishBlockRuleID = 1

// PublishBlockKey 是 SRS 禁推黑名单的 MuteRuleStore key（provider 与 callback handler 共用）。
func PublishBlockKey(stream string) string {
	return "srs_pub_block:" + stream
}

// withMuteStore 在锁内取得当前黑名单 store（可能为 nil，表示未注入共享 store），
// 并在同一临界区内执行 fn（Save/Delete/Get 等）。这样一次读写不会拆成
// “先取 store、后操作”两个步骤，避免 SetMuteRuleStore 在中间换 store 后
// 写入落在旧实例、读取落在新实例（旧写新读，黑名单静默丢失）。
func (s *Service) withMuteStore(fn func(sfu.MuteRuleStore) error) error {
	s.muteMu.Lock()
	defer s.muteMu.Unlock()
	return fn(s.muteRules)
}

// ruleStore 返回当前黑名单 store（读回/测试用）。未注入时为 nil，
// 生产读写统一走 withMuteStore，此处只做原子取引用。
func (s *Service) ruleStore() sfu.MuteRuleStore {
	var store sfu.MuteRuleStore
	_ = s.withMuteStore(func(st sfu.MuteRuleStore) error {
		store = st
		return nil
	})
	return store
}

// MuteParticipantTimed 实现 sfu.TimedMuteProvider（Discord 式，不踢流）：
// muted=true 写禁推黑名单——当前推流保留（订阅端静音由 member:muted 事件驱动，成员仍能听），
// 一旦断流/重连，SRS on_publish 回调会拒绝该 stream 重新发布；
// muted=false 移除黑名单，允许重新发布。ttlSeconds>0 时黑名单带 TTL。
//
// 兼容性说明：Discord 式禁言不再踢流，媒体层拦截依赖 SRS on_publish 禁推黑名单 +
// 订阅端 member:muted 静音。旧前端若不处理 member:muted，被禁言者仍可被听到；
// 升级前端与后端需同版本部署。
func (s *Service) MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error {
	stream := s.resolveStream(room, identity)
	return s.applyPublishBlock(stream, muted, ttlSeconds)
}

func (s *Service) MuteParticipant(room, identity, trackSid string, muted bool) error {
	return s.MuteParticipantTimed(room, identity, trackSid, muted, 0)
}

func (s *Service) resolveStream(room, identity string) string {
	if registry := s.registryRef(); registry != nil {
		if st, ok := registry.StreamForIdentity(room, identity); ok {
			return st
		}
	}
	return GenerateStreamName(room, identity)
}

func (s *Service) applyPublishBlock(stream string, muted bool, ttlSeconds int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.withMuteStore(func(store sfu.MuteRuleStore) error {
		if store == nil {
			logger.WithComponent("SRS").Warnf("mute rule store not wired, skip publish block stream=%s muted=%v", stream, muted)
			return nil
		}
		if muted {
			var ttl time.Duration
			if ttlSeconds > 0 {
				ttl = time.Duration(ttlSeconds) * time.Second
			}
			return store.Save(ctx, PublishBlockKey(stream), publishBlockRuleID, ttl)
		}
		return store.Delete(ctx, PublishBlockKey(stream))
	})
}

func (s *Service) RemoveParticipant(room, identity string) error {
	stream := ""
	if registry := s.registryRef(); registry != nil {
		if st, ok := registry.StreamForIdentity(room, identity); ok {
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
	if s.resolverRef() != nil {
		err := s.deleteRoomFromSRS(room)
		if err == nil {
			return nil
		}
		if errors.Is(err, errDeleteRoomPartial) {
			return err
		}
		logger.Warnf("[srs] delete room from SRS API failed, fallback to registry: %v", err)
	}
	// 降级：resolver 未注入时保持旧 registry 路径。
	registry := s.registryRef()
	if registry != nil {
		streams := registry.Streams(room)
		kicked, remaining, err := s.client.KickByStreams(streams)
		if err != nil {
			return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, err, err.Error())
		}
		if kicked == 0 && remaining == 0 && len(streams) == 0 {
			return pkg.NewAppError(pkg.NOT_FOUND, "srs room not found or empty")
		}
		registry.ClearRoom(room)
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
	roomStreams := s.collectRoomStreams(streams, room)
	targets := make([]string, 0, len(roomStreams))
	for _, rs := range roomStreams {
		targets = append(targets, rs.stream)
	}
	if len(targets) == 0 {
		return pkg.NewAppError(pkg.NOT_FOUND, "srs room not found or empty")
	}
	kicked, remaining, err := s.client.KickByStreams(targets)
	if err != nil {
		// KickByStreams 在 remaining>0 时返回 error；把计数带进错误，避免静默丢部分失败。
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, errDeleteRoomPartial,
			fmt.Sprintf("srs delete room partial failure: kicked=%d remaining=%d: %v", kicked, remaining, err))
	}
	if kicked == 0 {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, errDeleteRoomPartial, "srs delete room: no stream kicked")
	}
	// kick 后复查：KickByStreams 与 ClearRoom 之间新加入的流不能被连带清理。
	after, listErr := s.client.ListStreams()
	if listErr != nil {
		// 复查失败无法证明 room 已清空，按部分失败处理：不清理 registry。
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, errDeleteRoomPartial,
			fmt.Sprintf("srs delete room partial failure: re-check streams after kick: %v", listErr))
	}
	if len(s.collectRoomStreams(after, room)) > 0 {
		return pkg.NewAppErrorWithCause(pkg.SFU_ERROR, errDeleteRoomPartial,
			"srs delete room partial failure: new streams joined during kick")
	}
	if registry := s.registryRef(); registry != nil {
		registry.ClearRoom(room)
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
