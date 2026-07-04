import { createForm } from "@tanstack/solid-form";
import { createMemo, Show } from "solid-js";
import { Form, type FormFieldConfig } from "../../../form";
import { socketStore } from "@/stores/socketStore";
import type { SettingTabConfig } from "./types";

const RoomForm = () => {
	const form = createForm(() => ({
		defaultValues: {
			roomName: "",
			hasPassword: false,
			roomPassword: "",
			maxMembers: "",
		},
		onSubmit: ({ value }) => {
			const password = value.hasPassword ? value.roomPassword : undefined;
			socketStore.createRoom(value.roomName, password);
		},
	}));

	const hasPassword = () => form.getFieldValue("hasPassword") as boolean;

	const fields = createMemo<FormFieldConfig[]>(() => {
		const base: FormFieldConfig[] = [
			{
				name: "roomName",
				label: "房间名称",
				type: "text",
				placeholder: "请输入房间名称",
				required: true,
			},
			{
				name: "hasPassword",
				label: "设置密码",
				type: "switch",
			},
		];
		if (hasPassword()) {
			base.push({
				name: "roomPassword",
				label: "房间密码",
				type: "password",
				placeholder: "请输入房间密码",
				required: true,
			});
		}
		base.push({
			name: "maxMembers",
			label: "最大人数",
			type: "number",
			placeholder: "请输入最大人数",
		});
		return base;
	});

	return (
		<div class="p-4">
			<h3 class="mb-4 font-bold text-lg">房间</h3>
			<Form
				form={form}
				fields={fields()}
				showSubmitButton
				submitButtonText="创建"
				formClassName="grid grid-cols-2 gap-4 card"
			/>
		</div>
	);
};

const room: SettingTabConfig = {
	label: "房间",
	component: RoomForm,
};

export default room;
