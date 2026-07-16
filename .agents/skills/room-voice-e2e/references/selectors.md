# GOSpeak UI Selectors (Room Voice E2E)

当前前端几乎没有 `data-testid`。Playwright 优先用角色/文案，computer-use 优先用可见文本。

## Login `/login`

| 元素 | 选择器 / 文案 |
|------|----------------|
| 用户名 | placeholder `请输入用户名` |
| 密码 | placeholder `请输入密码` |
| 登录按钮 | role=button name=`登录` |
| 强制改密 | 文案含 `修改密码` / `首次登录`（e2e 账号应避开默认 admin 首登） |

## Channel 页 `/channel`

登录后 e2e 应进入 `/channel` 才有左侧房间列表。

## Room List 左侧列表

| 元素 | 选择器 / 文案 |
|------|----------------|
| 列表标题 | `服务器` |
| 刷新 | button `刷新` |
| 房间行 | 含房间名的列表项；**双击**进入 |
| 创建入口 | `button[title="新建房间"]`（图标 +） |
| 人数 | 房间行内 `count/limit`，如 `1/20` |
| 密码房提示 | title/tooltip `双击输入密码加入` |
| 无密码房提示 | title/tooltip `双击进入房间` |
| 空态 | `暂无房间` |

## Create Room Modal

| 元素 | 选择器 / 文案 |
|------|----------------|
| 房间名称 | label `房间名称` / placeholder `例如：产品评审会` |
| 房间密码 | label `房间密码` / placeholder `选填，留空表示公开房间` |
| 人数上限 | label `人数上限`（默认 12） |
| 创建后进入频道页 | label `创建后进入频道页`（默认 true；e2e 常关掉以便列表双击进房） |
| 弹窗标题 | heading `新建房间` |
| 提交 | role=button name=`创建房间` |
| 成功 toast | `已创建房间: {name}` |

## Room Detail 已加入态

| 元素 | 选择器 / 文案 |
|------|----------------|
| 房间标题 | `.font-bold.truncate` = 当前房间名 |
| 在线人数 | `{\d+} 人在线` |
| 离开 | role=button name=`离开` |
| 失败重试 | role=button name=`重试` + error 文案 |
| 等待成员 | `已连接，等待成员加入` |

## Loading / Phase Labels

来自 `voicePhaseLabel()`：

- `准备加入...`
- `加载语音引擎...`
- `连接媒体...`
- `媒体已连接`
- `加入房间...`
- `正在重连...`
- `正在离开...`
- `加入失败`
- `已连接`

**加入成功判定（推荐）**：

1. `离开` 按钮可见
2. 标题出现目标房间名
3. 不出现 `重试`
4. media probe：`getUserMediaCalls > 0` 或存在 live audio track / RTCPeerConnection

## Voice Chat 成员卡

| 元素 | 说明 |
|------|------|
| 成员卡片网格 | 房间详情主区域 |
| 本人优先排序 | 列表第一项通常是自己 |
| 远端音频 DOM | `document.querySelectorAll('audio,video')` 中带 `srcObject` 且含 audio track 的节点（由 `handler_audio` attach） |

## Computer-use 额外提示

- 双击房间行，不要单击。
- 切房时直接双击目标房间，无需先点离开（前端 session 会 teardown 再 join）。
- 快速切房时观察是否卡在 `连接媒体...` / `加入失败` / 出现多个 orphan loading。
- 多人测试用两个浏览器窗口/两个 Playwright context，不要复用同一登录态 cookie。
