import { createForm } from "@tanstack/solid-form";
import { useNavigate } from "@tanstack/solid-router";
import { type Component, createMemo, Show } from "solid-js";
import { showToast } from "solid-notifications";
import X from "lucide-solid/icons/x";
import { createRoom as createRoomApi } from "@/api/room";
import { Form, type FormFieldConfig } from "@/components/form";
import { socketStore } from "@/stores/socketStore";
import {
	createRoomLimitSchema,
	fieldError,
	roomDescriptionSchema,
	roomNameSchema,
	roomPasswordSchema,
} from "@/utils/formSchemas";

export interface CreateRoomConfig {
	name: string;
	password: string;
	limit: number;
	joinAfterCreate: boolean;
	audioOnly: boolean;
	allowAudience: boolean;
	description: string;
	type: "text" | "voice";
	domainUUID: string;
}

interface CreateRoomModalProps {
	ref: HTMLDialogElement;
	onClose: () => void;
	domainUUID: string;
	onCreated?: (config: CreateRoomConfig) => void | Promise<void>;
}

const CreateRoomModal: Component<CreateRoomModalProps> = (props) => {
	const navigate = useNavigate();
	const roomNameValidation = (value: string) =>
		fieldError(roomNameSchema, value);

	const limitValidation = (value: number) =>
		fieldError(createRoomLimitSchema, value);

	const form = createForm(() => ({
		defaultValues: {
			name: "",
			password: "",
			limit: 12,
			joinAfterCreate: true,
			audioOnly: true,
			allowAudience: true,
			description: "",
			type: "voice",
		},
		onSubmit: async ({ value }) => {
			try {
				const domainUUID = props.domainUUID.trim();
				if (!domainUUID) {
					showToast("请先选择域", { type: "warning" });
					return;
				}
				const payload: CreateRoomConfig = {
					name: value.name.trim(),
					password: value.password,
					limit: value.limit,
					joinAfterCreate: value.joinAfterCreate,
					audioOnly: value.audioOnly,
					allowAudience: value.allowAudience,
					description: value.description.trim(),
					type: value.type === "text" ? "text" : "voice",
					domainUUID,
				};
				const room = await createRoomApi({
					name: payload.name,
					password: payload.password || undefined,
					description: payload.description,
					limit: payload.limit,
					audio_only: payload.audioOnly,
					allow_audience: payload.allowAudience,
					type: payload.type,
					domain_uuid: payload.domainUUID,
				});
				await props.onCreated?.(payload);

				socketStore.connect();
				socketStore.selectRoom({
					id: room.id,
					uuid: room.uuid,
					name: room.name,
					domain_uuid: payload.domainUUID,
					hasPassword: !!payload.password,
					description: room.description,
					limit: room.limit,
					audioOnly: room.audio_only,
					allowAudience: room.allow_audience,
					type: room.type,
					members: [],
					count: 0,
					createdAt: Date.now(),
				});
				socketStore.listRooms();
				showToast(`已创建房间: ${payload.name}`, { type: "success" });
				props.onClose();

				if (payload.joinAfterCreate) {
					const domainUUID = payload.domainUUID;
					if (domainUUID)
						navigate({ to: "/domain/$domainUUID", params: { domainUUID } });
					else navigate({ to: "/discover" });
				}
			} catch (error) {
				console.error("[Room] create failed:", error);
				showToast(error instanceof Error ? error.message : String(error), {
					type: "error",
				});
			}
		},
	}));

	const fields: FormFieldConfig[] = [
		{
			name: "type",
			label: "房间类型",
			type: "select",
			options: [
				{ value: "voice", label: "语音房" },
				{ value: "text", label: "文字房" },
			],
		},
		{
			name: "name",
			label: "房间名称",
			type: "text",
			placeholder: "例如：产品评审会",
			required: true,
			validation: roomNameValidation,
		},
		{
			name: "password",
			label: "房间密码",
			type: "password",
			placeholder: "选填，留空表示公开房间",
			validation: (value: string) => fieldError(roomPasswordSchema, value),
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
			name: "joinAfterCreate",
			label: "创建后进入域页",
			type: "switch",
		},
		{
			name: "description",
			label: "房间说明",
			type: "textarea",
			placeholder: "选填，用于说明当前房间的主题或规则",
			validation: (value: string) => fieldError(roomDescriptionSchema, value),
			className: "min-h-24",
		},
	];

	const summary = createMemo(() => {
		const values = form.state.values;
		return {
			name: values.name.trim() || "未命名房间",
			limit: values.limit || 0,
			kind: values.type === "text" ? "文字房" : "语音房",
			mode:
				values.type === "text"
					? "文字聊天"
					: values.audioOnly
						? "语音"
						: "音视频",
			audience: values.allowAudience ? "允许旁听" : "仅成员加入",
		};
	});

	return (
		<dialog ref={props.ref} class="modal" id="create_room_modal">
			<div class="modal-box max-w-3xl rounded-lg p-0">
				<button
					class="top-2 right-2 absolute border-0 z-10 btn btn-sm btn-circle"
					onClick={props.onClose}
				>
					<X size={16} />
				</button>
				<div class="border-base-300 border-b px-6 py-5">
					<h3 class="text-lg font-semibold">新建房间</h3>
					<p class="mt-1 text-sm text-base-content/60">
						填写基础配置后创建房间。房间说明、人数上限和接入策略会随房间信息一起保存。
					</p>
				</div>

				<div class="grid gap-6 px-6 py-5 lg:grid-cols-[minmax(0,1fr)_260px]">
					<div>
						<Form
							form={form}
							fields={fields}
							formClassName="grid gap-2 md:grid-cols-2 md:gap-x-4"
							submitButtonText="创建房间"
						/>
					</div>

					<aside class="rounded-lg bg-base-200/70 p-4">
						<div class="text-sm font-medium">配置预览</div>
						<div class="mt-4 space-y-3 text-sm">
							<div>
								<div class="text-base-content/50">房间名称</div>
								<div class="mt-1 font-medium break-words">{summary().name}</div>
							</div>
							<div>
								<div class="text-base-content/50">接入规模</div>
								<div class="mt-1 font-medium">
									最多 {summary().limit || "-"} 人
								</div>
							</div>
							<div>
								<div class="text-base-content/50">媒体模式</div>
								<div class="mt-1 font-medium">{summary().mode}</div>
							</div>
							<div>
								<div class="text-base-content/50">房间类型</div>
								<div class="mt-1 font-medium">{summary().kind}</div>
							</div>
							<div>
								<div class="text-base-content/50">加入策略</div>
								<div class="mt-1 font-medium">{summary().audience}</div>
							</div>
						</div>

						<Show when={form.state.isSubmitting}>
							<div class="mt-4 text-sm text-base-content/60">
								正在提交房间配置...
							</div>
						</Show>
					</aside>
				</div>

				<form method="dialog" class="modal-backdrop">
					<button onClick={props.onClose}></button>
				</form>
			</div>
		</dialog>
	);
};

export default CreateRoomModal;
