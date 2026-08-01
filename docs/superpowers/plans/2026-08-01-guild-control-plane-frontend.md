# Guild 控制面前端闭环实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按 `docs/cluster-agent-worker-plan.md` 的 Phase 0、Phase 0.5 要求，补齐 Guild 管理、公开发现、邀请确认和权限状态的前端闭环，并为后续 Agent/Worker 路由保留清晰的 API 边界。

**Architecture:** 本计划只覆盖最终用户可见的 Guild 控制面前端，不在当前缺少后端契约的情况下臆造节点或副本接口。Guild 页面通过 `api/guild.ts` 访问 Agent 业务 API，`guildStore` 负责缓存、当前 Guild、成员和加入状态；管理页、发现页、邀请页只消费这些边界。Phase 3 的 `workerUrl` 路由和 Phase 5 的节点/实例组管理作为后续独立计划，待后端集群 API 落地后接入。

**Tech Stack:** SolidJS、TypeScript、TanStack Solid Router、Vitest、现有 `apiClient`、现有 `guildStore`/`userStore`、Tailwind/DaisyUI。

---

## 现状与范围边界

### 已存在、不得重复实现

- `app/web/src/api/guild.ts` 已有 Guild CRUD、公开列表、邀请码预览、加入/离开、成员列表和踢人 API。
- `app/web/src/pages/(app)/discover/index.tsx` 已有公开列表、关键词搜索、分页、邀请码/链接输入、剪贴板读取和加入预览初版。
- `app/web/src/pages/(app)/invite/g/$code/index.tsx` 已有邀请预览与确认加入初版。
- `app/web/src/components/guild/InviteShareModal.tsx` 已有二维码和复制邀请链接能力。
- `app/web/src/pages/(app)/guild/$guildUUID/manage.tsx` 已有基础设置保存、成员刷新和踢人初版。
- `app/web/src/socket/roomState.ts`、`app/web/src/stores/socketStore.ts` 已开始按 `guild_uuid` 过滤房间；本计划只补齐测试和页面状态，不重写 Socket 层。

### 本计划要解决的问题

- 管理页权限判断目前把全局权限、Guild 成员角色和资源归属混在一起，普通用户可能看到不应操作的控件。
- 管理页没有完整表达加载、刷新、保存、踢人、失败和重试状态，表格操作列缺少稳定的功能闭环。
- Guild 缓存的加入状态、当前 Guild 和删除/离开后的清理不够集中，发现页与邀请页可能依赖旧缓存。
- 发现/邀请页面需要统一“预览 → 已加入判断 → 确认加入 → 缓存更新 → 跳转”的流程，并处理剪贴板权限失败、重复提交和登录回跳契约。
- 当前权限表未完整覆盖 Guild 相关默认角色，需与后端 `DefaultRolePermissions` 保持一致。

### 明确不在本计划内

- 不修改后端 `internal/cluster`、Agent、Worker、NATS 或数据库模型。
- 不实现 `workerUrl` 获取、按房间连接 Worker、节点状态、Server 实例组、扩缩容和监控页面；这些需要后端 Phase 2/3/5 契约，另建计划。
- 不引入房间邀请链接 `invite_links`。
- 不做无关的视觉重构；布局调整只为保证文本、表格、操作按钮和窄屏交互不影响功能。
- 不覆盖当前工作区已有未提交改动；实现时必须基于当前工作树继续，并先确认目标文件没有新的用户修改。

## 文件边界

### 修改

- `app/web/src/api/guild.ts`：补齐明确的响应类型、加入状态所需的数据边界和统一错误类型；不在页面内拼接 API 请求。
- `app/web/src/api/guildApi.spec.ts`：为新增/调整的 Guild API 请求参数和响应解包增加单元测试。
- `app/web/src/stores/guildStore.ts`：集中维护 Guild、成员、我的 Guild UUID、当前 Guild、加载/错误状态，并提供更新/清理动作。
- `app/web/src/stores/guildStore.spec.ts`：测试缓存、加入、离开、删除、成员加载失败和当前 Guild 清理。
- `app/web/src/utils/permissions.ts`：同步 Guild 默认权限，并提供资源级判断所需的纯函数或明确调用边界。
- `app/web/src/utils/permissions.test.ts`：覆盖 Guild 管理、踢人、邀请、删除权限。
- `app/web/src/pages/(app)/guild/$guildUUID/manage.tsx`：实现设置与成员管理的状态闭环、权限分层、操作反馈和可恢复失败状态。
- `app/web/src/pages/(app)/discover/index.tsx`：统一公开发现、邀请码预览、剪贴板识别、加入确认和已加入跳转状态。
- `app/web/src/pages/(app)/invite/g/$code/index.tsx`：统一邀请页的登录回跳、加载/错误/加入状态和缓存更新。
- `app/web/src/pages/(app)/guild/$guildUUID/index.tsx`：统一复制邀请码、分享邀请、离开/删除后的缓存清理和管理入口权限。
- `app/web/src/layouts/common/sidebar.tsx`：只保留发现服务器入口，避免旧的加入服务器入口与新流程并存；保证加载失败时不阻断主布局。
- `app/web/src/utils/guildInvite.ts`：收敛邀请码/邀请链接解析规则，覆盖当前域名、路径、查询参数和非法输入。
- `app/web/src/utils/guildInvite.test.ts`：补齐邀请输入解析测试。

### 新增

- `app/web/src/components/guild/GuildInvitePreview.tsx`：提取发现页和邀请页共用的预览/确认受控组件；组件只接收 Guild、是否已加入、加载状态和回调，不调用 API。
- `app/web/src/components/guild/GuildInvitePreview.spec.tsx`：使用 Vitest + jsdom 验证“未加入显示确认加入、已加入显示进入服务器、加载/错误状态不可重复提交”。
- `app/web/src/components/guild/GuildMemberTable.tsx`：提取管理页成员表格，负责成员展示、角色、操作列和空/加载/错误状态；不在组件内调用 API。
- `app/web/src/components/guild/GuildMemberTable.spec.tsx`：验证拥有者不能被踢出、无权限时不渲染踢人按钮、目标成员操作回调携带正确 UUID。

### 不修改

- `app/web/src/routeTree.gen.ts`：路由生成文件由 TanStack Router 插件更新；只有新增路由时才运行既有生成命令并纳入变更。
- `app/web/src/socket/wsClient.ts`、`app/web/src/socket/events.ts`：本阶段不改变 WebSocket 协议。

## 实施任务

### Task 1: 固化 Guild API 与领域类型

**Files:**
- Modify: `app/web/src/api/guild.ts`
- Test: `app/web/src/api/guildApi.spec.ts`

- [ ] **Step 1: 先为请求边界写测试**

在 `guildApi.spec.ts` 增加以下行为测试，沿用现有 `apiClient.post` mock 和 `mockResult` helper。若这些行为已经通过，则作为回归测试保留，不以“必须先失败”作为本步骤通过条件：

```ts
it("listPublicGuilds trims keyword and preserves pagination", async () => {
  (apiClient.post as any).mockResolvedValue(
    mockResult({ guilds: [], total: 0 }),
  );

  await listPublicGuilds(2, 12, "  alpha  ");

  expect(apiClient.post).toHaveBeenCalledWith({
    url: "/api/v1/guild/list-public",
    data: { page: 2, page_size: 12, keyword: "alpha" },
  });
});

it("previewGuildInvite sends the normalized invite code", async () => {
  (apiClient.post as any).mockResolvedValue(mockResult({ uuid: "g-1" }));

  await previewGuildInvite("ABCDEFGH");

  expect(apiClient.post).toHaveBeenCalledWith({
    url: "/api/v1/guild/preview",
    data: { invite_code: "ABCDEFGH" },
  });
});
```

运行：`pnpm --dir app/web test -- guildApi.spec.ts`。

预期：测试运行成功；若测试立即通过，则确认行为已覆盖并继续下一步。

- [ ] **Step 2: 明确 API 类型和错误边界**

在 `guild.ts` 中保留现有函数名和 endpoint，补充可复用类型：

```ts
export interface GuildPage {
  guilds: Guild[];
  total: number;
}

export interface GuildApiError {
  code?: number;
  msg?: string;
}

export type GuildMutation =
  | "create"
  | "update"
  | "delete"
  | "join"
  | "leave"
  | "kick";
```

`listGuilds` 和 `listPublicGuilds` 返回 `GuildPage`；所有列表请求在函数边界处理默认页码和 `keyword.trim()`；页面不得直接依赖 Axios 响应结构。

- [ ] **Step 3: 运行 API 单测并确认通过**

运行：`pnpm --dir app/web test -- guildApi.spec.ts`。

预期：API 请求路径、请求体和响应解包测试全部通过。

### Task 2: 集中 Guild 缓存和权限判断

**Files:**
- Modify: `app/web/src/stores/guildStore.ts`
- Modify: `app/web/src/utils/permissions.ts`
- Test: `app/web/src/stores/guildStore.spec.ts`
- Test: `app/web/src/utils/permissions.test.ts`

- [ ] **Step 1: 先写缓存清理和加入状态测试**

为 store 增加可测试的动作边界，测试以下不变量：

```ts
it("removes a guild from ids and caches after leave or delete", async () => {
  const store = createTestGuildStore();
  const guild = makeGuild({ uuid: "g-1" });

  store.addGuild(guild);
  store.setCurrentGuild("g-1");
  store.removeGuild("g-1");

  expect(store.state.myGuildUUIDs).not.toContain("g-1");
  expect(store.state.guildCache["g-1"]).toBeUndefined();
  expect(store.state.memberCache["g-1"]).toBeUndefined();
  expect(store.state.currentGuildUUID).toBeNull();
});

it("keeps a failed member refresh retryable", async () => {
  guildMembersMock.mockRejectedValueOnce(new Error("network"));

  await expect(store.loadMembers("g-1")).rejects.toThrow("network");
  expect(store.state.memberLoading["g-1"]).toBe(false);
  expect(store.state.memberErrors["g-1"]).toBe("network");
});
```

如果当前 `createRoot` 导出方式难以直接测试，先将工厂函数导出为 `createGuildStore`，默认导出仍使用 `createRoot(createGuildStore)`；测试只调用工厂，不改变运行时单例。

- [ ] **Step 2: 实现明确的 Guild 状态字段和动作**

`GuildState` 至少增加以下字段：

```ts
interface GuildState {
  myGuildUUIDs: string[];
  currentGuildUUID: string | null;
  guildCache: Record<string, Guild>;
  memberCache: Record<string, GuildMember[]>;
  guildLoading: Record<string, boolean>;
  memberLoading: Record<string, boolean>;
  guildErrors: Record<string, string | null>;
  memberErrors: Record<string, string | null>;
  loading: boolean;
}
```

实现 `addGuild`、`updateCachedGuild`、`removeGuild`、`setCurrentGuild`、`loadMembers` 时保证：

- 加入后 UUID、Guild 缓存和当前 Guild 可立即被页面读取。
- 离开或删除后同时清理 Guild、成员、错误和加载状态。
- 刷新失败保留旧缓存，记录错误，允许再次调用 `loadMembers`。
- 任何异步加载在 `finally` 中恢复 loading 状态。

- [ ] **Step 3: 同步 Guild 默认权限并补纯函数测试**

确保 `rolePermissions` 至少与计划要求的权限码一致：`guild:create`、`guild:read`、`guild:manage`、`guild:delete`、`guild:invite`、`guild:kick`、`guild:role:manage`。管理页不能只用全局角色权限判断资源操作；优先使用“当前用户是 owner”或“当前成员角色/权限满足要求”的组合。

为权限测试增加：

```ts
it("keeps guild management permissions in the admin fallback", () => {
  expect(rolePermissions.admin).toEqual(
    expect.arrayContaining([
      "guild:manage",
      "guild:delete",
      "guild:invite",
      "guild:kick",
    ]),
  );
});
```

- [ ] **Step 4: 运行状态与权限测试**

运行：`pnpm --dir app/web test -- guildStore.spec.ts permissions.test.ts`。

预期：缓存清理、失败可重试和 Guild 权限测试通过。

### Task 3: 完成 Guild 管理页的功能闭环

**Files:**
- Modify: `app/web/src/pages/(app)/guild/$guildUUID/manage.tsx`
- Create: `app/web/src/components/guild/GuildMemberTable.tsx`
- Create: `app/web/src/components/guild/GuildMemberTable.spec.tsx`

- [ ] **Step 1: 先写管理页状态验收测试**

`GuildMemberTable.spec.tsx` 以组件挂载方式覆盖以下场景；管理页保存流程若不便于组件测试，则将相同断言放进可复用纯函数或 store 测试：

```tsx
import { render } from "solid-testing-library";
import { describe, expect, it, vi } from "vitest";
import { GuildMemberTable } from "./GuildMemberTable";

const owner = { id: 1, user_uuid: "u-owner", nickname: "owner", role_name: "owner", joined_at: "2026-01-01" };
const member = { id: 2, user_uuid: "u-member", nickname: "member", role_name: "member", joined_at: "2026-01-01" };

describe("GuildMemberTable", () => {
  it("does not render kick action for the owner row", async () => {
    const onKick = vi.fn();
    const { queryByLabelText } = render(() => (
      <GuildMemberTable
        members={[owner, member]}
        ownerUUID="u-owner"
        currentUserUUID="u-member"
        canKick
        onKick={onKick}
        loading={false}
        error={null}
      />
    ));
    expect(queryByLabelText("踢出 u-owner")).toBeNull();
  });

  it("calls onKick with the target UUID", async () => {
    const onKick = vi.fn();
    const { getByLabelText } = render(() => (
      <GuildMemberTable
        members={[owner, member]}
        ownerUUID="u-owner"
        currentUserUUID="u-owner"
        canKick
        onKick={onKick}
        loading={false}
        error={null}
      />
    ));
    getByLabelText("踢出 u-member").click();
    expect(onKick).toHaveBeenCalledWith("u-member");
  });

  it("hides all kick actions when canKick is false", async () => {
    const { queryByLabelText } = render(() => (
      <GuildMemberTable
        members={[member]}
        ownerUUID="u-owner"
        currentUserUUID="u-member"
        canKick={false}
        onKick={vi.fn()}
        loading={false}
        error={null}
      />
    ));
    expect(queryByLabelText("踢出 u-member")).toBeNull();
  });
});
```

说明：`solid-testing-library` 需先确认是否已在 `devDependencies`；若未安装，优先用 `@solidjs/testing-library`，或把测试收敛到组件导出的纯状态函数。测试必须断言 API 调用、按钮 disabled 状态和错误状态，不以截图或 CSS class 作为唯一断言。

- [ ] **Step 2: 分离资源加载状态和表单状态**

管理页使用 `createResource` 或 store 状态分别表达：Guild 加载、成员加载、保存中、踢人中。初始加载展示明确占位；Guild 加载失败显示错误和重试；成员加载失败保留表头并提供刷新/重试；保存失败保留用户输入。

表单字段规则：

- `name.trim()` 为空时阻止提交。
- `max_rooms` 必须是整数且 `>= 1`，输入非法时关联错误文案。
- 保存期间禁用表单提交，按钮文案为进行中状态，避免重复请求。
- 成功后以服务端返回 Guild 更新缓存，并同步页面标题、公开状态和列表缓存。
- 未拥有 Guild 管理权限时字段只读或不渲染保存按钮，不能只依赖点击后 toast。

- [ ] **Step 3: 完成成员表格操作列**

新建 `GuildMemberTable` 并接入管理页，表格必须实现：

- 显示加载、空数据、加载失败和刷新中状态。
- 显示用户标识、昵称回退、角色、加入时间。
- 当前用户不能踢出自己。
- Guild owner 不能被踢出。
- 无 `guild:kick`/管理员/owner 权限时不渲染踢人操作。
- 点击踢人先打开 `ConfirmModal`，确认后只对目标 UUID 调用 `kickGuildMember(guildUUID, userUUID)`。
- 成功后刷新成员缓存；失败时保留表格内容并展示可重试错误。

- [ ] **Step 4: 补齐删除和离开后的页面状态**

管理页返回 Guild 详情页；Guild 详情页的离开/删除动作必须使用统一的确认组件或明确的确认流程，成功后调用 `guildStore.removeGuild(uuid)` 再跳转首页。不能只导航而留下旧 Guild UUID、成员缓存或侧边栏条目。

- [ ] **Step 5: 运行针对性检查**

运行：`pnpm --dir app/web test -- guildStore.spec.ts permissions.test.ts guildApi.spec.ts GuildMemberTable.spec.tsx`，再运行：`pnpm --dir app/web build`。

预期：相关单测通过，TypeScript 和 Vite 构建通过；若构建暴露当前未提交改动造成的无关错误，不在本任务中修复，记录错误文件和原因。

### Task 4: 统一发现页与邀请页流程

**Files:**
- Modify: `app/web/src/pages/(app)/discover/index.tsx`
- Modify: `app/web/src/pages/(app)/invite/g/$code/index.tsx`
- Create: `app/web/src/components/guild/GuildInvitePreview.tsx`
- Create: `app/web/src/components/guild/GuildInvitePreview.spec.tsx`

- [ ] **Step 1: 先固定邀请解析和流程状态测试**

在 `guildInvite.test.ts` 补充：

```ts
it("accepts the invite path without relying on a host", () => {
  expect(extractGuildInviteCode("/invite/g/ABCDEFGH")).toBe("ABCDEFGH");
});

it("rejects malformed and overlong codes", () => {
  expect(extractGuildInviteCode("ABC")).toBeNull();
  expect(extractGuildInviteCode("ABCDEFGH1")).toBeNull();
  expect(extractGuildInviteCode("ABCD-1234")).toBeNull();
});
```

`GuildInvitePreview` 组件测试必须覆盖“加载中、错误、未加入确认、已加入进入、加入中禁用”。

- [ ] **Step 2: 统一预览到加入的数据流**

发现页和邀请页都按以下顺序执行：

```ts
const code = extractGuildInviteCode(input);
if (!code) showInputError();
const guild = await previewGuildInvite(code);
const alreadyJoined = guildStore.state.myGuildUUIDs.includes(guild.uuid);
if (!alreadyJoined) await joinGuild(code);
guildStore.addGuild(joinedGuild);
guildStore.setCurrentGuild(joinedGuild.uuid);
navigate({ to: "/guild/$guildUUID", params: { guildUUID: joinedGuild.uuid } });
```

要求：预览成功后才允许确认加入；加入请求期间按钮禁用；重复点击不产生第二个请求；已加入 Guild 直接显示“进入服务器”；加入失败保留预览信息和输入，不清空用户上下文。

- [ ] **Step 3: 完成公开发现页验收**

发现页必须：

- 只消费 `listPublicGuilds` 返回的公开 Guild。
- 搜索提交时重置页码，翻页时保留关键词。
- 首次加载、刷新、空结果、请求失败和重试均有独立状态。
- “新建服务器”保留为创建入口；旧“加入服务器”入口不再重复存在，统一使用邀请码输入和邀请预览。
- 已加入 Guild 标记为已加入并可直接跳转。
- 页面加载时尝试读取剪贴板；浏览器拒绝权限时静默失败，不影响手动输入。

- [ ] **Step 4: 完成邀请页登录回跳契约**

现有登录回跳机制已由 `app/web/src/pages/(app)/route.tsx` 在未登录时写入 `sessionStorage.gospeak_redirect`，并由 `app/web/src/pages/login/index.tsx` 的 `resolveLoginRedirect()` 消费。本任务只需验证并补齐缺口：邀请页路径和查询参数被原样保存；登录成功和 OAuth 回跳都回到原邀请页；`resolveLoginRedirect()` 只接受以 `/` 开头的内部路径；非法 code 不进入 API 请求。

- [ ] **Step 5: 运行邀请相关测试和构建**

运行：`pnpm --dir app/web test -- guildInvite.test.ts guildApi.spec.ts GuildInvitePreview.spec.tsx`，再运行：`pnpm --dir app/web build`。

预期：邀请码解析、公开列表、预览、加入和路由类型检查通过。

### Task 5: 完善 Guild 详情页、侧边栏和跨页缓存同步

**Files:**
- Modify: `app/web/src/pages/(app)/guild/$guildUUID/index.tsx`
- Modify: `app/web/src/layouts/common/sidebar.tsx`
- Modify: `app/web/src/stores/guildStore.ts`
- Test: `app/web/src/stores/guildStore.spec.ts`

- [ ] **Step 1: 写离开/删除缓存同步测试**

覆盖以下结果：离开成功后当前 Guild、成员缓存、侧边栏 UUID 被清除；删除成功后同样清除；API 失败时不清除缓存且页面保留错误提示。

- [ ] **Step 2: 统一详情页操作权限和确认**

详情页只为 owner 展示删除，为非 owner 且已加入用户展示离开；分享邀请仅在 Guild 返回有效 `invite_code` 时启用。复制邀请码、复制完整邀请链接、生成二维码都必须捕获剪贴板或 QR 失败并给出可理解反馈。

- [ ] **Step 3: 统一侧边栏状态**

侧边栏加载我的 Guild 时不因单个 Guild 请求失败而阻断其他 Guild；当前 Guild 切换前更新 store，导航失败时保持当前可用状态；发现入口只有一个，避免旧加入入口和新发现页分叉。

- [ ] **Step 4: 运行跨页回归**

运行：`pnpm --dir app/web test -- guildStore.spec.ts guildInvite.test.ts`，再运行：`pnpm --dir app/web check`。

预期：缓存和邀请回归测试通过，Biome 检查只报告本次改动可修复的问题；不改动无关文件。

### Task 6: 端到端验收 Guild 控制面前端

**Files:**
- Test/Logs: `agent_test_logs/`（仅写入已有测试日志，不新增业务文件）
- Verify: `app/web/src/pages/(app)/guild/$guildUUID/manage.tsx`
- Verify: `app/web/src/pages/(app)/discover/index.tsx`
- Verify: `app/web/src/pages/(app)/invite/g/$code/index.tsx`

- [ ] **Step 1: 启动现有开发服务并确认入口**

使用项目既有启动方式运行前端和后端，确认以下页面可打开：

- `/discover`
- `/invite/g/:code`
- `/guild/:guildUUID`
- `/guild/:guildUUID/manage`

- [ ] **Step 2: 验证公开发现与邀请流程**

使用一个公开 Guild 和一个私有 Guild 验证：私有 Guild 不出现在公开列表；公开 Guild 可搜索；邀请码/完整邀请链接/剪贴板内容均可预览；加入前展示 Guild 信息；确认加入后缓存更新并进入 Guild；已加入 Guild 显示进入而不是重复加入。

- [ ] **Step 3: 验证管理页权限和状态**

至少验证 owner、Guild admin、普通成员三种身份：owner 可保存和管理成员；有 Guild 管理权限但非 owner 的用户按权限可编辑；普通成员不能看到保存/踢人操作；加载失败、保存失败、踢人失败均保留页面上下文并可重试。

- [ ] **Step 4: 验证破坏性操作和缓存清理**

确认离开/删除均有二次确认；成功后不再显示旧 Guild 侧边栏条目、成员缓存和详情内容；失败不丢失当前页面状态。

- [ ] **Step 5: 验证当前已知边界**

记录并明确未验收内容：`workerUrl` 路由、节点列表、Server 副本、扩缩容、节点心跳和 SFU 集群健康指标。它们必须进入后续独立的 Cluster 前端计划，不得以当前 Guild 页面中的静态字段冒充已实现能力。

## 计划自审

### 需求覆盖

- Phase 0 Guild 基础闭环：Task 1、Task 2、Task 3、Task 5、Task 6。
- Phase 0.5 公开发现：Task 1、Task 4、Task 5、Task 6。
- Phase 0.5 邀请预览、确认、二维码、剪贴板、登录回跳：Task 4、Task 5、Task 6；`InviteShareModal` 已有基础能力，仅在二维码/复制失败处理缺失时补充，不重写。
- Phase 3 前端 `workerUrl` 路由：明确留待后端契约完成后的独立计划，不在本计划中猜测字段。
- Phase 5 节点/实例组管理：明确留待 `internal/cluster` API 完成后的独立计划。

### 一致性检查

- API 统一使用 `Guild`、`GuildMember`、`GuildPage`、`GuildApiError`；页面不直接读取 Axios 响应。
- store 的清理动作统一命名为 `removeGuild`，同时清理 `guildCache`、`memberCache`、错误和 loading 状态。
- 邀请 API 统一使用 `previewGuildInvite(code)` 与 `joinGuild(code)`；页面不重复拼接 endpoint。
- 权限码使用计划中已有的 `guild:*` 命名，不引入第二套命名。
- Socket 本阶段只验证并保持 `guild_uuid` 过滤，不新增未定义的 `workerUrl` 类型。

### 未发现的占位或歧义

- 计划没有使用 `TBD`、`TODO` 或“稍后补充”作为实施步骤。
- 组件文件决策已固定：`GuildMemberTable`、`GuildInvitePreview` 及其测试均为确定新增，不存在“若需要再创建”的条件分支。
- 登录回跳描述与现有 `sessionStorage.gospeak_redirect` 实现一致，不再要求重复实现已有机制。
- 对后端尚不存在的集群 API 明确给出范围边界，而不是把未知接口写入前端实现。
- 当前仓库已有大量未提交修改；执行前必须逐文件确认差异归属，不能重置或覆盖这些修改。

## 执行交接

本计划不自动提交 Git 变更；当前工作区已有用户未提交修改，提交由用户后续明确决定。

**Plan complete and saved to `docs/superpowers/plans/2026-08-01-guild-control-plane-frontend.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - 使用 `superpowers:subagent-driven-development`，每个任务派遣独立子代理，主线程逐项复核。

**2. Inline Execution** - 使用 `superpowers:executing-plans`，在当前会话按任务批量执行并设置检查点。

请选择执行方式。
