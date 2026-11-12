import { Loader2 } from "lucide-react";
import type { ReactNode } from "react";

/** 管理页根容器：由 manage 布局统一滚动，避免子页切换时整壳卸载跳闪 */
export function ManagePage({
	children,
	className,
}: {
	children: ReactNode;
	className?: string;
}) {
	return (
		<div
			className={`flex w-full min-w-0 flex-col gap-5 p-4 md:p-5 ${className ?? "min-h-full"}`}
		>
			{children}
		</div>
	);
}

/** 页内加载占位：保留页头时使用，避免整页被 spinner 替换造成闪白 */
export function ManageLoading({
	className,
	label,
}: {
	className?: string;
	label?: string;
}) {
	return (
		<div
			className={`text-faint-foreground flex min-h-40 flex-1 items-center justify-center py-12 ${className ?? ""}`}
			aria-busy="true"
			aria-live="polite"
		>
			<div className="flex flex-col items-center gap-2">
				<Loader2 className="size-6 animate-spin" />
				<span className="text-xs">{label ?? "加载中..."}</span>
			</div>
		</div>
	);
}

/** 页头：图标 + 标题 + 副标题 + 右侧操作 */
export function ManageHeader({
	icon,
	title,
	description,
	actions,
}: {
	icon?: ReactNode;
	title: string;
	description?: string;
	actions?: ReactNode;
}) {
	return (
		<div className="flex shrink-0 items-start justify-between gap-3">
			<div className="flex min-w-0 items-center gap-2.5">
				{icon && (
					<span className="border-border bg-card text-muted-foreground flex size-9 shrink-0 items-center justify-center rounded-lg border">
						{icon}
					</span>
				)}
				<div className="min-w-0">
					<h3 className="text-lg leading-tight font-bold">{title}</h3>
					{description && (
						<p className="text-faint-foreground mt-0.5 text-xs">
							{description}
						</p>
					)}
				</div>
			</div>
			{actions && (
				<div className="flex shrink-0 items-center gap-2">{actions}</div>
			)}
		</div>
	);
}

/** 白卡片分区 */
export function ManageSection({
	title,
	description,
	actions,
	children,
	className,
	bodyClass,
	padded = true,
}: {
	title?: string;
	description?: string;
	actions?: ReactNode;
	children?: ReactNode;
	className?: string;
	bodyClass?: string;
	padded?: boolean;
}) {
	return (
		<section
			className={`border-border/80 bg-card flex min-h-0 flex-col overflow-hidden rounded-xl border shadow-sm ${className ?? ""}`}
		>
			{(title || actions) && (
				<div
					className={`border-border/70 flex shrink-0 flex-wrap items-start justify-between gap-3 border-b px-4 py-3 md:px-5 ${!children ? "border-b-0" : ""}`}
				>
					<div className="min-w-0">
						{title && (
							<div className="text-foreground text-sm font-semibold">
								{title}
							</div>
						)}
						{description && (
							<p className="text-faint-foreground mt-0.5 text-xs">
								{description}
							</p>
						)}
					</div>
					{actions && (
						<div className="flex shrink-0 items-center gap-2">{actions}</div>
					)}
				</div>
			)}
			<div
				className={`min-h-0 flex-1 ${padded ? "p-4 md:p-5" : ""} ${bodyClass ?? ""}`}
			>
				{children}
			</div>
		</section>
	);
}

/** 中性提示条 */
export function ManageNotice({
	children,
	tone = "neutral",
	icon,
}: {
	children: ReactNode;
	tone?: "neutral" | "warning" | "info";
	icon?: ReactNode;
}) {
	const toneClass =
		tone === "warning"
			? "border-warning/20 bg-warning/10"
			: tone === "info"
				? "border-border/80 bg-muted/40"
				: "border-border/80 bg-card shadow-sm";
	return (
		<div
			className={`text-muted-foreground flex items-start gap-2.5 rounded-xl border px-4 py-3 text-sm ${toneClass}`}
		>
			{icon && (
				<span className="text-faint-foreground mt-0.5 shrink-0">{icon}</span>
			)}
			<div className="min-w-0 flex-1">{children}</div>
		</div>
	);
}

/** 统一白底标签，不做颜色区分 */
export function ManageTag({
	children,
	className,
}: {
	children: ReactNode;
	className?: string;
}) {
	return (
		<span
			className={`border-border bg-card text-foreground inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium ${className ?? ""}`}
		>
			{children}
		</span>
	);
}

/** 表格头统一样式 */
export const manageTableHeadClass =
	"border-border bg-muted/50 text-muted-foreground border-b text-xs font-semibold tracking-wide";

export const manageTableRowClass =
	"border-border text-foreground hover:bg-muted/40 border-b";
