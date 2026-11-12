// biome-ignore-all lint/suspicious/noArrayIndexKey: 骨架占位行/块数量固定，无业务主键可用
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
