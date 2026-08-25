import { createEffect, createSignal, Show } from "solid-js";
import type { GuestDomain } from "@/api/guest";

export interface GuestConfigPatch {
	allow_guest?: boolean;
	guest_can_listen?: boolean;
	guest_can_speak?: boolean;
	guest_can_message?: boolean;
	guest_limit?: number;
}

interface ToggleRowProps {
	label: string;
	hint?: string;
	value: boolean;
	disabled: boolean;
	onChange: (v: boolean) => void;
}

function ToggleRow(props: ToggleRowProps) {
	return (
		<label class="flex items-center justify-between gap-4 py-1">
			<span>
				<span class="block text-sm">{props.label}</span>
				<Show when={props.hint}>
					<span class="block text-xs text-base-content/50">{props.hint}</span>
				</Show>
			</span>
			<input
				type="checkbox"
				class="toggle toggle-primary"
				checked={props.value}
				disabled={props.disabled}
				onChange={(e) => props.onChange(e.currentTarget.checked)}
			/>
		</label>
	);
}

interface GuestAccessSettingsProps {
	config: GuestDomain | null;
	loading: boolean;
	canManage: boolean;
	saving: boolean;
	onSave: (patch: GuestConfigPatch) => void;
}

export function buildGuestConfigPatch(
	current: GuestDomain,
	next: {
		allow_guest: boolean;
		guest_can_listen: boolean;
		guest_can_speak: boolean;
		guest_can_message: boolean;
		guest_limit: number;
	},
): GuestConfigPatch {
	const patch: GuestConfigPatch = {};
	if (current.allow_guest !== next.allow_guest)
		patch.allow_guest = next.allow_guest;
	if (current.guest_can_listen !== next.guest_can_listen)
		patch.guest_can_listen = next.guest_can_listen;
	if (current.guest_can_speak !== next.guest_can_speak)
		patch.guest_can_speak = next.guest_can_speak;
	if (current.guest_can_message !== next.guest_can_message)
		patch.guest_can_message = next.guest_can_message;
	if ((current.guest_limit ?? 0) !== next.guest_limit)
		patch.guest_limit = next.guest_limit;
	return patch;
}

const GuestAccessSettings = (props: GuestAccessSettingsProps) => {
	const [allowGuest, setAllowGuest] = createSignal(false);
	const [canListen, setCanListen] = createSignal(true);
	const [canSpeak, setCanSpeak] = createSignal(true);
	const [canMessage, setCanMessage] = createSignal(false);
	const [limit, setLimit] = createSignal(50);

	createEffect(() => {
		const cfg = props.config;
		if (!cfg) return;
		setAllowGuest(!!cfg.allow_guest);
		setCanListen(cfg.guest_can_listen ?? true);
		setCanSpeak(cfg.guest_can_speak ?? true);
		setCanMessage(cfg.guest_can_message ?? false);
		setLimit(cfg.guest_limit ?? 50);
	});

	function collect() {
		const current = props.config;
		if (!current) return;
		const patch = buildGuestConfigPatch(current, {
			allow_guest: allowGuest(),
			guest_can_listen: canListen(),
			guest_can_speak: canSpeak(),
			guest_can_message: canMessage(),
			guest_limit: limit(),
		});
		if (Object.keys(patch).length === 0) return;
		props.onSave(patch);
	}

	return (
		<div class="flex flex-col gap-2">
			<Show
				when={!props.loading}
				fallback={
					<div class="flex justify-center py-4">
						<span class="loading loading-spinner loading-sm" />
					</div>
				}
			>
				<ToggleRow
					label="允许访客进入"
					hint="关闭后新访客无法通过邀请码或公开入口加入"
					value={allowGuest()}
					disabled={!props.canManage || props.saving}
					onChange={setAllowGuest}
				/>
				<div
					class="flex flex-col gap-1 pl-2 border-l"
					classList={{
						"opacity-50 pointer-events-none": !allowGuest(),
					}}
				>
					<ToggleRow
						label="访客可旁听"
						value={canListen()}
						disabled={!props.canManage || props.saving || !allowGuest()}
						onChange={setCanListen}
					/>
					<ToggleRow
						label="访客可发言"
						value={canSpeak()}
						disabled={!props.canManage || props.saving || !allowGuest()}
						onChange={setCanSpeak}
					/>
					<ToggleRow
						label="访客可发消息"
						value={canMessage()}
						disabled={!props.canManage || props.saving || !allowGuest()}
						onChange={setCanMessage}
					/>
					<label class="flex items-center justify-between gap-4 py-1">
						<span class="text-sm">在线访客上限（0 为不限）</span>
						<input
							type="number"
							min={0}
							class="input input-bordered input-sm w-24"
							value={limit()}
							disabled={!props.canManage || props.saving || !allowGuest()}
							onInput={(e) => setLimit(Number(e.currentTarget.value) || 0)}
						/>
					</label>
				</div>
				<Show when={props.canManage}>
					<button
						type="button"
						class="btn btn-primary btn-sm self-end"
						disabled={props.saving}
						onClick={collect}
					>
						保存配置
					</button>
				</Show>
			</Show>
		</div>
	);
};

export default GuestAccessSettings;
