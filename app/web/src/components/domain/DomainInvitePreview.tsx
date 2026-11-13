import { Button, Skeleton } from "@gospeak/components";
import type { Domain } from "@/api/domain";
import { LoadingRegion } from "@/components/common/Skeletons";
import DomainIcon from "./DomainIcon";

export type DomainInvitePreviewStatus =
	| "loading"
	| "error"
	| "ready"
	| "ready-with-error";

export interface DomainInviteAction {
	label: string;
	disabled: boolean;
	onClick: () => void;
}

export function getDomainInvitePreviewStatus(
	loading: boolean,
	error: string | null | undefined,
	domain: Domain | null | undefined,
): DomainInvitePreviewStatus {
	if (!domain && loading) return "loading";
	if (!domain && error) return "error";
	if (!domain) return "loading";
	return error ? "ready-with-error" : "ready";
}

export function getDomainInviteAction(
	domain: Domain | null | undefined,
	joined: boolean,
	joining: boolean,
	onConfirm: () => void,
): DomainInviteAction | null {
	if (!domain) return null;
	if (joined) {
		return {
			label: "进入域",
			disabled: false,
			onClick: onConfirm,
		};
	}
	return {
		label: joining ? "加入中..." : "确认加入",
		disabled: joining,
		onClick: onConfirm,
	};
}

export interface DomainInvitePreviewProps {
	domain?: Domain | null;
	joined?: boolean;
	loading?: boolean;
	error?: string | null;
	joining?: boolean;
	onConfirm: () => void;
	onCancel?: () => void;
}

export default function DomainInvitePreview({
	domain,
	joined,
	loading,
	error,
	joining,
	onConfirm,
	onCancel,
}: DomainInvitePreviewProps) {
	const status = getDomainInvitePreviewStatus(!!loading, error || null, domain);
	const action = getDomainInviteAction(domain, !!joined, !!joining, onConfirm);

	return (
		<div className="min-w-0">
			{status === "loading" && (
				<LoadingRegion label="正在加载邀请信息">
					<div className="flex flex-col gap-3 py-2">
						<Skeleton className="h-6 w-32" />
						<Skeleton className="h-4 w-full" />
						<Skeleton className="h-4 w-2/3" />
						<Skeleton className="mt-2 h-9 w-full rounded-sm" />
					</div>
				</LoadingRegion>
			)}

			{error && (
				<div
					role="alert"
					className="border-destructive/50 bg-destructive/10 text-destructive mb-4 flex items-center gap-2 rounded-sm px-3 py-2 text-sm"
				>
					<span>{error}</span>
				</div>
			)}

			{domain && (
				<div className="mb-5 flex items-start gap-3">
					<DomainIcon
						name={domain.name}
						iconUrl={domain.icon_url}
						className="shrink-0"
					/>
					<div className="min-w-0 flex-1">
						<div className="font-semibold">{domain.name}</div>
						<p className="text-muted-foreground mt-1 text-sm">
							{domain.description || "暂无描述"}
						</p>
					</div>
				</div>
			)}

			{action && (
				<div className="flex justify-end gap-2">
					{onCancel && (
						<button
							type="button"
							className="border-border hover:bg-muted inline-flex h-9 cursor-pointer items-center rounded-sm border px-3 text-sm transition-colors"
							onClick={() => onCancel?.()}
						>
							取消
						</button>
					)}
					<Button disabled={action.disabled} onClick={() => action.onClick()}>
						{action.label}
					</Button>
				</div>
			)}
		</div>
	);
}
