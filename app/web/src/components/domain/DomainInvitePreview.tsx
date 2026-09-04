import type { Component } from "solid-js";
import { Show } from "solid-js";
import type { Domain } from "@/api/domain";
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

const DomainInvitePreview: Component<DomainInvitePreviewProps> = (props) => {
	const status = () =>
		getDomainInvitePreviewStatus(
			!!props.loading,
			props.error || null,
			props.domain,
		);
	const action = () =>
		getDomainInviteAction(
			props.domain,
			!!props.joined,
			!!props.joining,
			props.onConfirm,
		);

	return (
		<div class="min-w-0">
			<Show when={status() === "loading"}>
				<div class="flex justify-center py-6">
					<span class="loading loading-spinner loading-md" />
				</div>
			</Show>

			<Show when={props.error}>
				<div role="alert" class="alert alert-error mb-4">
					<span>{props.error}</span>
				</div>
			</Show>

			<Show when={props.domain}>
				{(domain) => (
					<div class="flex items-start gap-3 mb-5">
						<DomainIcon
							name={domain().name}
							iconUrl={domain().icon_url}
							class="shrink-0"
						/>
						<div class="min-w-0 flex-1">
							<div class="font-semibold">{domain().name}</div>
							<p class="text-sm text-base-content/60 mt-1">
								{domain().description || "暂无描述"}
							</p>
						</div>
					</div>
				)}
			</Show>

			<Show when={action()}>
				<div class="modal-action">
					<Show when={props.onCancel}>
						<button
							type="button"
							class="btn"
							onClick={() => props.onCancel?.()}
						>
							取消
						</button>
					</Show>
					<button
						type="button"
						class="btn btn-primary"
						disabled={action()?.disabled}
						onClick={() => action()?.onClick()}
					>
						{action()?.label}
					</button>
				</div>
			</Show>
		</div>
	);
};

export default DomainInvitePreview;
