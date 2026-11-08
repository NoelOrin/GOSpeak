import { Switch } from "@gospeak/components";
import { Globe, Lock, Save } from "lucide-react";
import type { Domain } from "@/api/domain";

export interface DomainSettingsFormProps {
	canManage: boolean;
	domain?: Domain | null;
	name: string;
	setName: (value: string) => void;
	nameError: string;
	setNameError: (value: string) => void;
	description: string;
	setDescription: (value: string) => void;
	isPublic: boolean;
	setIsPublic: (value: boolean) => void;
	saving: boolean;
	formError: string;
	onSave: (event: React.FormEvent) => void;
}

const inputClass =
	"border-input bg-card text-foreground focus-visible:ring-ring flex h-9 w-full rounded-sm border px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2";

export function DomainSettingsForm(props: DomainSettingsFormProps) {
	if (!props.canManage) {
		return (
			<div className="grid gap-3 text-sm">
				<div className="text-muted-foreground">当前账号无编辑权限</div>
				<dl className="grid gap-2">
					<div>
						<dt className="text-faint-foreground text-xs">域名称</dt>
						<dd>{props.domain?.name}</dd>
					</div>
					<div>
						<dt className="text-faint-foreground text-xs">描述</dt>
						<dd>{props.domain?.description || "-"}</dd>
					</div>
					<div>
						<dt className="text-faint-foreground text-xs">公开状态</dt>
						<dd>{props.domain?.is_public ? "公开" : "私有"}</dd>
					</div>
				</dl>
			</div>
		);
	}

	return (
		<form onSubmit={props.onSave} className="flex flex-col gap-4" noValidate>
			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">
					域名称
				</span>
				<input
					id="domain-name"
					className={inputClass}
					value={props.name}
					maxLength={100}
					aria-invalid={!!props.nameError}
					aria-describedby={props.nameError ? "domain-name-error" : undefined}
					onChange={(e) => {
						props.setName(e.currentTarget.value);
						if (e.currentTarget.value.trim()) {
							props.setNameError("");
						}
					}}
				/>
				{props.nameError && (
					<span
						id="domain-name-error"
						className="text-destructive mt-1 block text-xs"
					>
						{props.nameError}
					</span>
				)}
			</label>
			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">
					描述
				</span>
				<textarea
					id="domain-description"
					className="border-input bg-card text-foreground focus-visible:ring-ring min-h-24 w-full rounded-sm border px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2"
					value={props.description}
					onChange={(e) => props.setDescription(e.currentTarget.value)}
				/>
			</label>
			<label className="border-border flex cursor-pointer items-center justify-between rounded-lg border px-3 py-2.5">
				<span className="flex items-center gap-2 text-sm">
					{props.isPublic ? (
						<Globe size={16} className="text-success" />
					) : (
						<Lock size={16} className="text-faint-foreground" />
					)}
					公开域
				</span>
				<Switch
					checked={props.isPublic}
					disabled={props.saving}
					onCheckedChange={props.setIsPublic}
				/>
			</label>
			{props.formError && (
				<p role="alert" className="text-destructive text-xs">
					{props.formError}
				</p>
			)}
			<button
				type="submit"
				className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-9 cursor-pointer items-center gap-1.5 self-start rounded-sm px-3 text-sm transition-opacity disabled:pointer-events-none disabled:opacity-50"
				disabled={props.saving}
			>
				<Save size={16} />
				保存设置
			</button>
		</form>
	);
}
