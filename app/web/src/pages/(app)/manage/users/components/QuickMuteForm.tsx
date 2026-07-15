import Gavel from "lucide-solid/icons/gavel";
import { For, Show } from "solid-js";
import type { UserRow } from "./UsersTable";

export interface QuickMuteFormProps {
	users: UserRow[];
	userMap: Map<number, string>;
	muteUserId: number | "";
	muteDuration: number;
	mutePerm: boolean;
	muteReason: string;
	submitting: boolean;
	setMuteUserId: (v: number | "") => void;
	setMuteDuration: (v: number) => void;
	setMutePerm: (v: boolean) => void;
	setMuteReason: (v: string) => void;
	onSubmit: () => void;
}

export default function QuickMuteForm(props: QuickMuteFormProps) {
	return (
		<div>
			<div class="mb-3 flex items-center gap-2 font-semibold text-sm">
				<Gavel size={16} />
				<span>快速禁言</span>
				<Show when={props.muteUserId}>
					<span class="text-primary text-xs">
						目标:{" "}
						{props.userMap.get(props.muteUserId as number) ||
							`#${props.muteUserId}`}
					</span>
				</Show>
			</div>
			<div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
				<div class="form-control">
					<label class="label py-1" for="mute-user">
						<span class="label-text text-xs">用户</span>
					</label>
					<select
						id="mute-user"
						class="select select-bordered select-sm"
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
				</div>

				<div class="form-control">
					<label class="label py-1" for="None">
						<span class="label-text text-xs">类型</span>
					</label>
					<div id="None" class="flex items-center gap-3 pt-1">
						<label class="flex items-center gap-1.5 text-xs">
							<input
								type="radio"
								name="mute-type"
								class="radio radio-xs"
								checked={!props.mutePerm}
								onChange={() => props.setMutePerm(false)}
							/>
							定时
						</label>
						<label class="flex items-center gap-1.5 text-xs">
							<input
								type="radio"
								name="mute-type"
								class="radio radio-xs"
								checked={props.mutePerm}
								onChange={() => props.setMutePerm(true)}
							/>
							永久
						</label>
					</div>
				</div>

				<Show when={!props.mutePerm}>
					<div class="form-control">
						<label class="label py-1" for="mute-duration">
							<span class="label-text text-xs">时长（秒）</span>
						</label>
						<input
							id="mute-duration"
							type="number"
							class="input input-bordered input-sm"
							value={props.muteDuration}
							onInput={(e) =>
								props.setMuteDuration(Number(e.currentTarget.value) || 0)
							}
							min={1}
						/>
					</div>
				</Show>

				<div class="form-control">
					<label class="label py-1" for="mute-reason">
						<span class="label-text text-xs">原因</span>
					</label>
					<input
						id="mute-reason"
						type="text"
						class="input input-bordered input-sm"
						placeholder="违规发言"
						value={props.muteReason}
						onInput={(e) => props.setMuteReason(e.currentTarget.value)}
					/>
				</div>
			</div>

			<div class="mt-3 flex justify-end">
				<button
					type="button"
					class="btn btn-primary btn-sm gap-2"
					disabled={!props.muteUserId || props.submitting}
					onClick={props.onSubmit}
				>
					<Gavel size={15} />
					确认禁言
				</button>
			</div>
		</div>
	);
}
