# Media Assertions

## 为什么不用“真的听见声音”

自动化环境默认用 Chromium fake media device，重点验证：

1. 采集是否发生
2. 发布会话是否建立
3. 远端 track 是否订阅并 attach 到 DOM
4. 切房后旧会话是否释放、新会话是否重建

真实听感只在 computer-use 人工辅助或有物理声卡回路时补充。

## Probe 注入

脚本 `scripts/media-probe.mjs` 在每个 page 注入 `window.__gospeakMediaProbe`：

- hook `navigator.mediaDevices.getUserMedia`
- wrap `RTCPeerConnection` 记录 connection/ice 状态与 sender/receiver 数
- 扫描 DOM 中带 `srcObject` 的 audio/video 作为 remote playback 证据
- 读取房间 UI：离开按钮、人数、phase 文案

## 推荐断言

### Publish / 本地推流

```js
const snap = await page.evaluate(() => window.__gospeakMediaProbe.getSnapshot());
const published =
  snap.getUserMediaCalls > 0 ||
  snap.localTracks.some((t) => t.kind === "audio" && t.readyState === "live");
const sessioned = snap.peerConnections.some((pc) =>
  ["connecting", "connected", "checking", "completed"].includes(pc.iceConnectionState) ||
  ["connecting", "connected"].includes(pc.connectionState)
);
expect(published && (sessioned || snap.hasLeaveButton)).toBeTruthy();
```

### Pull / 远端拉流（多人）

```js
const remote = await page.evaluate(() =>
  window.__gospeakMediaProbe.waitForRemoteAudio(1, 25000)
);
// remote.ok === true
// remote.remote[i].liveTracks >= 1
```

对应前端路径：

- SFU client `onRemoteAudioTrack`
- `handler_audio` `track.attach()` 后把 `<audio>` 挂到 `document.body`

### 切房健康

每次 joined 后：

- `hasLeaveButton === true`
- `currentRoomText` 为目标房
- 不应长期 `phaseText === '加入失败'`
- rapid switch 全程无 retry

## Provider 差异

| Provider | 注意 |
|----------|------|
| LiveKit | PC/track 语义最标准 |
| SRS | WHIP publish + WHEP subscribe；`media_ready` 可能早于 signal 完成 |
| Agora | SDK 内部封装，PC hook 可能不完整，更多依赖 UI joined + remote audio DOM |
| MediaSoup | 依赖 socket 自定义信令；确保 socket 已连接 |
| Cloudflare | WHIP/WHEP 会话；关注 tracks/new 与 remote attach |

若某 provider 下 `RTCPeerConnection` wrap 抓不到实例，降级断言：

1. joined UI
2. getUserMedia 调用
3. multi-user remote audio DOM
4. 网络面板 token/WHIP/WHEP 2xx（computer-use 或 Playwright request 日志）

## 常见失败分类

| 现象 | 更可能层 |
|------|----------|
| 登录后无房间列表 | API/WS 连接 |
| 双击后永久 loading | token / SFU client load |
| 加入失败可重试 | media join / ICE / provider config |
| 双方成员可见但无声 | subscribe / attach / autoplay |
| 快切后卡死 | join abort/teardown 竞态 |
| 只一方有远端轨 | 单向 publish 或 peer subscribe 过滤 |

## 手工 computer-use 补充检查

当 Playwright fake media 不足以证明听感时：

1. 系统设置允许浏览器麦克风
2. 两窗口分别登录
3. 一窗口对麦克风发声，另一窗口看 speaking 指示/听筒输出
4. 记录是否单向/双向、延迟是否异常升高
