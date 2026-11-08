import { Dialog, DialogContent, DialogTitle, toast } from "@gospeak/components";
import { useNavigate } from "@tanstack/react-router";
import { X } from "lucide-react";
import { useState } from "react";
import { createRoom as createRoomApi } from "@/api/room";
import { Form, type FormFieldConfig } from "@/components/form";
import { socketStore } from "@/stores/socketStore";
import { toastActionError } from "@/utils/toast";
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
	open: boolean;
	onClose: () => void;
	domainUUID: string;
	onCreated?: (config: CreateRoomConfig) => void | Promise<void>;
}

export default function CreateRoomModal({
	open,
	onClose,
	domainUUID,
	onCreated,
}: CreateRoomModalProps) {
	const navigate = useNavigate();
	const [values, setValues] = useState({
		type: "voice",
		name: "",
		password: "",
		limit: 12,
		audioOnly: true,
		allowAudience: true,
		joinAfterCreate: true,
		description: "",
	});

	const _set = (
		k: keyof typeof values,
		v: (typeof values)[keyof typeof values],
	) => setValues((prev) => ({ ...prev, [k]: v }));

	const fields: Array<FormFieldConfig & { kind?: "switch" }> = [
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
			placeholder: "请输入房间名称",
			required: true,
			validation: (v: string) => fieldError(roomNameSchema, v),
		},
		{
			name: "password",
			label: "房间密码",
			type: "password",
			placeholder: "选填，留空表示公开房间",
			validation: (v: string) => fieldError(roomPasswordSchema, v),
		},
		{
			name: "limit",
			label: "人数上限",
			type: "number",
			required: true,
			validation: (v: number) => fieldError(createRoomLimitSchema, v),
		},
		{ name: "audioOnly", label: "仅语音模式", type: "switch" },
		{ name: "allowAudience", label: "允许听众加入", type: "switch" },
		{ name: "joinAfterCreate", label: "创建后进入域页", type: "switch" },
		{
			name: "description",
			label: "房间说明",
			type: "textarea",
			placeholder: "选填，用于说明当前房间的主题或规则",
			validation: (v: string) => fieldError(roomDescriptionSchema, v),
		},
	];

	const summary = {
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

	async function handleSubmit(formValues: Record<string, any>) {
		try {
			const duuid = domainUUID.trim();
			if (!duuid) {
				toast.warning("请先选择域");
				return;
			}
			const payload: CreateRoomConfig = {
				name: formValues.name.trim(),
				password: formValues.password,
				limit: formValues.limit,
				joinAfterCreate: formValues.joinAfterCreate,
				audioOnly: formValues.audioOnly,
				allowAudience: formValues.allowAudience,
				description: formValues.description.trim(),
				type: formValues.type === "text" ? "text" : "voice",
				domainUUID: duuid,
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
			await onCreated?.(payload);

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
			toast.success(`已创建房间: ${payload.name}`);
			onClose();

			if (payload.joinAfterCreate) {
				navigate({ to: "/domain/$domainUUID", params: { domainUUID: duuid } });
			}
		} catch (error) {
			console.error("[Room] create failed:", error);
			toastActionError(error, "创建房间失败");
		}
	}

	if (!open) return null;

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			<DialogContent
				showClose={false}
				className="max-w-3xl gap-0 rounded-lg p-0"
			>
				<button
					type="button"
					className="hover:bg-muted absolute top-2 right-2 z-10 flex size-9 cursor-pointer items-center justify-center rounded-full border-0"
					onClick={onClose}
				>
					<X size={16} />
				</button>
				<div className="border-border border-b px-6 py-5">
					<DialogTitle className="text-lg font-semibold">新建房间</DialogTitle>
					<p className="text-muted-foreground mt-1 text-sm">
						填写基础配置后创建房间。房间说明、人数上限和接入策略会随房间信息一起保存。
					</p>
				</div>

				<div className="grid gap-6 px-6 py-5 lg:grid-cols-[minmax(0,1fr)_260px]">
					<div>
						<Form
							fields={fields}
							defaultValues={values}
							formClassName="grid gap-2 md:grid-cols-2 md:gap-x-4"
							submitButtonText="创建房间"
							onSubmit={handleSubmit}
						/>
					</div>

					<aside className="bg-muted/70 rounded-lg p-4">
						<div className="text-sm font-medium">配置预览</div>
						<div className="mt-4 space-y-3 text-sm">
							<div>
								<div className="text-faint-foreground">房间名称</div>
								<div className="mt-1 font-medium break-words">
									{summary.name}
								</div>
							</div>
							<div>
								<div className="text-faint-foreground">接入规模</div>
								<div className="mt-1 font-medium">
									最多 {summary.limit || "-"} 人
								</div>
							</div>
							<div>
								<div className="text-faint-foreground">媒体模式</div>
								<div className="mt-1 font-medium">{summary.mode}</div>
							</div>
							<div>
								<div className="text-faint-foreground">房间类型</div>
								<div className="mt-1 font-medium">{summary.kind}</div>
							</div>
							<div>
								<div className="text-faint-foreground">加入策略</div>
								<div className="mt-1 font-medium">{summary.audience}</div>
							</div>
						</div>
					</aside>
				</div>
			</DialogContent>
		</Dialog>
	);
}
