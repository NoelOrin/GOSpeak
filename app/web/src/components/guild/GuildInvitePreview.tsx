import type { Component } from "solid-js";
import { Show } from "solid-js";
import type { Guild } from "@/api/guild";
import GuildIcon from "./GuildIcon";

export type GuildInvitePreviewStatus =
	| "loading"
	| "error"
	| "ready"
	| "ready-with-error";

export interface GuildInviteAction {
	label: string;
	disabled: boolean;
	onClick: () => void;
}

export function getGuildInvitePreviewStatus(
	loading: boolean,
	error: string | null | undefined,
	guild: Guild | null | undefined,
): GuildInvitePreviewStatus {
	if (!guild && loading) return "loading";
	if (!guild && error) return "error";
	if (!guild) return "loading";
	return error ? "ready-with-error" : "ready";
}

export function getGuildInviteAction(
	guild: Guild | null | undefined,
	joined: boolean,
	joining: boolean,
	onConfirm: () => void,
): GuildInviteAction | null {
	if (!guild) return null;
	if (joined) {
		return {
			label: "进入服务器",
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

export interface GuildInvitePreviewProps {
	guild?: Guild | null;
	joined?: boolean;
	loading?: boolean;
	error?: string | null;
	joining?: boolean;
	onConfirm: () => void;
	onCancel?: () => void;
}

const GuildInvitePreview: Component<GuildInvitePreviewProps> = (props) => {
	const status = () =>
		getGuildInvitePreviewStatus(
			!!props.loading,
			props.error || null,
			props.guild,
		);
	const action = () =>
		getGuildInviteAction(
			props.guild,
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

			<Show when={props.guild}>
				{(guild) => (
					<div class="flex items-start gap-3 mb-5">
						<GuildIcon
							name={guild().name}
							iconUrl={guild().icon_url}
							class="shrink-0"
						/>
						<div class="min-w-0 flex-1">
							<div class="font-semibold">{guild().name}</div>
							<p class="text-sm text-base-content/60 mt-1">
								{guild().description || "暂无描述"}
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

export default GuildInvitePreview;
