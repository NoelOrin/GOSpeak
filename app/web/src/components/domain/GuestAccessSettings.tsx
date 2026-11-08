import { Switch } from "@gospeak/components";
import { useEffect, useState } from "react";
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

function ToggleRow({ label, hint, value, disabled, onChange }: ToggleRowProps) {
	return (
		<label className="flex cursor-pointer items-center justify-between gap-4 py-1">
			<span>
				<span className="block text-sm">{label}</span>
				{hint && (
					<span className="text-faint-foreground block text-xs">{hint}</span>
				)}
			</span>
			<Switch checked={value} disabled={disabled} onCheckedChange={onChange} />
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

export default function GuestAccessSettings({
	config,
	loading,
	canManage,
	saving,
	onSave,
}: GuestAccessSettingsProps) {
	const [allowGuest, setAllowGuest] = useState(false);
	const [canListen, setCanListen] = useState(true);
	const [canSpeak, setCanSpeak] = useState(true);
	const [canMessage, setCanMessage] = useState(false);
	const [limit, setLimit] = useState(50);

	useEffect(() => {
		if (!config) return;
		setAllowGuest(!!config.allow_guest);
		setCanListen(config.guest_can_listen ?? true);
		setCanSpeak(config.guest_can_speak ?? true);
		setCanMessage(config.guest_can_message ?? false);
		setLimit(config.guest_limit ?? 50);
	}, [config]);

	function collect() {
		const current = config;
		if (!current) return;
		const patch = buildGuestConfigPatch(current, {
			allow_guest: allowGuest,
			guest_can_listen: canListen,
			guest_can_speak: canSpeak,
			guest_can_message: canMessage,
			guest_limit: limit,
		});
		if (Object.keys(patch).length === 0) return;
		onSave(patch);
	}

	if (loading) {
		return (
			<div className="flex justify-center py-4">
				<span className="border-primary border-t-primary size-5 animate-spin rounded-full border-2" />
			</div>
		);
	}

	return (
		<div className="flex flex-col gap-2">
			<ToggleRow
				label="允许访客进入"
				hint="关闭后新访客无法通过邀请码或公开入口加入"
				value={allowGuest}
				disabled={!canManage || saving}
				onChange={setAllowGuest}
			/>
			<div
				className={`flex flex-col gap-1 border-l pl-2 ${!allowGuest ? "pointer-events-none opacity-50" : ""}`}
			>
				<ToggleRow
					label="访客可旁听"
					value={canListen}
					disabled={!canManage || saving || !allowGuest}
					onChange={setCanListen}
				/>
				<ToggleRow
					label="访客可发言"
					value={canSpeak}
					disabled={!canManage || saving || !allowGuest}
					onChange={setCanSpeak}
				/>
				<ToggleRow
					label="访客可发消息"
					value={canMessage}
					disabled={!canManage || saving || !allowGuest}
					onChange={setCanMessage}
				/>
				<label className="flex items-center justify-between gap-4 py-1">
					<span className="text-sm">在线访客上限（0 为不限）</span>
					<input
						type="number"
						min={0}
						className="border-input bg-card text-foreground focus-visible:ring-ring flex h-9 w-24 rounded-sm border px-2 text-sm focus-visible:outline-none focus-visible:ring-2"
						value={limit}
						disabled={!canManage || saving || !allowGuest}
						onChange={(e) => setLimit(Number(e.currentTarget.value) || 0)}
					/>
				</label>
			</div>
			{canManage && (
				<button
					type="button"
					className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-9 cursor-pointer items-center self-end rounded-sm px-3 text-sm transition-opacity disabled:pointer-events-none disabled:opacity-50"
					disabled={saving}
					onClick={collect}
				>
					保存配置
				</button>
			)}
		</div>
	);
}
