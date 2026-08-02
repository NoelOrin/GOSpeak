import { createForm } from "@tanstack/solid-form";
import type { Component } from "solid-js";
import { showToast } from "solid-notifications";
import type { RoomRecord } from "@/api/room";
import { updateRoom } from "@/api/room";
import { Form, type FormFieldConfig } from "@/components/form";

export interface EditRoomFormErrors {
	name?: string;
	description?: string;
	limit?: string;
}

export function validateEditRoomForm(
	name: string,
	description: string,
	limit: number | "",
): EditRoomFormErrors {
	const errors: EditRoomFormErrors = {};
	const trimmed = name.trim();
	if (trimmed.length < 2) errors.name = "房间名称至少需要 2 个字符";
	else if (trimmed.length > 32) errors.name = "房间名称不能超过 32 个字符";
	if (description.trim().length > 120)
		errors.description = "房间说明不能超过 120 个字符";
	if (limit === "" || Number.isNaN(limit) || limit < 2)
		errors.limit = "人数上限至少为 2";
	else if (limit > 200) errors.limit = "人数上限不能超过 200";
	return errors;
}

interface EditRoomModalProps {
	ref: HTMLDialogElement;
	room: RoomRecord;
	onClose: () => void;
	onSaved: (room: RoomRecord) => void | Promise<void>;
}

const EditRoomModal: Component<EditRoomModalProps> = (props) => {
	const roomNameValidation = (value: string) => {
		const trimmed = value.trim();
		if (trimmed.length < 2) return "房间名称至少需要 2 个字符";
		if (trimmed.length > 32) return "房间名称不能超过 32 个字符";
		return undefined;
	};

	const limitValidation = (value: number) => {
		if (Number.isNaN(value)) return "请填写人数上限";
		if (value < 2) return "人数上限至少为 2";
		if (value > 200) return "人数上限不能超过 200";
		return undefined;
	};

	const form = createForm(() => ({
		defaultValues: {
			name: props.room.name,
			description: props.room.description,
			limit: props.room.limit,
			audioOnly: props.room.audio_only,
			allowAudience: props.room.allow_audience,
		},
		onSubmit: async ({ value }) => {
			try {
				const updated = await updateRoom({
					id: props.room.id,
					name: value.name.trim(),
					description: value.description.trim(),
					limit: value.limit,
					audio_only: value.audioOnly,
					allow_audience: value.allowAudience,
				});
				await props.onSaved(updated);
				showToast(`已更新房间: ${updated.name}`, { type: "success" });
				props.onClose();
			} catch {}
		},
	}));

	const fields: FormFieldConfig[] = [
		{
			name: "name",
			label: "房间名称",
			type: "text",
			placeholder: "例如：产品评审会",
			required: true,
			validation: roomNameValidation,
		},
		{
			name: "limit",
			label: "人数上限",
			type: "number",
			required: true,
			validation: limitValidation,
		},
		{
			name: "audioOnly",
			label: "仅语音模式",
			type: "switch",
		},
		{
			name: "allowAudience",
			label: "允许听众加入",
			type: "switch",
		},
		{
			name: "description",
			label: "房间说明",
			type: "textarea",
			placeholder: "选填，用于说明当前房间的主题或规则",
			validation: (value: string) =>
				value.trim().length > 120 ? "房间说明不能超过 120 个字符" : undefined,
			className: "min-h-24",
		},
	];

	return (
		<dialog ref={props.ref} class="modal" id="edit_room_modal">
			<div class="modal-box max-w-xl rounded-lg p-0">
				<button
					class="top-2 right-2 absolute border-0 z-10 btn btn-sm btn-circle"
					onClick={props.onClose}
				>
					✕
				</button>
				<div class="border-base-300 border-b px-6 py-5">
					<h3 class="text-lg font-semibold">编辑房间</h3>
					<p class="mt-1 text-sm text-base-content/60">
						修改房间名称、说明、人数上限与接入策略。密码不在本页修改。
					</p>
				</div>
				<div class="grid gap-6 px-6 py-5">
					<Form
						form={form}
						fields={fields}
						formClassName="grid gap-2 md:grid-cols-2 md:gap-x-4"
						submitButtonText="保存修改"
					/>
				</div>
				<form method="dialog" class="modal-backdrop">
					<button onClick={props.onClose}></button>
				</form>
			</div>
		</dialog>
	);
};

export default EditRoomModal;
