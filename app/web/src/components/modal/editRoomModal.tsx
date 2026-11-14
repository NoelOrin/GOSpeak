import { Dialog, DialogContent, DialogTitle, toast } from "@gospeak/components";
import { X } from "lucide-react";
import type { RoomRecord } from "@/api/room";
import { updateRoom } from "@/api/room";
import { Form, type FormFieldConfig } from "@/components/form";
import {
	editRoomLimitSchema,
	fieldError,
	roomDescriptionSchema,
	roomNameSchema,
} from "@/utils/formSchemas";
import { toastActionError } from "@/utils/toast";

interface EditRoomModalProps {
	open: boolean;
	room: RoomRecord;
	onClose: () => void;
	onSaved: (room: RoomRecord) => void | Promise<void>;
}

export default function EditRoomModal({
	open,
	room,
	onClose,
	onSaved,
}: EditRoomModalProps) {
	const fields: FormFieldConfig[] = [
		{
			name: "name",
			label: "房间名称",
			type: "text",
			placeholder: "请输入房间名称",
			required: true,
			validation: (v: string) => fieldError(roomNameSchema, v),
		},
		{
			name: "limit",
			label: "人数上限",
			type: "number",
			required: true,
			validation: (v: number) => fieldError(editRoomLimitSchema, v),
		},
		{ name: "audio_only", label: "仅语音模式", type: "switch" },
		{ name: "allow_audience", label: "允许听众加入", type: "switch" },
		{
			name: "description",
			label: "房间说明",
			type: "textarea",
			placeholder: "选填，用于说明当前房间的主题或规则",
			validation: (v: string) => fieldError(roomDescriptionSchema, v),
		},
	];

	async function handleSubmit(formValues: Record<string, any>) {
		try {
			const updated = await updateRoom({
				id: room.id,
				name: formValues.name.trim(),
				description: formValues.description.trim(),
				limit: formValues.limit,
				audio_only: formValues.audio_only,
				allow_audience: formValues.allow_audience,
			});
			await onSaved(updated);
			toast.success(`已更新房间: ${updated.name}`);
			onClose();
		} catch (error) {
			toastActionError(error, "更新房间失败");
		}
	}

	if (!open) return null;

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			<DialogContent
				showClose={false}
				className="max-w-xl gap-0 rounded-lg p-0"
			>
				<button
					type="button"
					className="hover:bg-muted absolute top-2 right-2 z-10 flex size-9 cursor-pointer items-center justify-center rounded-full border-0"
					onClick={onClose}
				>
					<X size={16} />
				</button>
				<div className="border-border border-b px-6 py-5">
					<DialogTitle className="text-lg font-semibold">编辑房间</DialogTitle>
					<p className="text-muted-foreground mt-1 text-sm">
						修改房间名称、说明、人数上限与接入策略。密码不在本页修改。
					</p>
				</div>
				<div className="grid gap-6 px-6 py-5">
					<Form
						fields={fields}
						defaultValues={{
							name: room.name,
							description: room.description,
							limit: room.limit,
							audio_only: room.audio_only,
							allow_audience: room.allow_audience,
						}}
						formClassName="grid gap-2 md:grid-cols-2 md:gap-x-4"
						submitButtonText="保存修改"
						onSubmit={handleSubmit}
					/>
				</div>
			</DialogContent>
		</Dialog>
	);
}
