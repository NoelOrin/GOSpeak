import Globe from "lucide-solid/icons/globe";
import Lock from "lucide-solid/icons/lock";
import Save from "lucide-solid/icons/save";
import { type Accessor, Show } from "solid-js";
import type { Domain } from "@/api/domain";

export interface DomainSettingsFormProps {
	canManage: boolean;
	domain?: Domain | null;
	name: Accessor<string>;
	setName: (value: string) => void;
	nameError: Accessor<string>;
	setNameError: (value: string) => void;
	description: Accessor<string>;
	setDescription: (value: string) => void;
	isPublic: Accessor<boolean>;
	setIsPublic: (value: boolean) => void;
	saving: Accessor<boolean>;
	formError: Accessor<string>;
	onSave: (event: Event) => void;
}

export function DomainSettingsForm(props: DomainSettingsFormProps) {
	return (
		<Show
			when={props.canManage}
			fallback={
				<div class="grid gap-3 text-sm">
					<div class="text-base-content/60">当前账号无编辑权限</div>
					<dl class="grid gap-2">
						<div>
							<dt class="text-xs text-base-content/50">域名称</dt>
							<dd>{props.domain?.name}</dd>
						</div>
						<div>
							<dt class="text-xs text-base-content/50">描述</dt>
							<dd>{props.domain?.description || "-"}</dd>
						</div>
						<div>
							<dt class="text-xs text-base-content/50">公开状态</dt>
							<dd>{props.domain?.is_public ? "公开" : "私有"}</dd>
						</div>
					</dl>
				</div>
			}
		>
			<form onSubmit={props.onSave} class="flex flex-col gap-4" novalidate>
				<label class="form-control">
					<span class="label">
						<span class="label-text text-xs font-medium text-base-content/70">
							域名称
						</span>
					</span>
					<input
						id="domain-name"
						class="input input-bordered input-sm"
						value={props.name()}
						maxLength={100}
						aria-invalid={!!props.nameError()}
						aria-describedby={
							props.nameError() ? "domain-name-error" : undefined
						}
						onInput={(e) => {
							props.setName(e.currentTarget.value);
							if (e.currentTarget.value.trim()) {
								props.setNameError("");
							}
						}}
					/>
					<Show when={props.nameError()}>
						<span id="domain-name-error" class="mt-1 text-xs text-error">
							{props.nameError()}
						</span>
					</Show>
				</label>
				<label class="form-control">
					<span class="label">
						<span class="label-text text-xs font-medium text-base-content/70">
							描述
						</span>
					</span>
					<textarea
						id="domain-description"
						class="textarea textarea-bordered textarea-sm min-h-24"
						value={props.description()}
						onInput={(e) => props.setDescription(e.currentTarget.value)}
					/>
				</label>
				<label class="flex items-center justify-between rounded-lg border border-base-300 px-3 py-2.5 cursor-pointer">
					<span class="flex items-center gap-2 text-sm">
						{props.isPublic() ? (
							<Globe size={16} class="text-success" />
						) : (
							<Lock size={16} class="text-base-content/50" />
						)}
						公开域
					</span>
					<input
						type="checkbox"
						class="toggle toggle-primary toggle-sm"
						checked={props.isPublic()}
						disabled={props.saving()}
						onChange={(e) => props.setIsPublic(e.currentTarget.checked)}
					/>
				</label>
				<Show when={props.formError()}>
					<p role="alert" class="text-xs text-error">
						{props.formError()}
					</p>
				</Show>
				<button
					type="submit"
					class="btn btn-primary btn-sm"
					disabled={props.saving()}
				>
					{props.saving() ? (
						<>
							<span class="loading loading-spinner loading-xs" />
							保存中...
						</>
					) : (
						<>
							<Save size={15} />
							保存设置
						</>
					)}
				</button>
			</form>
		</Show>
	);
}
