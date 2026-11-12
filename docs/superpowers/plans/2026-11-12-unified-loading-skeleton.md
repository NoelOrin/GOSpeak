# 统一 Loading 与骨架屏 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 `@gospeak/web` 建立一套统一的「加载中」表达体系——共享的 `Skeleton` 原语 + 一组骨架组合件，把散落的 spinner、纯文字「加载中…」、以及两个孤岛骨架全部收敛成同一套骨架屏，并为当前完全没有 loading 反馈的区域补上骨架。

**架构：** 在 `packages/components` 新增无依赖的 `Skeleton` 原语（`bg-border animate-pulse`，读屏隐藏）。在 `app/web/src/components/common/Skeletons.tsx` 组合出 `SkeletonTableRows / SkeletonListRows / SkeletonCardGrid / SkeletonForm / SkeletonPage / LoadingRegion`。把既有三套入口——`ManageLoading`（spinner）、`AsyncState`（spinner 三态）、`keepAliveOutlet.PageSkeleton`（局部骨架）——统一到这套组合件上，再逐页替换内联 spinner 与纯文字加载态。

**技术栈：** React 19 + TypeScript、Tailwind 4 + 主题 token（`bg-border` / `text-muted-foreground` / `animate-pulse`）、vitest 5 + @testing-library/react、lucide-react。

**规格来源：** 用户直接需求「做好统一的 loading 与骨架屏；没骨架的都配一下」+ 本仓库现状勘察。

---

## 统一原则（实现前必读）

**1. 加载态分两类，处理方式不同：**
- **数据区域加载**（首次进入页面 / 面板 / 表格 / 列表且无旧数据）→ **骨架屏**。这是本计划的主战场。
- **操作进行中反馈**（提交按钮、上传、OAuth 登录中、刷新图标、「解除」按钮 pending）→ **保留 spinner**。骨架屏表达"内容还没到"，不表达"这一下点击正在处理"。**不要**把这类 spinner 也换成骨架。

**2. 有旧数据时不闪骨架：** 刷新/翻页沿用现有「保留内容 + 小 spinner」策略（如 `refreshing` prop），只在 `loading && 数据为空` 时才用骨架。所有改动都遵守 `loading && X.length === 0` 的既有守卫，不破坏翻页体验。

**3. 骨架只画结构，不画内容：** 颜色只走 token（`bg-border` 底色、`rounded-*` 与真实卡片一致），数量与真实行数接近（表格默认 5 行、列表 4~6 行），避免加载完成时高度跳变。

**4. 可访问性：** 每个骨架区域外层用 `LoadingRegion` 提供 `role="status"` + `aria-live="polite"` + `aria-busy="true"` + `sr-only` 文案；骨架块自身 `aria-hidden="true"`，读屏不会念出一堆空 div。

**5. 不引入新依赖：** 不装 `@radix-ui` 之外的东西，骨架就是 div + Tailwind。

---

## 现状覆盖清单

下表是本次要统一 / 补齐的全部位置。**「现状」为 `spinner` / `文字` / `无` 的都是本计划要换骨架或补骨架的对象；「已有骨架」只做去重收敛。**

| 文件 | 区域 | 现状 | 本计划处理 |
|------|------|------|-----------|
| `components/manage/ManageShell.tsx` | `ManageLoading` | spinner | 改为骨架（入口统一，惠及所有使用方） |
| `components/common/keepAliveOutlet.tsx` | `PageSkeleton` | 局部骨架 | 收敛到共享 `SkeletonPage` |
| `components/common/AsyncState.tsx` | 三态容器 | spinner | 新增 `skeleton` prop，加载态可渲染骨架 |
| `pages/(app)/manage/announcements/index.tsx` | 公告列表 | `ManageLoading` | 随 `ManageLoading` 自动升级 |
| `pages/(app)/manage/email/index.tsx` | 邮件配置 | `ManageLoading` | 随升级 |
| `pages/(app)/manage/roles/index.tsx` | 角色 / 权限 | `ManageLoading` | 随升级 |
| `pages/(app)/manage/sfu/index.tsx` | SFU 配置 | `ManageLoading` | 随升级 |
| `pages/(app)/manage/storage/index.tsx` | 存储配置 | `ManageLoading` | 随升级 |
| `pages/(app)/link.tsx` | 域列表 | `ManageLoading` | 随升级 |
| `components/discover/DiscoverDomainGrid.tsx` | 公开域网格 | `ManageLoading` | 换 `SkeletonCardGrid` |
| `pages/(app)/manage/ban/index.tsx` | 封禁表 | 内联 spinner | 换 `SkeletonTableRows` |
| `pages/(app)/manage/mute/index.tsx` | 禁言表 | 内联 spinner | 换 `SkeletonTableRows` |
| `pages/(app)/manage/users/index.tsx` | 用户表 | 内联 spinner | 换 `SkeletonTableRows` |
| `pages/(app)/manage/oauth/components/OAuthProviderTable.tsx` | OAuth 表 | 内联 spinner | 换 `SkeletonTableRows` |
| `components/domain/DomainMemberTable.tsx` | 成员表 loading 行 | 内联 spinner 行 | 换骨架行 |
| `components/domain/JoinRequestTable.tsx` | 申请列表 | `Loader2` 居中 | 换 `SkeletonTableRows` |
| `components/domain/DomainRoomTable.tsx` | 房间表 loading 行 | 内联 spinner 行 | 换骨架行 |
| `components/profile/UserGroupPanel.tsx` | 用户分组 | 纯文字「正在加载分组...」 | 换 `SkeletonListRows` |
| `components/domain/InviteRedemptionsPanel.tsx` | 邀请使用记录 | 纯文字「加载中…」 | 换 `SkeletonListRows` |
| `components/chat/memberSidebar.tsx` | 服务器成员 | 无 | 连接中补 `SkeletonListRows` |
| `pages/invite/d/$code.tsx` | 邀请落地 | `Loader2` 居中 | 换卡片骨架 |
| `pages/(app)/manage/monitor/index.tsx` | 监控指标卡 | 无（显示 `—`） | 首屏未拿到数据时指标值渲染骨架 |
| `components/chat/chatWindow.tsx` | 消息区 | `AsyncState` spinner | 传 `skeleton` |
| `components/chat/conversationList.tsx` | 会话列表 | `AsyncState` spinner | 传 `skeleton` |
| `components/textRoom/MessageList.tsx` | 文本房消息 | `AsyncState` spinner | 传 `skeleton` |
| `components/discover/DiscoverMyDomains.tsx` | 我的域 | `AsyncState` spinner | 传 `skeleton` |
| `pages/(app)/manage/domains/index.tsx` | 域列表 | `AsyncState` spinner | 传 `skeleton` |
| `components/room/roomList.tsx` | 房间列表 | `RoomItemSkeleton`（孤岛） | 重写为使用共享 `Skeleton` 原语 |
| `pages/(app)/discover.tsx` 刷新按钮 / `login.tsx` OAuth 登录中 / 各提交按钮 | 操作中 | spinner | **不改**（操作反馈） |

---

## 文件结构

### 新建

| 文件 | 职责 |
|------|------|
| `packages/components/src/components/skeleton.tsx` | `Skeleton` 原语：一个带 `animate-pulse` 的占位 div，`aria-hidden`，支持 `className` 覆盖尺寸与圆角 |
| `packages/components/src/components/skeleton.test.tsx` | 单测：服务端渲染静态标记，断言 `animate-pulse` / `aria-hidden` / className 透传 |
| `app/web/src/components/common/Skeletons.tsx` | 骨架组合件：`LoadingRegion`、`SkeletonLine`、`SkeletonTableRows`、`SkeletonListRows`、`SkeletonCardGrid`、`SkeletonForm`、`SkeletonPage` |
| `app/web/src/components/common/skeletons.test.tsx` | 单测：各组合件渲染结构、行数、`role="status"` 语义 |

### 修改

见「现状覆盖清单」与各任务「文件」段落。

---

## 任务 1：Skeleton 原语（packages/components）

**文件：**
- 创建：`packages/components/src/components/skeleton.tsx`
- 创建：`packages/components/src/components/skeleton.test.tsx`
- 修改：`packages/components/src/index.ts`

- [ ] **步骤 1：编写失败的测试**

创建 `packages/components/src/components/skeleton.test.tsx`：

```tsx
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Skeleton } from "./skeleton";

describe("Skeleton", () => {
	it("渲染带脉冲动画的占位块并对读屏隐藏", () => {
		const html = renderToStaticMarkup(<Skeleton className="h-4 w-10" />);
		expect(html).toContain("animate-pulse");
		expect(html).toContain('aria-hidden="true"');
		expect(html).toContain('data-slot="skeleton"');
	});

	it("透传并合并 className（尺寸/圆角覆盖默认值）", () => {
		const html = renderToStaticMarkup(
			<Skeleton className="h-24 w-full rounded-lg" />,
		);
		expect(html).toContain("h-24");
		expect(html).toContain("rounded-lg");
		// tailwind-merge 应让调用方圆角覆盖默认 rounded-md
		expect(html).not.toContain("rounded-md");
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pnpm --filter @gospeak/components test skeleton`
预期：FAIL，报错 `Failed to resolve import "./skeleton"`。

- [ ] **步骤 3：实现 Skeleton**

创建 `packages/components/src/components/skeleton.tsx`：

```tsx
import type { HTMLAttributes } from "react";
import { cn } from "../lib/utils";

export type SkeletonProps = HTMLAttributes<HTMLDivElement>;

/**
 * 骨架占位块：一个带脉冲动画的灰色方块。
 * 尺寸与圆角由调用方用 className 覆盖（tailwind-merge 会处理冲突）。
 * 对读屏隐藏——骨架只是视觉占位，语义由外层 LoadingRegion 承担。
 */
export function Skeleton({ className, ...props }: SkeletonProps) {
	return (
		<div
			data-slot="skeleton"
			aria-hidden="true"
			className={cn("bg-border animate-pulse rounded-md", className)}
			{...props}
		/>
	);
}
```

- [ ] **步骤 4：导出 Skeleton**

在 `packages/components/src/index.ts` 末尾追加：

```ts
export { Skeleton, type SkeletonProps } from "./components/skeleton";
```

- [ ] **步骤 5：运行测试验证通过**

运行：`pnpm --filter @gospeak/components test skeleton`
预期：PASS（2 passed）。

- [ ] **步骤 6：Typecheck**

运行：`pnpm --filter @gospeak/components typecheck`
预期：无错误。

- [ ] **步骤 7：Commit**

```bash
git add packages/components/src/components/skeleton.tsx packages/components/src/components/skeleton.test.tsx packages/components/src/index.ts
git commit -m "feat(components): 新增 Skeleton 骨架占位原语"
```

---

## 任务 2：骨架组合件（web）

**文件：**
- 创建：`app/web/src/components/common/Skeletons.tsx`
- 创建：`app/web/src/components/common/skeletons.test.tsx`

- [ ] **步骤 1：编写失败的测试**

创建 `app/web/src/components/common/skeletons.test.tsx`：

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
	LoadingRegion,
	SkeletonCardGrid,
	SkeletonForm,
	SkeletonListRows,
	SkeletonPage,
	SkeletonTableRows,
} from "./Skeletons";

describe("Skeletons", () => {
	it("LoadingRegion 暴露 status 语义与读屏文案", () => {
		render(
			<LoadingRegion label="正在加载用户">
				<SkeletonTableRows rows={2} cols={3} />
			</LoadingRegion>,
		);
		const region = screen.getByRole("status");
		expect(region.getAttribute("aria-busy")).toBe("true");
		expect(region.getAttribute("aria-label")).toBe("正在加载用户");
	});

	it("SkeletonTableRows 按 rows*cols 渲染占位块", () => {
		const { container } = render(
			<LoadingRegion label="加载">
				<SkeletonTableRows rows={3} cols={4} />
			</LoadingRegion>,
		);
		expect(
			container.querySelectorAll('[data-slot="skeleton"]').length,
		).toBe(12);
	});

	it("SkeletonListRows / SkeletonCardGrid / SkeletonForm 至少渲染一个占位块", () => {
		for (const node of [
			<SkeletonListRows key="l" rows={4} />,
			<SkeletonCardGrid key="c" cards={4} />,
			<SkeletonForm key="f" fields={3} />,
		]) {
			const { container, unmount } = render(node);
			expect(
				container.querySelectorAll('[data-slot="skeleton"]').length,
			).toBeGreaterThan(0);
			unmount();
		}
	});

	it("SkeletonPage 保留页面加载骨架的可访问性契约", () => {
		render(<SkeletonPage />);
		const region = screen.getByRole("status");
		expect(region.getAttribute("aria-label")).toBe("页面加载中");
		expect(
			region.querySelectorAll('[data-slot="skeleton"]').length,
		).toBeGreaterThan(0);
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pnpm --filter @gospeak/web test skeletons`
预期：FAIL，`Failed to resolve import "./Skeletons"`。

- [ ] **步骤 3：实现组合件**

创建 `app/web/src/components/common/Skeletons.tsx`：

```tsx
import { Skeleton } from "@gospeak/components";
import type { ReactNode } from "react";

/** 加载区域的语义外壳：role=status + sr-only 文案，骨架块自身对读屏隐藏。 */
export function LoadingRegion({
	label = "加载中",
	className,
	children,
}: {
	label?: string;
	className?: string;
	children: ReactNode;
}) {
	return (
		<div
			role="status"
			aria-live="polite"
			aria-busy="true"
			aria-label={label}
			className={`w-full ${className ?? ""}`}
		>
			<span className="sr-only">{label}</span>
			{children}
		</div>
	);
}

export function SkeletonLine({ className }: { className?: string }) {
	return <Skeleton className={`h-4 ${className ?? "w-full"}`} />;
}

/** 表格骨架：rows 行、每行 cols 列等宽占位。 */
export function SkeletonTableRows({
	rows = 5,
	cols = 4,
	className,
}: {
	rows?: number;
	cols?: number;
	className?: string;
}) {
	return (
		<div
			data-skeleton="table"
			className={`flex flex-col gap-3 p-3 ${className ?? ""}`}
		>
			{Array.from({ length: rows }).map((_, r) => (
				<div key={r} className="flex items-center gap-3">
					{Array.from({ length: cols }).map((_, c) => (
						<Skeleton key={c} className="h-4 flex-1" />
					))}
				</div>
			))}
		</div>
	);
}

/** 列表骨架：圆形头像 + 两行文字，用于会话/成员/分组等竖列表。 */
export function SkeletonListRows({
	rows = 5,
	className,
}: {
	rows?: number;
	className?: string;
}) {
	return (
		<div
			data-skeleton="list"
			className={`flex flex-col gap-3 p-3 ${className ?? ""}`}
		>
			{Array.from({ length: rows }).map((_, i) => (
				<div key={i} className="flex items-center gap-3">
					<Skeleton className="size-8 shrink-0 rounded-full" />
					<div className="flex min-w-0 flex-1 flex-col gap-1.5">
						<Skeleton className="h-3.5 w-1/3" />
						<Skeleton className="h-3 w-2/3" />
					</div>
				</div>
			))}
		</div>
	);
}

/** 卡片网格骨架：用于域卡片等网格布局。 */
export function SkeletonCardGrid({
	cards = 4,
	columns = 2,
	className,
}: {
	cards?: number;
	columns?: number;
	className?: string;
}) {
	const gridCols =
		columns >= 3
			? "sm:grid-cols-2 lg:grid-cols-3"
			: columns === 2
				? "sm:grid-cols-2"
				: "";
	return (
		<div
			data-skeleton="card-grid"
			className={`grid gap-3 ${gridCols} ${className ?? ""}`}
		>
			{Array.from({ length: cards }).map((_, i) => (
				<div key={i} className="border-border/60 rounded-lg border p-4">
					<div className="flex items-center gap-3">
						<Skeleton className="size-10 shrink-0 rounded-lg" />
						<div className="flex flex-1 flex-col gap-2">
							<Skeleton className="h-4 w-1/2" />
							<Skeleton className="h-3 w-3/4" />
						</div>
					</div>
					<Skeleton className="mt-3 h-3 w-full" />
				</div>
			))}
		</div>
	);
}

/** 表单骨架：标签 + 输入框。 */
export function SkeletonForm({
	fields = 3,
	className,
}: {
	fields?: number;
	className?: string;
}) {
	return (
		<div
			data-skeleton="form"
			className={`flex flex-col gap-4 ${className ?? ""}`}
		>
			{Array.from({ length: fields }).map((_, i) => (
				<div key={i} className="flex flex-col gap-2">
					<Skeleton className="h-3.5 w-24" />
					<Skeleton className="h-9 w-full rounded-sm" />
				</div>
			))}
		</div>
	);
}

/** 整页骨架：Suspense fallback，保留既有 aria-label 契约。 */
export function SkeletonPage() {
	return (
		<LoadingRegion label="页面加载中" className="h-full p-4">
			<div className="flex h-full w-full flex-col gap-3">
				<Skeleton className="h-6 w-40" />
				<Skeleton className="h-24 w-full rounded-lg" />
				<Skeleton className="h-24 w-full rounded-lg" />
				<div className="flex gap-3">
					<Skeleton className="h-16 flex-1 rounded-lg" />
					<Skeleton className="h-16 flex-1 rounded-lg" />
				</div>
			</div>
		</LoadingRegion>
	);
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`pnpm --filter @gospeak/web test skeletons`
预期：PASS（4 passed）。

- [ ] **步骤 5：Commit**

```bash
git add app/web/src/components/common/Skeletons.tsx app/web/src/components/common/skeletons.test.tsx
git commit -m "feat(web): 新增骨架组合件与 LoadingRegion"
```

---

## 任务 3：收敛 ManageLoading 与 PageSkeleton

**文件：**
- 修改：`app/web/src/components/manage/ManageShell.tsx`（`ManageLoading`）
- 修改：`app/web/src/components/common/keepAliveOutlet.tsx`（`PageSkeleton`）
- 测试：`app/web/src/components/common/keepAliveOutlet.test.tsx`（既有，不改断言）

- [ ] **步骤 1：先确认既有骨架测试通过（基线）**

运行：`pnpm --filter @gospeak/web test keepAliveOutlet`
预期：PASS。记录断言：`getByRole("status")` 的 `aria-label === "页面加载中"`，且 `.animate-pulse` 数量 > 0。

- [ ] **步骤 2：把 ManageLoading 改为骨架**

将 `app/web/src/components/manage/ManageShell.tsx` 中的 `ManageLoading` 替换为（同时把顶部 `import { Loader2 } from "lucide-react";` 改为 `import type { ReactNode } from "react";`，并新增 `import { LoadingRegion } from "@/components/common/Skeletons";`；若文件已有 `ReactNode` 导入则合并）：

```tsx
/** 页内加载占位：骨架屏而非 spinner，保留页头时使用，避免整页闪白与高度跳变。 */
export function ManageLoading({
	className,
	label,
}: {
	className?: string;
	label?: string;
}) {
	return (
		<LoadingRegion label={label ?? "加载中"} className={`py-2 ${className ?? ""}`}>
			<div className="flex flex-col gap-3">
				<Skeleton className="h-5 w-40" />
				<Skeleton className="h-24 w-full rounded-lg" />
				<Skeleton className="h-24 w-full rounded-lg" />
			</div>
		</LoadingRegion>
	);
}
```

其中 `Skeleton` 需从 `@gospeak/components` 导入：`import { Skeleton } from "@gospeak/components";`（该文件已从 `@gospeak/components` 导入过组件，合并进现有 import）。

> 注意：`ManageLoading` 是 7 处页面/组件的统一入口，此一步即把 announcements / email / roles / sfu / storage / link / DiscoverDomainGrid 的 spinner 全量升级为骨架。

- [ ] **步骤 3：把 keepAliveOutlet 的 PageSkeleton 换成共享组件**

修改 `app/web/src/components/common/keepAliveOutlet.tsx`：
1) 删除本地 `function PageSkeleton()`（含其上方注释块）；
2) 顶部新增 `import { SkeletonPage } from "@/components/common/Skeletons";`；
3) 将渲染处 `<Suspense fallback={<PageSkeleton />}>` 改为 `<Suspense fallback={<SkeletonPage />}>`。

- [ ] **步骤 4：运行既有骨架测试验证契约不变**

运行：`pnpm --filter @gospeak/web test keepAliveOutlet`
预期：PASS（`aria-label === "页面加载中"`、存在 `[data-slot="skeleton"]` 且 `.animate-pulse` 数量 > 0）。

- [ ] **步骤 5：Typecheck**

运行：`pnpm --filter @gospeak/web typecheck`
预期：无错误（注意 `@/components/common/Skeletons` 路径别名可用）。

- [ ] **步骤 6：Commit**

```bash
git add app/web/src/components/manage/ManageShell.tsx app/web/src/components/common/keepAliveOutlet.tsx
git commit -m "refactor(web): ManageLoading 与 PageSkeleton 收敛到共享骨架"
```

---

## 任务 4：AsyncState 支持 skeleton，并接入 5 处数据容器

**文件：**
- 修改：`app/web/src/components/common/AsyncState.tsx`
- 修改：`app/web/src/components/chat/chatWindow.tsx`
- 修改：`app/web/src/components/chat/conversationList.tsx`
- 修改：`app/web/src/components/textRoom/MessageList.tsx`
- 修改：`app/web/src/components/discover/DiscoverMyDomains.tsx`
- 修改：`app/web/src/pages/(app)/manage/domains/index.tsx`
- 测试：`app/web/src/components/common/asyncState.test.tsx`（新建）

- [ ] **步骤 1：编写失败的测试**

创建 `app/web/src/components/common/asyncState.test.tsx`：

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import AsyncState from "./AsyncState";

describe("AsyncState", () => {
	it("loading 且提供 skeleton 时渲染骨架而不是 spinner", () => {
		render(
			<AsyncState
				loading
				loadingText="正在加载消息…"
				skeleton={<div data-testid="sk">骨架</div>}
			>
				<div>内容</div>
			</AsyncState>,
		);
		expect(screen.getByTestId("sk")).toBeTruthy();
		expect(screen.queryByText("正在加载消息…")).toBeNull();
	});

	it("loading 且无 skeleton 时保留 spinner + loadingText", () => {
		render(<AsyncState loading loadingText="正在加载…" />);
		expect(screen.getByText("正在加载…")).toBeTruthy();
	});

	it("非 loading/error/empty 时渲染 children", () => {
		render(
			<AsyncState>
				<div>内容</div>
			</AsyncState>,
		);
		expect(screen.getByText("内容")).toBeTruthy();
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pnpm --filter @gospeak/web test asyncState`
预期：FAIL（`skeleton` 不生效，仍渲染 loadingText）。

- [ ] **步骤 3：给 AsyncState 增加 skeleton prop**

在 `app/web/src/components/common/AsyncState.tsx`：
1) 在 `AsyncStateProps` 增加字段：

```ts
	/** 加载态的骨架屏；提供时替代默认 spinner 文案区。 */
	skeleton?: ReactNode;
```

2) 在参数解构中增加 `skeleton,`；
3) 把 `if (loading) { return (...) }` 分支改为先判断 skeleton：

```tsx
	if (loading) {
		if (skeleton) {
			return <LoadingRegion label={typeof loadingText === "string" ? loadingText : "加载中"}>{skeleton}</LoadingRegion>;
		}
		return (
			<div
				className={`${box} text-muted-foreground ${className ?? ""}`}
				aria-live="polite"
				aria-busy="true"
			>
				<Loader2 className="size-4 animate-spin" />
				<span>{loadingText}</span>
			</div>
		);
	}
```

4) 顶部新增 `import { LoadingRegion } from "@/components/common/Skeletons";`（`ReactNode` 已存在）。

- [ ] **步骤 4：运行测试验证通过**

运行：`pnpm --filter @gospeak/web test asyncState`
预期：PASS（3 passed）。

- [ ] **步骤 5：接入 5 处调用方**

逐处给 `AsyncState` 传 `skeleton`（其余 props 保持不变）：

`app/web/src/components/chat/conversationList.tsx`（会话列表，竖列表）：

```tsx
			<AsyncState
				loading={loadingList && conversations.length === 0}
				loadingText="正在加载会话…"
				skeleton={<SkeletonListRows rows={6} />}
				empty={conversations.length === 0 && !loadingList}
				...
```

顶部新增 `import { SkeletonListRows } from "@/components/common/Skeletons";`。

`app/web/src/components/chat/chatWindow.tsx`（消息区，气泡列表，用列表骨架即可）：

```tsx
			<AsyncState
				loading={isLoading && messages.length === 0}
				loadingText="正在加载消息…"
				skeleton={<SkeletonListRows rows={6} className="max-w-2xl" />}
				empty={messages.length === 0 && !isLoading}
```

`app/web/src/components/textRoom/MessageList.tsx`：

```tsx
			<AsyncState
				loading={loading && visibleMessages.length === 0}
				loadingText="正在加载消息…"
				skeleton={<SkeletonListRows rows={6} />}
```

（保留其余 props）

`app/web/src/components/discover/DiscoverMyDomains.tsx`：

```tsx
			<AsyncState
				compact
				loading={(loading || domainsLoading) && myUUIDs.length > 0}
				loadingText="正在加载我的域…"
				skeleton={<SkeletonCardGrid cards={2} columns={2} />}
```

`app/web/src/pages/(app)/manage/domains/index.tsx`：

```tsx
				<AsyncState
					loading={uuidsLoading || domainsLoading}
					loadingText="正在加载你的域…"
					skeleton={<SkeletonCardGrid cards={4} columns={2} />}
```

以上每处均新增对应 `Skeleton*` 导入。

- [ ] **步骤 6：运行相关测试 + typecheck**

运行：`pnpm --filter @gospeak/web test asyncState chatWindow conversationList`（若无对应用例名，则退回 `pnpm --filter @gospeak/web test components/chat`）
再运行：`pnpm --filter @gospeak/web typecheck`
预期：通过。

- [ ] **步骤 7：Commit**

```bash
git add app/web/src/components/common/AsyncState.tsx app/web/src/components/common/asyncState.test.tsx \
  app/web/src/components/chat/chatWindow.tsx app/web/src/components/chat/conversationList.tsx \
  app/web/src/components/textRoom/MessageList.tsx app/web/src/components/discover/DiscoverMyDomains.tsx \
  "app/web/src/pages/(app)/manage/domains/index.tsx"
git commit -m "feat(web): AsyncState 支持骨架屏并接入消息/会话/域列表"
```

---

## 任务 5：管理页内联 spinner → 表格骨架

**文件：**
- 修改：`app/web/src/pages/(app)/manage/ban/index.tsx:135`
- 修改：`app/web/src/pages/(app)/manage/mute/index.tsx:182`
- 修改：`app/web/src/pages/(app)/manage/users/index.tsx:317`
- 修改：`app/web/src/pages/(app)/manage/oauth/components/OAuthProviderTable.tsx:34`

- [ ] **步骤 1：替换 ban 的加载态**

`ban/index.tsx`：把

```tsx
				{isLoading ? (
					<span className="border-primary border-t-primary m-4 block size-5 animate-spin rounded-full border-2" />
				) : bannedUsers.length === 0 ? (
```

改为

```tsx
				{isLoading ? (
					<LoadingRegion label="正在加载封禁用户">
						<SkeletonTableRows rows={4} cols={4} />
					</LoadingRegion>
				) : bannedUsers.length === 0 ? (
```

顶部新增 `import { LoadingRegion, SkeletonTableRows } from "@/components/common/Skeletons";`。

- [ ] **步骤 2：替换 mute 的加载态**

`mute/index.tsx`：把

```tsx
				{mutesLoading ? (
					<span className="border-primary border-t-primary m-4 block size-5 animate-spin rounded-full border-2" />
				) : (mutes?.length || 0) === 0 ? (
```

改为

```tsx
				{mutesLoading ? (
					<LoadingRegion label="正在加载禁言列表">
						<SkeletonTableRows rows={4} cols={6} />
					</LoadingRegion>
				) : (mutes?.length || 0) === 0 ? (
```

顶部新增 import。

- [ ] **步骤 3：替换 users 的加载态**

`users/index.tsx`：把

```tsx
				{usersLoading ? (
					<span className="border-primary border-t-primary mx-auto block size-5 animate-spin rounded-full border-2 py-10" />
				) : users.length === 0 ? (
```

改为

```tsx
				{usersLoading ? (
					<LoadingRegion label="正在加载用户">
						<SkeletonTableRows rows={8} cols={6} />
					</LoadingRegion>
				) : users.length === 0 ? (
```

顶部新增 import。

- [ ] **步骤 4：替换 OAuthProviderTable 的加载态**

`OAuthProviderTable.tsx`：把

```tsx
			{loading ? (
				<span className="border-primary border-t-primary m-4 block size-5 animate-spin rounded-full border-2" />
			) : providers.length === 0 ? (
```

改为

```tsx
			{loading ? (
				<LoadingRegion label="正在加载 OAuth 提供商">
					<SkeletonTableRows rows={4} cols={5} />
				</LoadingRegion>
			) : providers.length === 0 ? (
```

顶部新增 `import { LoadingRegion, SkeletonTableRows } from "@/components/common/Skeletons";`。

- [ ] **步骤 5：验证无残留内联 spinner + typecheck**

运行：
```bash
grep -rn "border-primary border-t-primary" "app/web/src/pages/(app)/manage/ban/index.tsx" "app/web/src/pages/(app)/manage/mute/index.tsx" "app/web/src/pages/(app)/manage/users/index.tsx" "app/web/src/pages/(app)/manage/oauth/components/OAuthProviderTable.tsx"
```
预期：无输出。
再运行：`pnpm --filter @gospeak/web typecheck`，预期通过。

- [ ] **步骤 6：Commit**

```bash
git add "app/web/src/pages/(app)/manage/ban/index.tsx" "app/web/src/pages/(app)/manage/mute/index.tsx" "app/web/src/pages/(app)/manage/users/index.tsx" "app/web/src/pages/(app)/manage/oauth/components/OAuthProviderTable.tsx"
git commit -m "refactor(web): 管理页表格加载态统一为骨架"
```

---

## 任务 6：领域表格组件 loading 行 → 骨架行

**文件：**
- 修改：`app/web/src/components/domain/DomainMemberTable.tsx:154`
- 修改：`app/web/src/components/domain/DomainRoomTable.tsx:82`
- 修改：`app/web/src/components/domain/JoinRequestTable.tsx:90`

> 这三个组件把加载态放在 `<tbody>` 内的一行（`colSpan`），或整体居中 spinner。骨架行继续放在 `<tbody>` 内，保持表格列宽不跳。`getMemberTableStatus` / `getRoomTableStatus` / `isMemberTableBusy` 等既有导出与逻辑**不改**。

- [ ] **步骤 1：DomainMemberTable 的 loading 行改骨架**

把

```tsx
						{status === "loading" && (
							<tr>
								<td
									colSpan={4}
									className="text-faint-foreground py-10 text-center text-sm"
								>
									<span className="border-primary border-t-primary mr-2 inline-block size-4 animate-spin rounded-full border-2" />
									正在加载成员
								</td>
							</tr>
						)}
```

改为

```tsx
						{status === "loading" &&
							Array.from({ length: 5 }).map((_, i) => (
								<tr key={`sk-${i}`} className="border-border/60 border-b">
									<td className="px-3 py-2.5">
										<Skeleton className="h-4 w-24" />
									</td>
									<td className="px-3 py-2.5">
										<Skeleton className="h-4 w-16" />
									</td>
									<td className="px-3 py-2.5">
										<Skeleton className="h-4 w-28" />
									</td>
									<td className="px-3 py-2.5">
										<Skeleton className="ml-auto h-4 w-16" />
									</td>
								</tr>
							))}
```

顶部新增 `import { Skeleton } from "@gospeak/components";`（该文件已从 @gospeak/components 导入 Select 等，合并）。

- [ ] **步骤 2：DomainRoomTable 的 loading 行改骨架**

把

```tsx
						{status === "loading" && (
							<tr>
								<td
									colSpan={6}
									className="text-faint-foreground py-10 text-center text-sm"
								>
									<span className="border-primary border-t-primary mr-2 inline-block size-4 animate-spin rounded-full border-2" />
									正在加载房间
								</td>
							</tr>
						)}
```

改为

```tsx
						{status === "loading" &&
							Array.from({ length: 5 }).map((_, i) => (
								<tr key={`sk-${i}`} className="border-border/60 border-b">
									{Array.from({ length: 6 }).map((__, c) => (
										<td key={c} className="px-3 py-2.5">
											<Skeleton className={c === 5 ? "ml-auto h-4 w-16" : "h-4 w-20"} />
										</td>
									))}
								</tr>
							))}
```

顶部新增 `import { Skeleton } from "@gospeak/components";`。

- [ ] **步骤 3：JoinRequestTable 的 loading 改骨架**

把

```tsx
			{props.loading ? (
				<div className="text-faint-foreground flex items-center justify-center py-8">
					<Loader2 className="size-5 animate-spin" />
				</div>
			) : props.items.length === 0 ? (
```

改为

```tsx
			{props.loading ? (
				<LoadingRegion label="正在加载加入申请">
					<SkeletonTableRows rows={5} cols={6} />
				</LoadingRegion>
			) : props.items.length === 0 ? (
```

顶部新增 `import { LoadingRegion, SkeletonTableRows } from "@/components/common/Skeletons";`；若 `Loader2` 再无其他使用则从 lucide 导入中移除。

- [ ] **步骤 4：运行相关测试 + typecheck**

运行：`pnpm --filter @gospeak/web test JoinRequestTable DomainMemberTable DomainRoomTable`
预期：既有状态逻辑测试全绿（UI 断言如有针对 spinner 的需同步改为 `[data-slot="skeleton"]`；本步允许修正测试以匹配新结构）。
再运行：`pnpm --filter @gospeak/web typecheck`，预期通过。

- [ ] **步骤 5：Commit**

```bash
git add app/web/src/components/domain/DomainMemberTable.tsx app/web/src/components/domain/DomainRoomTable.tsx app/web/src/components/domain/JoinRequestTable.tsx
git commit -m "refactor(web): 领域表格加载行统一为骨架"
```

---

## 任务 7：纯文字/缺失加载态 → 骨架

**文件：**
- 修改：`app/web/src/components/profile/UserGroupPanel.tsx:209`
- 修改：`app/web/src/components/domain/InviteRedemptionsPanel.tsx:68`
- 修改：`app/web/src/components/chat/memberSidebar.tsx`
- 修改：`app/web/src/pages/invite/d/$code.tsx:206`

- [ ] **步骤 1：UserGroupPanel 加载态改骨架**

把

```tsx
				) : (
					<div className="bg-muted/70 text-faint-foreground rounded-lg px-4 py-8 text-center text-sm">
						{loading ? "正在加载分组..." : "还没有用户分组"}
					</div>
				)}
```

改为

```tsx
				) : loading ? (
					<LoadingRegion label="正在加载分组">
						<SkeletonListRows rows={3} />
					</LoadingRegion>
				) : (
					<div className="bg-muted/70 text-faint-foreground rounded-lg px-4 py-8 text-center text-sm">
						还没有用户分组
					</div>
				)}
```

顶部新增 `import { LoadingRegion, SkeletonListRows } from "@/components/common/Skeletons";`。

- [ ] **步骤 2：InviteRedemptionsPanel 加载态改骨架**

把

```tsx
			{loading && <p className="text-muted-foreground text-xs">加载中…</p>}
```

改为

```tsx
			{loading && (
				<LoadingRegion label="正在加载使用记录">
					<SkeletonListRows rows={4} />
				</LoadingRegion>
			)}
```

顶部新增 import。

- [ ] **步骤 3：memberSidebar 连接中补骨架**

`memberSidebar.tsx` 已有 `const connected = useSocketStore((s) => s.connected);`。把成员列表区域：

```tsx
			<div className="min-h-0 flex-1 overflow-y-auto px-1 py-1">
				{sortedMembers.length > 0 ? (
					sortedMembers.map((member) => (
```

改为在列表前插入加载分支：

```tsx
			<div className="min-h-0 flex-1 overflow-y-auto px-1 py-1">
				{!connected && sortedMembers.length === 0 ? (
					<LoadingRegion label="正在加载成员">
						<SkeletonListRows rows={6} />
					</LoadingRegion>
				) : sortedMembers.length > 0 ? (
					sortedMembers.map((member) => (
```

保持后面 `: (...)` 的空态分支不变。顶部新增 import。

- [ ] **步骤 4：邀请落地页 loading 改卡片骨架**

`pages/invite/d/$code.tsx`：把

```tsx
				{phase === "loading" && (
					<div className="flex flex-col items-center gap-3 py-10">
						<Loader2 className="text-muted-foreground size-6 animate-spin" />
						<p className="text-muted-foreground text-sm">正在打开邀请…</p>
					</div>
				)}
```

改为

```tsx
				{phase === "loading" && (
					<LoadingRegion label="正在打开邀请">
						<div className="flex flex-col gap-3 py-2">
							<Skeleton className="h-5 w-32" />
							<Skeleton className="h-4 w-full" />
							<Skeleton className="h-4 w-2/3" />
							<Skeleton className="mt-2 h-10 w-full rounded-sm" />
						</div>
					</LoadingRegion>
				)}
```

顶部新增 `import { LoadingRegion } from "@/components/common/Skeletons";` 与 `import { Skeleton } from "@gospeak/components";`；若 `Loader2` 不再使用则移除（该文件第 343 行可能仍在用，按实际保留）。

- [ ] **步骤 5：Typecheck**

运行：`pnpm --filter @gospeak/web typecheck`
预期：通过。

- [ ] **步骤 6：Commit**

```bash
git add app/web/src/components/profile/UserGroupPanel.tsx app/web/src/components/domain/InviteRedemptionsPanel.tsx app/web/src/components/chat/memberSidebar.tsx "app/web/src/pages/invite/d/\$code.tsx"
git commit -m "feat(web): 补齐分组/邀请记录/成员/邀请落地的骨架屏"
```

---

## 任务 8：监控指标卡首屏骨架

**文件：**
- 修改：`app/web/src/pages/(app)/manage/monitor/index.tsx`（`MetricCard`）

- [ ] **步骤 1：MetricCard 空值渲染骨架**

在 `MetricCard` 中，把读取 `value` 的渲染分支改为：当 `value === null`（即未拿到首帧数据）且未提供 badge 时渲染骨架，而不是 `—`。修改 `MetricCard` 内部：

```tsx
				{badge || (
					value === null ? (
						<Skeleton className="h-6 w-16" />
					) : (
						<div
							className={`text-lg font-bold ${dim ? "text-faint-foreground" : ""}`}
						>
							{value}
						</div>
					)
				)}
```

> 各调用点已用 `snap?.xxx != null ? String(...) : "—"` 传值，需同步把这些 `: "—"` 改为 `: null`，让骨架生效。逐一替换 `value={ ... : "—"}` → `value={ ... : null}`（保留 `snap?.x != null` 判断）。

顶部新增 `import { Skeleton } from "@gospeak/components";`（该文件已从 @gospeak/components 导入 Button，合并）。

- [ ] **步骤 2：Typecheck**

运行：`pnpm --filter @gospeak/web typecheck`
预期：通过（`MetricCard` 的 `value` 类型已是 `string | null`）。

- [ ] **步骤 3：Commit**

```bash
git add "app/web/src/pages/(app)/manage/monitor/index.tsx"
git commit -m "feat(web): 监控指标卡首屏使用骨架占位"
```

---

## 任务 9：roomList 孤岛骨架归并

**文件：**
- 修改：`app/web/src/components/room/roomList.tsx:30`

- [ ] **步骤 1：用共享 Skeleton 重写 RoomItemSkeleton**

把本地 `RoomItemSkeleton` 的实现改为使用共享原语（保留组件名与调用点不变，避免牵连 `RoomListFallback` 逻辑）：

```tsx
function RoomItemSkeleton() {
	return (
		<div className="flex w-full shrink-0 flex-col">
			<div className="flex items-center justify-between border-0 px-1.5 py-2">
				<div className="flex w-full items-center space-x-1">
					<Skeleton className="h-4 flex-1 rounded-sm" />
				</div>
				<div className="flex items-center text-xs">
					<Skeleton className="h-4 w-8 rounded-sm" />
				</div>
			</div>
		</div>
	);
}
```

顶部新增 `import { Skeleton } from "@gospeak/components";`（该文件已从 @gospeak/components 导入 toast、Button，合并）。

- [ ] **步骤 2：Typecheck**

运行：`pnpm --filter @gospeak/web typecheck`
预期：通过。

- [ ] **步骤 3：Commit**

```bash
git add app/web/src/components/room/roomList.tsx
git commit -m "refactor(web): roomList 骨架改用共享 Skeleton 原语"
```

---

## 任务 10：整体验证与收尾

**文件：** 无新增

- [ ] **步骤 1：全量静态检查**

运行：
```bash
pnpm --filter @gospeak/components typecheck
pnpm --filter @gospeak/web typecheck
pnpm --filter @gospeak/components lint
pnpm --filter @gospeak/web lint
```
预期：全部通过。若 biome 报格式化，运行 `pnpm --filter @gospeak/web exec biome check --write <改动文件>` 后重跑。

- [ ] **步骤 2：全量单测**

运行：
```bash
pnpm --filter @gospeak/components test
pnpm --filter @gospeak/web test
```
预期：全绿。重点关注 `keepAliveOutlet`、`asyncState`、`skeletons`、`JoinRequestTable`。

- [ ] **步骤 3：残留检查（确认"没骨架的"清零）**

运行：
```bash
grep -rn "border-primary border-t-primary" app/web/src --include=*.tsx
grep -rn "loading && <p" app/web/src --include=*.tsx
```
预期：第一条只剩操作型小 spinner（如 DomainMemberTable 的 refreshing 行内刷新、Mobile 刷新），不再有整块表格/面板加载态；第二条无输出。人工逐条确认剩余项属于「操作进行中反馈」。

- [ ] **步骤 4：人工视觉验收（可选但建议）**

在 worktree 启动：`pnpm --filter @gospeak/web dev`，Chrome DevTools 用 Network 节流（Slow 3G）逐页刷新，确认：
- 管理页各表格首屏是骨架行而非空白/spinner；
- 消息、会话、域列表首屏是骨架；
- 切到 /chat /discover /profile chunk 加载时是整页骨架；
- 无高度剧烈跳变、无控制台报错。

- [ ] **步骤 5：Commit 收尾（如 lint 自动修复产生改动）**

```bash
git add -A
git commit -m "chore(web): 统一 loading/骨架屏后格式化收尾"
```

---

## 风险与回滚

- **测试耦合 spinner 断言**：`DomainMemberTable` / `DomainRoomTable` / `JoinRequestTable` 的既有测试若断言了 `animate-spin` 或「正在加载成员」文案，需改为断言 `[data-slot="skeleton"]`。任务 6 已注明允许同步修正。
- **ManageLoading 全局改动面大**：一次改动影响 7 处，但入口单一、API 不变（仍接收 `label`/`className`），风险可控；先跑 `keepAliveOutlet` 与全量 web 单测兜底。
- **翻页闪骨架**：所有骨架分支都在 `loading && 数据为空` 守卫下，翻页有旧数据时不触发，不会闪。
- **可访问性**：`Skeleton` 自身 `aria-hidden`，语义由 `LoadingRegion` 提供，避免读屏朗读大量空块。

**回滚：** 本计划按任务分 9 次原子 commit，任一任务出问题可 `git revert` 对应 commit，不影响其他任务。
