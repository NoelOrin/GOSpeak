import type { ErrorInfo } from "react";

/** 错误来源层级。用于日志过滤，也方便后续接 Sentry / 自建日志端点。 */
export type ErrorScope = "component" | "page" | "route" | "global";

export interface ErrorReportPayload {
	error: Error;
	scope: ErrorScope;
	/** 出错的边界名称、路由或全局事件来源 */
	source?: string;
	componentStack?: string;
	timestamp: number;
}

export type ErrorReporter = (payload: ErrorReportPayload) => void;

const reporters = new Set<ErrorReporter>();

/**
 * 同一个 Error 实例常常会被 React 边界和 window.onerror 各上报一次。
 * 这里记住已上报的作用域，并允许被更精确的来源「升级」重报。
 */
const reportedScopes = new WeakMap<object, ErrorScope>();

const SCOPE_PRIORITY: Record<ErrorScope, number> = {
	global: 0,
	page: 1,
	route: 1,
	component: 2,
};

/** 把 throw 出来的任意值收敛成 Error，避免 undefined / 字符串 /  plain object 让边界 UI 崩掉。 */
export function normalizeError(value: unknown): Error {
	if (value instanceof Error) return value;
	if (typeof value === "string") return new Error(value);
	if (value && typeof value === "object") {
		const candidate = value as { message?: unknown; name?: unknown };
		if (typeof candidate.message === "string" && candidate.message) {
			const error = new Error(candidate.message);
			if (typeof candidate.name === "string") error.name = candidate.name;
			return error;
		}
	}
	try {
		return new Error(`Unknown error: ${JSON.stringify(value)}`);
	} catch {
		return new Error("Unknown error");
	}
}

/**
 * 注册一个上报通道（例如 Sentry.captureException、fetch('/api/client-errors')）。
 * 返回注销函数；上报通道自身抛错不会影响应用。
 */
export function registerErrorReporter(reporter: ErrorReporter): () => void {
	reporters.add(reporter);
	return () => {
		reporters.delete(reporter);
	};
}

export interface ReportErrorInput {
	error: unknown;
	scope: ErrorScope;
	source?: string;
	componentStack?: string | null;
	info?: ErrorInfo | { componentStack?: string | null } | null;
}

/** 统一上报入口：控制台 + 所有已注册通道。返回归一化后的 Error。 */
export function reportError(input: ReportErrorInput): Error {
	const error = normalizeError(input.error);
	const previousScope = reportedScopes.get(error);
	const shouldReport =
		previousScope === undefined ||
		SCOPE_PRIORITY[input.scope] > SCOPE_PRIORITY[previousScope];
	if (!shouldReport) return error;

	reportedScopes.set(error, input.scope);
	const componentStack =
		input.componentStack ?? input.info?.componentStack ?? undefined;
	const payload: ErrorReportPayload = {
		error,
		scope: input.scope,
		source: input.source,
		componentStack: componentStack ?? undefined,
		timestamp: Date.now(),
	};

	console.error(
		`[GOSpeak:${payload.scope}]${payload.source ? ` ${payload.source}` : ""}`,
		error,
		componentStack ?? "",
	);
	for (const reporter of reporters) {
		try {
			reporter(payload);
		} catch (reporterError) {
			// 上报通道故障不能反过来把页面搞崩
			console.error("[GOSpeak] error reporter failed", reporterError);
		}
	}
	return error;
}

/** 用户可见的简短错误摘要（不含堆栈）。 */
export function errorSummary(error: unknown): string {
	const normalized = normalizeError(error);
	return `${normalized.name}: ${normalized.message}`;
}

/**
 * 安装全局兜底监听。
 *
 * React 错误边界捕获不到事件回调、定时器、Promise 等异步错误，
 * 这些错误只能靠 window 的 error / unhandledrejection 事件记录。
 * 不改变浏览器默认行为（不调用 preventDefault）。
 */
export function installGlobalErrorHandlers(): () => void {
	if (typeof window === "undefined") return () => {};

	const onError = (event: ErrorEvent) => {
		reportError({
			error: event.error ?? event.message,
			scope: "global",
			source: "window.error",
		});
	};
	const onUnhandledRejection = (event: PromiseRejectionEvent) => {
		reportError({
			error: event.reason,
			scope: "global",
			source: "window.unhandledrejection",
		});
	};

	window.addEventListener("error", onError);
	window.addEventListener("unhandledrejection", onUnhandledRejection);
	return () => {
		window.removeEventListener("error", onError);
		window.removeEventListener("unhandledrejection", onUnhandledRejection);
	};
}
