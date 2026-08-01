# Room List 与域详情主体 UI 优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复房间列表长名称布局，并将域管理详情主体统一为 ManageShell 风格的响应式双栏布局。

**Architecture:** 保持现有 SolidJS 状态、API、权限与事件处理不变，仅调整 RoomItem 的 flex 截断/Tooltip 标记和域详情 JSX 结构。域详情通过 `ManagePage`、`ManageHeader`、`ManageSection` 复用管理页视觉规范，桌面端设置固定列、成员自适应，窄屏上下排列。

**Tech Stack:** SolidJS · TypeScript · TanStack Solid Router · Tailwind CSS v4 · daisyUI · lucide-solid

## Global Constraints

- 不新增 API、模型、权限码或路由。
- 不改变域设置校验规则、保存请求、成员加载和移除请求。
- 不改左侧管理导航、顶部导航和其他管理页面。
- 不改变房间项选择、加入、密码弹窗和成员点击行为。
- 代码中不加非必要注释，不使用 emoji。

---

### Task 1: 修复 RoomList 长名称布局与 Tooltip

**Files:**
- Modify: `app/web/src/components/room/roomList.tsx:1-148`

**Interfaces:**
- Consumes: existing `RoomItemPropsType`, `props.room`, `handleJoin`.
- Produces: unchanged `RoomItem` behavior; button exposes full room name through `data-tip` and truncates visible name without squeezing count.

- [ ] **Step 1: Update imports and tooltip content without changing interaction handlers**

保留现有 daisyUI tooltip wrapper，将 `data-tip` 从进入提示改为完整房间信息；房间按钮 `onClick`、`onDblClick` 原样保留。

```tsx
<div
  class={clsx(
    "tooltip tooltip-right w-full min-w-0",
    isSelected() && !props.isActive ? "tooltip-open" : "",
  )}
  data-tip={`${props.room.type === "text" ? "# " : ""}${props.room.name}${props.room.hasPassword ? " · 需要密码" : ""}`}
>
```

- [ ] **Step 2: Make button children shrink-safe**

将当前按钮内部两列替换为以下结构：左侧容器 `min-w-0 flex-1`，名称 `truncate whitespace-nowrap`，密码图标 `shrink-0`，人数 `shrink-0`。

```tsx
<div class="flex min-w-0 flex-1 items-center gap-1">
  <span class="shrink-0">{/* existing microphone SVG */}</span>
  <span class="min-w-0 flex-1 truncate whitespace-nowrap text-left text-[14px] leading-0">
    {props.room.type === "text" ? "# " : ""}
    {props.room.name}
  </span>
  <Show when={props.room.hasPassword}>
    <span class="shrink-0 text-base-content/50">{/* existing lock SVG */}</span>
  </Show>
</div>
<div class="shrink-0 text-[12px]">
  {props.room.count}/{props.room.limit}
</div>
```

Keep existing SVG bodies unchanged; only move them into the shrink-safe wrappers.

- [ ] **Step 3: Run frontend checks**

Run:

```bash
pnpm --filter @go-rtc/web check
```

Expected: command exits `0` with no TypeScript/format errors.

- [ ] **Step 4: Commit RoomList change**

```bash
git add app/web/src/components/room/roomList.tsx
git commit -m "fix(web): truncate room names with tooltip"
```

---

### Task 2: Restructure domain detail主体 with ManageShell

**Files:**
- Modify: `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx:1-463`

**Interfaces:**
- Consumes: existing domain signals, `handleSave`, `requestKick`, `retryDomain`, `DomainMemberTable`, and `ConfirmModal`.
- Produces: same domain detail route and actions; rendered structure uses `ManagePage`, `ManageHeader`, and `ManageSection`.

- [ ] **Step 1: Add ManageShell imports**

Add the following import after existing component imports:

```tsx
import {
  ManageHeader,
  ManagePage,
  ManageSection,
} from "@/components/manage/ManageShell";
```

- [ ] **Step 2: Replace detail page outer wrapper and header**

Replace lines 220-240 with `ManagePage` and `ManageHeader`. Keep link target `/domain/$domainUUID` unchanged.

```tsx
<ManagePage class="mx-auto max-w-6xl">
  <ManageHeader
    icon={<Settings2 size={18} />}
    title={domain()?.name || "域管理"}
    description={domain()?.description || "管理域设置与成员"}
    actions={
      <Link
        to="/domain/$domainUUID"
        params={{ domainUUID: uuid() }}
        class="btn btn-ghost btn-sm"
      >
        <ArrowLeft size={16} />
        返回
      </Link>
    }
  />
```

Keep the surrounding `Show when={domain()}` and all loading/error behavior.

- [ ] **Step 3: Replace settings section shell**

Replace the current settings `<section>` opening and heading with:

```tsx
<ManageSection
  title="域设置"
  description="调整域名称、房间容量与公开状态"
  class="min-w-0"
>
```

Keep existing `Show when={canManage()}` content, form fields, validation messages, and save handler. Change form layout to `class="flex flex-col gap-4"`; set each label’s label text to `text-xs font-medium text-base-content/70`; use `class="btn btn-primary btn-sm w-full gap-2"` on save button so action aligns with the narrow settings column.

- [ ] **Step 4: Replace member section shell and add header actions**

Replace the current member `<section>` with:

```tsx
<ManageSection
  title="成员管理"
  description={`${members().length} 位成员`}
  class="min-w-0"
  bodyClass="p-0"
  padded={false}
  actions={
    <button
      type="button"
      class="btn btn-ghost btn-xs"
      disabled={memberLoading() || kicking()}
      onClick={() => void loadMembers(uuid()).catch(() => {})}
    >
      {memberLoading() ? (
        <span class="loading loading-spinner loading-xs" />
      ) : null}
      刷新
    </button>
  }
>
  <DomainMemberTable
    members={members()}
    ownerUUID={domain()?.owner_uuid}
    currentUserUUID={currentUser()?.uuid}
    canKick={canKick()}
    kickDisabled={kicking()}
    loading={memberLoading() && members().length === 0}
    refreshing={(memberLoading() || kicking()) && members().length > 0}
    error={memberError()}
    onRefresh={() => void loadMembers(uuid()).catch(() => {})}
    onKick={requestKick}
  />
</ManageSection>
```

Do not remove `DomainMemberTable`’s own refresh callback; header refresh is a visual convenience and uses the same `loadMembers` function.

- [ ] **Step 5: Add responsive grid around sections**

Wrap settings and member `ManageSection` siblings in:

```tsx
<div class="grid min-w-0 gap-5 lg:grid-cols-[minmax(360px,400px)_minmax(0,1fr)]">
  {/* settings section */}
  {/* member section */}
</div>
```

This creates desktop fixed settings/adaptive member columns and mobile stacked cards.

- [ ] **Step 6: Preserve modal placement and close ManagePage**

Keep `ConfirmModal` outside the content grid but inside the route component. Close `</ManagePage>` after loading/error blocks. Ensure no old `</div>` from the previous `flex-1 ... overflow-y-auto` wrapper remains.

- [ ] **Step 7: Run formatter and type checks**

Run:

```bash
pnpm --filter @go-rtc/web check
pnpm --filter @go-rtc/web lint
```

Expected: both commands exit `0`.

- [ ] **Step 8: Commit domain detail change**

```bash
git add 'app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx'
git commit -m "refactor(web): align domain detail layout"
```

---

### Task 3: Browser verification

**Files:**
- No source changes expected.

**Interfaces:**
- Consumes: committed RoomList and domain detail UI changes.
- Produces: verified desktop/mobile layout and unchanged interactions.

- [ ] **Step 1: Start frontend preview**

Use project preview configuration and open the web app. Navigate to `/manage/domains/<existing-domain-uuid>` and a room list route.

- [ ] **Step 2: Verify domain detail structure**

Check accessibility snapshot and DOM:

- `域设置` and `成员管理` are present.
- `返回` remains available.
- settings and member cards are side-by-side at desktop width.
- resize to mobile width; cards stack vertically.

- [ ] **Step 3: Verify RoomList overflow behavior**

Use a room with a long name and confirm:

- visible name ends with ellipsis rather than overlapping `count/limit`;
- hovering item exposes full name Tooltip;
- password icon remains visible when applicable.

- [ ] **Step 4: Check runtime errors**

Inspect browser console and preview logs. Expected: no new errors or failed module requests.

- [ ] **Step 5: Run final status check**

Run:

```bash
git status --short
```

Expected: only pre-existing unrelated worktree changes remain; both requested UI changes and design/plan docs are committed.
