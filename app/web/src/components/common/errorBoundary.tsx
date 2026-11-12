import type { ErrorComponentProps } from "@tanstack/react-router";
import {
	Component,
	Fragment,
	useEffect,
	useRef,
	type ComponentType,
	type ErrorInfo,
	type ReactNode,
} from "react";
import {
	ErrorFallback,
	type ErrorLevel,
} from "@/components/common/errorFallback";
import { normalizeError, reportError } from "@/utils/errorReport";

export interface ErrorFallbackRenderProps {
	error: Error;
	reset: () => void;
	level: ErrorLevel;
	source?: string;
	componentStack?: string;
}

export interface ErrorBoundaryProps {
	children: ReactNode;
	/** 边界标识，写进上报日志，建议按模块命名，如 "room-detail" */
	name?: string;
	/** component（默认）：只兜住局部模块；page：整页兜底 */
	level?: ErrorLevel;
	/** 自定义兜底 UI；函数形式可以拿到 error / reset 自行渲染 */
	fallback?: ReactNode | ((props: ErrorFallbackRenderProps) => ReactNode);
	title?: string;
	description?: string;
	/** 这些依赖变化时自动恢复，例如路由参数、房间 id、tab */
	resetKeys?: readonly unknown[];
	/** 重试前回调，可用来清 react-query 缓存或重置 store */
	onReset?: () => void;
	onError?: (error: Error, info: ErrorInfo) => void;
	/** 传 false 可跳过上报（例如已知且可忽略的错误） */
	report?: boolean;
}

interface ErrorBoundaryState {
	error: Error | null;
	componentStack: string | undefined;
	/** 每次 reset 递增，作为 Fragment 的 key 强制子树重新挂载 */
	resetCount: number;
}

function sameResetKeys(
	a: readonly unknown[] | undefined,
	b: readonly unknown[] | undefined,
): boolean {
	if (a === b) return true;
	if (!a || !b || a.length !== b.length) return false;
	return a.every((value, index) => Object.is(value, b[index]));
}

/**
 * 组件级错误边界。
 *
 * 用法：<ErrorBoundary name="member-list"><MemberList /></ErrorBoundary>
 * 只影响被包裹的子树；fallback 自带「重试」，重试会重新挂载整棵子树。
 */
export class ErrorBoundary extends Component<
	ErrorBoundaryProps,
	ErrorBoundaryState
> {
	state: ErrorBoundaryState = {
		error: null,
		componentStack: undefined,
		resetCount: 0,
	};

	static getDerivedStateFromError(error: unknown): Partial<ErrorBoundaryState> {
		return { error: normalizeError(error) };
	}

	componentDidCatch(error: unknown, info: ErrorInfo): void {
		const normalized = normalizeError(error);
		const componentStack = info.componentStack ?? undefined;
		// componentStack 只在 componentDidCatch 里拿得到，补存到 state 供详情展示
		this.setState({ componentStack });
		if (this.props.report !== false) {
			reportError({
				error: normalized,
				scope: this.props.level === "page" ? "page" : "component",
				source: this.props.name,
				componentStack,
			});
		}
		this.props.onError?.(normalized, info);
	}

	componentDidUpdate(prevProps: ErrorBoundaryProps): void {
		if (
			this.state.error !== null &&
			!sameResetKeys(prevProps.resetKeys, this.props.resetKeys)
		) {
			this.reset();
		}
	}

	reset = (): void => {
		this.props.onReset?.();
		this.setState((prev) => ({
			error: null,
			componentStack: undefined,
			resetCount: prev.resetCount + 1,
		}));
	};

	render(): ReactNode {
		const { error, componentStack, resetCount } = this.state;
		const {
			children,
			fallback,
			level = "component",
			name,
			title,
			description,
		} = this.props;

		if (error !== null) {
			const fallbackProps: ErrorFallbackRenderProps = {
				error,
				reset: this.reset,
				level,
				source: name,
				componentStack,
			};
			if (typeof fallback === "function") return fallback(fallbackProps);
			if (fallback !== undefined) return fallback;
			return (
				<ErrorFallback
					{...fallbackProps}
					title={title}
					description={description}
				/>
			);
		}

		return <Fragment key={resetCount}>{children}</Fragment>;
	}
}

/** 页面级边界：包住整页内容，出错时展示全屏兜底页而不是白屏。 */
export function PageErrorBoundary(
	props: Omit<ErrorBoundaryProps, "level">,
): ReactNode {
	return <ErrorBoundary {...props} level="page" />;
}

export interface RouteErrorComponentProps extends ErrorComponentProps {
	/**
	 * 重试前的额外动作。路由 loader 抛错时单靠 reset 不会重新请求，
	 * 需要由 router 注入 invalidate。
	 */
	onRetry?: () => void;
	title?: string;
	description?: string;
}

/**
 * 页面级路由错误组件，供 router 的 defaultErrorComponent 与
 * 根路由 errorComponent 使用。覆盖渲染错误、loader 错误、beforeLoad 错误。
 */
export function RouteErrorComponent({
	error,
	info,
	reset,
	onRetry,
	title,
	description,
}: RouteErrorComponentProps): ReactNode {
	const lastReported = useRef<unknown>(undefined);
	useEffect(() => {
		if (lastReported.current === error) return;
		lastReported.current = error;
		reportError({
			error,
			scope: "route",
			source: "route",
			componentStack: info?.componentStack,
		});
	}, [error, info]);

	const handleReset = () => {
		onRetry?.();
		reset();
	};

	return (
		<ErrorFallback
			level="page"
			error={normalizeError(error)}
			reset={handleReset}
			source="route"
			componentStack={info?.componentStack ?? undefined}
			title={title}
			description={description}
		/>
	);
}

/** 给已有组件套一层组件级边界，适合列表项、面板等重复出现的高风险模块。 */
export function withErrorBoundary<P extends object>(
	WrappedComponent: ComponentType<P>,
	boundaryProps?: Omit<ErrorBoundaryProps, "children">,
): ComponentType<P> {
	const displayName = WrappedComponent.displayName ?? "Component";
	function WithErrorBoundary(props: P): ReactNode {
		return (
			<ErrorBoundary {...boundaryProps}>
				<WrappedComponent {...props} />
			</ErrorBoundary>
		);
	}
	WithErrorBoundary.displayName = `withErrorBoundary(${displayName})`;
	return WithErrorBoundary;
}
