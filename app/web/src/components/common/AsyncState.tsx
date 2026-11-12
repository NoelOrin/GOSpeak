import { Button } from "@gospeak/components";
import { Loader2, RefreshCw } from "lucide-react";
import type { ReactNode } from "react";
import { LoadingRegion } from "@/components/common/Skeletons";

export interface AsyncStateProps {
	loading?: boolean;
	/** 非空的错误文案；有值即渲染错误态 */
	error?: string | null;
	/** 数据为空（仅在非 loading / 非 error 时生效） */
	empty?: boolean;
	loadingText?: ReactNode;
	emptyText?: ReactNode;
	retryText?: string;
	onRetry?: () => void;
	className?: string;
	/** 紧凑模式：用于表格内、列表内等小空间 */
	compact?: boolean;
	/** 加载态的骨架屏；提供时替代默认 spinner 文案区 */
	skeleton?: ReactNode;
	children?: ReactNode;
}

/**
 * 异步内容的三态容器：加载中 / 失败 / 空 / 内容。
 *
 * 存在的理由：项目里大量面板在 loading 时直接渲染"暂无数据"，
 * 用户分不清"还没有"和"正在加载"和"加载失败了"——这三件事必须长得不一样。
 */
export default function AsyncState({
	loading,
	error,
	empty,
	loadingText = "加载中…",
	emptyText = "暂无数据",
	retryText = "重试",
	onRetry,
	className,
	compact,
	skeleton,
	children,
}: AsyncStateProps) {
	const box = compact
		? "flex w-full flex-col items-center justify-center gap-1.5 px-3 py-4 text-xs"
		: "flex h-full min-h-24 w-full flex-col items-center justify-center gap-2 px-4 py-6 text-sm";

	if (loading) {
		if (skeleton) {
			return (
				<LoadingRegion
					label={typeof loadingText === "string" ? loadingText : "加载中"}
					className={className}
				>
					{skeleton}
				</LoadingRegion>
			);
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

	if (error) {
		return (
			<div
				className={`${box} text-muted-foreground ${className ?? ""}`}
				role="alert"
			>
				<span className="text-destructive text-center">{error}</span>
				{onRetry && (
					<Button variant="outline" size="xs" onClick={onRetry}>
						<RefreshCw className="size-3.5" />
						{retryText}
					</Button>
				)}
			</div>
		);
	}

	if (empty) {
		return (
			<div className={`${box} text-faint-foreground ${className ?? ""}`}>
				<span className="text-center">{emptyText}</span>
			</div>
		);
	}

	return <>{children}</>;
}
