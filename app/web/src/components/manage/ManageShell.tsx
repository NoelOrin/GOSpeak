import type { JSX } from "solid-js";
import { Show } from "solid-js";

/** 管理页根容器：由 manage 布局统一滚动，避免子页切换时整壳卸载跳闪 */
export function ManagePage(props: { children: JSX.Element; class?: string }) {
	// 默认 min-h-full 跟随外层滚动；需要卡片内滚动的页面请传 h-full min-h-0 overflow-hidden
	return (
		<div
			class={`manage-page flex w-full min-w-0 flex-col gap-5 p-4 md:p-5 ${props.class ?? "min-h-full"}`}
		>
			{props.children}
		</div>
	);
}

/** 页内加载占位：保留页头时使用，避免整页被 spinner 替换造成闪白 */
export function ManageLoading(props: { class?: string; label?: string }) {
	return (
		<div
			class={`flex min-h-40 flex-1 items-center justify-center py-12 ${props.class ?? ""}`}
			aria-busy="true"
			aria-live="polite"
		>
			<div class="flex flex-col items-center gap-2 text-base-content/50">
				<span class="loading loading-spinner loading-md" />
				<span class="text-xs">{props.label ?? "加载中..."}</span>
			</div>
		</div>
	);
}

/** 页头：图标 + 标题 + 副标题 + 右侧操作 */
export function ManageHeader(props: {
	icon?: JSX.Element;
	title: string;
	description?: string;
	actions?: JSX.Element;
}) {
	return (
		<div class="flex shrink-0 items-start justify-between gap-3">
			<div class="flex min-w-0 items-center gap-2.5">
				<Show when={props.icon}>
					<span class="flex size-9 shrink-0 items-center justify-center rounded-xl border border-base-300 bg-base-100 text-base-content/60">
						{props.icon}
					</span>
				</Show>
				<div class="min-w-0">
					<h3 class="text-lg font-bold leading-tight">{props.title}</h3>
					<Show when={props.description}>
						<p class="mt-0.5 text-xs text-base-content/50">
							{props.description}
						</p>
					</Show>
				</div>
			</div>
			<Show when={props.actions}>
				<div class="flex shrink-0 items-center gap-2">{props.actions}</div>
			</Show>
		</div>
	);
}

/** 白卡片分区 */
export function ManageSection(props: {
	title?: string;
	description?: string;
	actions?: JSX.Element;
	children: JSX.Element;
	class?: string;
	bodyClass?: string;
	padded?: boolean;
}) {
	const padded = () => props.padded !== false;
	return (
		<section
			class={`flex min-h-0 flex-col overflow-hidden rounded-2xl border border-base-300/80 bg-base-100 shadow-sm ${props.class ?? ""}`}
		>
			<Show when={props.title || props.actions}>
				<div
					class="flex shrink-0 flex-wrap items-start justify-between gap-3 border-b border-base-300/70 px-4 py-3 md:px-5"
					classList={{ "border-b-0": !props.children }}
				>
					<div class="min-w-0">
						<Show when={props.title}>
							<div class="text-sm font-semibold text-base-content">
								{props.title}
							</div>
						</Show>
						<Show when={props.description}>
							<p class="mt-0.5 text-xs text-base-content/50">
								{props.description}
							</p>
						</Show>
					</div>
					<Show when={props.actions}>
						<div class="flex shrink-0 items-center gap-2">{props.actions}</div>
					</Show>
				</div>
			</Show>
			<div
				class={`min-h-0 flex-1 ${padded() ? "p-4 md:p-5" : ""} ${props.bodyClass ?? ""}`}
			>
				{props.children}
			</div>
		</section>
	);
}

/** 中性提示条 */
export function ManageNotice(props: {
	children: JSX.Element;
	tone?: "neutral" | "warning" | "info";
	icon?: JSX.Element;
}) {
	const tone = () => props.tone ?? "neutral";
	return (
		<div
			class="flex items-start gap-2.5 rounded-2xl border px-4 py-3 text-sm text-base-content/70"
			classList={{
				"border-base-300/80 bg-base-100 shadow-sm": tone() === "neutral",
				"border-warning/20 bg-warning/8": tone() === "warning",
				"border-base-300/80 bg-base-200/40": tone() === "info",
			}}
		>
			<Show when={props.icon}>
				<span class="mt-0.5 shrink-0 text-base-content/45">{props.icon}</span>
			</Show>
			<div class="min-w-0 flex-1">{props.children}</div>
		</div>
	);
}

/** 统一白底标签，不做颜色区分 */
export function ManageTag(props: { children: JSX.Element; class?: string }) {
	return (
		<span
			class={`inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2 py-0.5 text-[11px] font-medium text-base-content/80 ${props.class ?? ""}`}
		>
			{props.children}
		</span>
	);
}

/** 表格头统一样式 */
export const manageTableHeadClass =
	"border-b border-base-300 bg-base-200/50 text-xs font-semibold tracking-wide text-base-content/70";

export const manageTableRowClass =
	"border-b border-base-200 text-base-content hover:bg-base-200/40";
