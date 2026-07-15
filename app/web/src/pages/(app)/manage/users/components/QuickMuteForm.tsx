import Gavel from "lucide-solid/icons/gavel";
import { For, Show } from "solid-js";
import { ManageSection } from "@/components/manage/ManageShell";
import MuteDurationPicker from "@/components/manage/MuteDurationPicker";
import type { UserRow } from "./UsersTable";

export interface QuickMuteFormProps {
	users: UserRow[];
	userMap: Map<number, string>;
	muteUserId: number | "";
	muteDuration: number;
	mutePerm: boolean;
	muteReason: string;
	submitting: boolean;
	setMuteUserId: (value: number | "") => void;
	setMuteDuration: (value: number) => void;
	setMutePerm: (value: boolean) => void;
	setMuteReason: (value: string) => void;
	onSubmit: () => void;
}

export default function QuickMuteForm(props: QuickMuteFormProps) {
	const selectedLabel = () => {
		if (!props.muteUserId) return "";
		const u = props.users.find((item) => item.id === props.muteUserId);
		if (!u)
			return props.userMap.get(props.muteUserId) || `#${props.muteUserId}`;
		return `${u.display_name || u.name} (${u.name})`;
	};

	return (
		<ManageSection
			title="快速禁言"
			description="选择用户与时长后立即生效"
			actions={
				<button
					type="button"
					class="btn btn-sm gap-2 border border-base-300 bg-base-100 text-base-content shadow-none hover:bg-base-200"
					disabled={!props.muteUserId || props.submitting}
					onClick={props.onSubmit}
				>
					<Show when={props.submitting} fallback={<Gavel size={15} />}>
						<span class="loading loading-spinner loading-xs" />
					</Show>
					确认禁言
				</button>
			}
		>
			<div class="grid grid-cols-1 gap-4 xl:grid-cols-[220px_minmax(0,1fr)_240px]">
				<div class="form-control">
					<label class="label py-1" for="mute-user">
						<span class="label-text text-xs font-medium text-base-content/70">
							用户
						</span>
					</label>
					<select
						id="mute-user"
						class="select select-bordered select-sm w-full bg-base-100"
						value={props.muteUserId}
						onChange={(e) =>
							props.setMuteUserId(
								e.currentTarget.value ? Number(e.currentTarget.value) : "",
							)
						}
					>
						<option value="">选择用户</option>
						<For each={props.users}>
							{(u) => (
								<option value={u.id}>
									{u.display_name || u.name} ({u.name})
								</option>
							)}
						</For>
					</select>
					<div class="mt-3 rounded-xl border border-base-300 bg-base-200/25 px-3 py-2 text-xs text-base-content/65">
						{props.muteUserId
							? `将禁言：${selectedLabel()}`
							: "请先选择要禁言的用户"}
					</div>
				</div>

				<div class="form-control min-w-0">
					<div class="label py-1">
						<span class="label-text text-xs font-medium text-base-content/70">
							禁言时长
						</span>
					</div>
					<MuteDurationPicker
						permanent={props.mutePerm}
						duration={props.muteDuration}
						onChange={(value) => {
							props.setMutePerm(value.permanent);
							props.setMuteDuration(value.duration);
						}}
					/>
				</div>

				<div class="form-control">
					<label class="label py-1" for="mute-reason">
						<span class="label-text text-xs font-medium text-base-content/70">
							原因
						</span>
					</label>
					<input
						id="mute-reason"
						type="text"
						class="input input-bordered input-sm w-full bg-base-100 placeholder:text-base-content/40"
						placeholder="违规发言"
						value={props.muteReason}
						onInput={(e) => props.setMuteReason(e.currentTarget.value)}
					/>
				</div>
			</div>
		</ManageSection>
	);
}
