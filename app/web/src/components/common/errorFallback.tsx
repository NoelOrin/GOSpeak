import { Button } from "@gospeak/components";
import { Copy, House, RefreshCw, RotateCcw, TriangleAlert } from "lucide-react";
import { useState, type ReactNode } from "react";

const IS_DEV = import.meta.env.DEV;

/** component = 局部模块兜底；page = 整页兜底（含路由级错误）。 */
export type ErrorLevel = "component" | "page";

export interface ErrorFallbackProps {
	error: Error;
	level: ErrorLevel;
	reset: () => void;
	/** 边界标识，便于用户反馈时定位 */
	source?: string;
	componentStack?: string;
	title?: string;
	description?: string;
	extraActions?: ReactNode;
}

/** 生产环境不暴露堆栈，只给出可用于反馈的基本信息。 */
function buildDetailsText({
	error,
	source,
	componentStack,
}: Pick<ErrorFallbackProps, "error" | "source" | "componentStack">): string {
	const lines = [
		`时间: ${new Date().toLocaleString()}`,
		`来源: ${source ?? "unknown"}`,
		`错误: ${error.name}: ${error.message}`,
	];
	if (IS_DEV) {
		if (error.stack) lines.push(`\n堆栈:\n${error.stack}`);
		if (componentStack) lines.push(`\n组件栈:\n${componentStack}`);
	}
	return lines.join("\n");
}

export function ErrorFallback({
	error,
	level,
	reset,
	source,
	componentStack,
	title,
	description,
	extraActions,
}: ErrorFallbackProps) {
	const [copied, setCopied] = useState(false);
	const detailsText = buildDetailsText({ error, source, componentStack });
	const isPage = level === "page";

	const copyDetails = async () => {
		try {
			await navigator.clipboard.writeText(detailsText);
			setCopied(true);
			window.setTimeout(() => setCopied(false), 2000);
		} catch {
			// HTTP / 无权限环境下剪贴板不可用，用户仍可展开详情手动复制
		}
	};

	return (
		<div
			role="alert"
			className={
				isPage
					? "bg-background text-foreground flex min-h-[calc(100dvh-var(--mygo-titlebar-height,0px))] w-full flex-col items-center justify-center gap-5 px-6 py-10 text-center"
					: "border-border bg-card text-card-foreground flex h-full min-h-32 w-full flex-col items-center justify-center gap-3 rounded-lg border border-dashed p-5 text-center"
			}
		>
			<div className="bg-destructive/10 text-destructive flex size-11 items-center justify-center rounded-full">
				<TriangleAlert className="size-5" />
			</div>

			<div className="space-y-1.5">
				<p
					className={isPage ? "text-lg font-semibold" : "text-sm font-semibold"}
				>
					{title ?? (isPage ? "页面出错了" : "模块加载失败")}
				</p>
				<p className="text-muted-foreground max-w-md text-xs leading-5">
					{description ??
						(isPage
							? "页面渲染时遇到异常，错误已记录。你可以重试，或返回首页继续使用。"
							: "该区域暂时无法显示，页面其余功能仍可正常使用。")}
				</p>
			</div>

			<div className="flex flex-wrap items-center justify-center gap-2">
				<Button size="sm" onClick={reset}>
					<RotateCcw className="size-3.5" />
					重试
				</Button>
				{isPage && (
					<>
						<Button
							size="sm"
							variant="outline"
							onClick={() => window.location.reload()}
						>
							<RefreshCw className="size-3.5" />
							刷新页面
						</Button>
						<Button size="sm" variant="ghost" asChild>
							<a href="/">
								<House className="size-3.5" />
								返回首页
							</a>
						</Button>
					</>
				)}
				{extraActions}
			</div>

			<div className="flex w-full max-w-lg flex-col items-center gap-1.5">
				<details className="text-muted-foreground w-full text-left">
					<summary className="hover:text-foreground w-fit cursor-pointer text-xs">
						查看错误详情
					</summary>
					<pre className="bg-muted border-border mt-2 max-h-48 overflow-auto rounded-sm border p-3 text-2xs leading-relaxed whitespace-pre-wrap break-all">
						{detailsText}
					</pre>
				</details>
				<button
					type="button"
					className="text-muted-foreground hover:text-foreground inline-flex cursor-pointer items-center gap-1 text-xs"
					onClick={() => void copyDetails()}
				>
					<Copy className="size-3" />
					{copied ? "已复制" : "复制错误信息"}
				</button>
			</div>
		</div>
	);
}
