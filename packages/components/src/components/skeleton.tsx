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
