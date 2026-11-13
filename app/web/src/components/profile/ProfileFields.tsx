import {
	Field,
	ReadonlyRow,
	formInputClass,
} from "@/components/common/formPrimitives";

export type ProfileFieldsProps = {
	mode: "view" | "edit";
	displayName: string;
	onDisplayNameChange: (v: string) => void;
	onEnter: () => void;
	saving: boolean;
	showUsername: boolean;
	showRole: boolean;
	username: string;
	roleLabel: string;
	isCompact: boolean;
};

export default function ProfileFields({
	mode,
	displayName,
	onDisplayNameChange,
	onEnter,
	saving,
	showUsername,
	showRole,
	username,
	roleLabel,
	isCompact,
}: ProfileFieldsProps) {
	return (
		<div className="flex w-full flex-col gap-1.5">
			{showUsername && (
				<ReadonlyRow
					label="用户名"
					value={username || "—"}
					desc="用户名由系统分配，不可修改"
				/>
			)}

			{mode === "view" ? (
				<ReadonlyRow label="昵称" value={displayName || "—"} />
			) : (
				<Field label="昵称">
					<input
						type="text"
						className={formInputClass}
						value={displayName}
						placeholder="请输入昵称"
						maxLength={32}
						onChange={(e) => onDisplayNameChange(e.currentTarget.value)}
						onKeyDown={(e) => {
							if (e.key === "Enter" && !e.nativeEvent.isComposing && !saving) {
								e.preventDefault();
								onEnter();
							}
						}}
					/>
				</Field>
			)}

			{showRole && !isCompact && (
				<ReadonlyRow
					label="角色"
					value={roleLabel}
					desc="角色由管理员分配，不可修改"
				/>
			)}
		</div>
	);
}
