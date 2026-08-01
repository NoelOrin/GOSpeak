import { createFileRoute, Link } from "@tanstack/solid-router";
import ArrowLeft from "lucide-solid/icons/arrow-left";
import Globe from "lucide-solid/icons/globe";
import Lock from "lucide-solid/icons/lock";
import Save from "lucide-solid/icons/save";
import Settings2 from "lucide-solid/icons/settings-2";
import { createEffect, createMemo, createSignal, Show } from "solid-js";
import { showToast } from "solid-notifications";
import type { GuildMember } from "@/api/guild";
import { kickGuildMember, updateGuild } from "@/api/guild";
import ConfirmModal from "@/components/common/ConfirmModal";
import GuildMemberTable, {
	executeKickMember,
} from "@/components/guild/GuildMemberTable";
import guildStore from "@/stores/guildStore";
import userStore from "@/stores/userStore";
import { hasPermission } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/guild/$guildUUID/manage")({
	component: RouteComponent,
	staticData: {
		title: "服务器管理",
		icon: "icon-manage",
	},
});

export interface GuildFormErrors {
	name?: string;
	maxRooms?: string;
}

export function validateGuildForm(
	name: string,
	maxRooms: number | "",
): GuildFormErrors {
	const errors: GuildFormErrors = {};
	if (!name.trim()) {
		errors.name = "服务器名称不能为空";
	}
	if (maxRooms === "" || !Number.isInteger(maxRooms) || maxRooms < 1) {
		errors.maxRooms = "房间上限必须是大于等于 1 的整数";
	}
	return errors;
}

function apiErrorMessage(error: unknown) {
	const response = (
		error as {
			response?: { data?: { msg?: string } };
		}
	)?.response?.data?.msg;
	if (response) return response;
	if (error instanceof Error) return error.message;
	return "请求失败";
}

function RouteComponent() {
	const params = Route.useParams();
	const { state, setCurrentGuild, loadMembers, updateCachedGuild } = guildStore;
	const uuid = () => params().guildUUID;
	const currentUser = () => userStore.user();

	const guild = createMemo(() => state.guildCache[uuid()]);
	const members = createMemo(() => state.memberCache[uuid()] ?? []);
	const guildError = createMemo(() => state.guildErrors[uuid()] ?? "");
	const memberLoading = createMemo(
		() => state.memberLoading[uuid()] ?? !state.memberCache[uuid()],
	);
	const memberError = createMemo(() => state.memberErrors[uuid()] ?? "");

	const currentRole = createMemo(
		() =>
			members().find((member) => member.user_uuid === currentUser()?.uuid)
				?.role_name ?? "",
	);
	const isOwner = createMemo(
		() => !!currentUser() && guild()?.owner_uuid === currentUser()?.uuid,
	);
	const canManage = createMemo(
		() =>
			isOwner() || currentRole() === "admin" || hasPermission("guild:manage"),
	);
	const canKick = createMemo(
		() => isOwner() || currentRole() === "admin" || hasPermission("guild:kick"),
	);

	const [name, setName] = createSignal("");
	const [description, setDescription] = createSignal("");
	const [isPublic, setIsPublic] = createSignal(false);
	const [maxRooms, setMaxRooms] = createSignal<number | "">("");
	const [nameError, setNameError] = createSignal("");
	const [maxRoomsError, setMaxRoomsError] = createSignal("");
	const [formError, setFormError] = createSignal("");
	const [saving, setSaving] = createSignal(false);
	const [kickTarget, setKickTarget] = createSignal<GuildMember | null>(null);
	const [kicking, setKicking] = createSignal(false);
	const [kickError, setKickError] = createSignal("");
	const maxRoomsValue = () =>
		maxRooms() === "" || Number.isNaN(maxRooms()) ? "" : String(maxRooms());
	let formUUID = "";
	let kickDialogRef!: HTMLDialogElement;

	createEffect(() => {
		const currentUUID = uuid();
		setCurrentGuild(currentUUID);
		void loadMembers(currentUUID).catch(() => {});
	});

	createEffect(() => {
		const current = guild();
		if (current) {
			document.title = `${current.name} 管理 | GOSpeak`;
		}
	});

	createEffect(() => {
		const current = guild();
		if (!current || formUUID === current.uuid) return;
		formUUID = current.uuid;
		setName(current.name);
		setDescription(current.description ?? "");
		setIsPublic(current.is_public);
		setMaxRooms(current.max_rooms || 20);
		setNameError("");
		setMaxRoomsError("");
		setFormError("");
	});

	function requestKick(userUUID: string) {
		const target = members().find((member) => member.user_uuid === userUUID);
		if (!target) return;
		setKickError("");
		setKickTarget(target);
		queueMicrotask(() => kickDialogRef?.showModal());
	}

	function closeKickModal() {
		kickDialogRef?.close();
		setKickTarget(null);
		setKickError("");
	}

	async function handleSave(e: Event) {
		e.preventDefault();
		if (!canManage()) {
			setFormError("无管理权限");
			return;
		}

		const errors = validateGuildForm(name(), maxRooms());
		setNameError(errors.name ?? "");
		setMaxRoomsError(errors.maxRooms ?? "");
		if (errors.name || errors.maxRooms) {
			setFormError("请修正表单错误");
			return;
		}

		setSaving(true);
		setFormError("");
		try {
			const updated = await updateGuild({
				uuid: uuid(),
				name: name().trim(),
				description: description().trim(),
				is_public: isPublic(),
				max_rooms: maxRooms() as number,
			});
			updateCachedGuild(updated);
			setName(updated.name);
			setDescription(updated.description ?? "");
			setIsPublic(updated.is_public);
			setMaxRooms(updated.max_rooms || 20);
			showToast("设置已保存", { type: "success" });
		} catch (error) {
			const message = apiErrorMessage(error);
			setFormError(message);
			showToast(message, { type: "error" });
		} finally {
			setSaving(false);
		}
	}

	async function handleKick() {
		const target = kickTarget();
		if (!target) return;
		setKicking(true);
		setKickError("");
		try {
			await executeKickMember(
				uuid(),
				target.user_uuid,
				kickGuildMember,
				async (guildUUID) => {
					try {
						await loadMembers(guildUUID);
					} catch {
						showToast("成员已移出，但成员列表刷新失败", {
							type: "warning",
						});
					}
				},
			);
			closeKickModal();
			showToast("成员已移出", { type: "success" });
		} catch (error) {
			const message = apiErrorMessage(error);
			setKickError(message);
			showToast(message, { type: "error" });
		} finally {
			setKicking(false);
		}
	}

	function retryGuild() {
		setCurrentGuild(uuid());
	}

	return (
		<div class="flex-1 min-w-0 h-full overflow-y-auto p-4 sm:p-6">
			<div class="max-w-6xl mx-auto">
				<div class="flex flex-wrap items-center gap-3 mb-6">
					<Link
						to="/guild/$guildUUID"
						params={{ guildUUID: uuid() }}
						class="btn btn-ghost btn-sm"
					>
						<ArrowLeft size={16} />
						返回
					</Link>
					<div class="min-w-0">
						<h1 class="text-xl font-bold truncate">
							{guild()?.name || "服务器管理"}
						</h1>
						<p class="text-sm text-base-content/60 truncate">
							{guild()?.description || ""}
						</p>
					</div>
				</div>

				<Show when={guild()}>
					<div class="grid gap-5 lg:grid-cols-[minmax(0,420px)_minmax(0,1fr)]">
						<section class="rounded-lg border border-base-300 bg-base-100 p-4">
							<div class="flex items-center gap-2 mb-4">
								<Settings2 size={17} class="text-base-content/60" />
								<h2 class="font-semibold">服务器设置</h2>
							</div>
							<Show
								when={canManage()}
								fallback={
									<div class="grid gap-3 text-sm">
										<div class="text-base-content/60">当前账号无编辑权限</div>
										<dl class="grid gap-2">
											<div>
												<dt class="text-xs text-base-content/50">服务器名称</dt>
												<dd>{guild()?.name}</dd>
											</div>
											<div>
												<dt class="text-xs text-base-content/50">描述</dt>
												<dd>{guild()?.description || "-"}</dd>
											</div>
											<div>
												<dt class="text-xs text-base-content/50">房间上限</dt>
												<dd>{guild()?.max_rooms || 20}</dd>
											</div>
											<div>
												<dt class="text-xs text-base-content/50">公开状态</dt>
												<dd>{guild()?.is_public ? "公开" : "私有"}</dd>
											</div>
										</dl>
									</div>
								}
							>
								<form
									onSubmit={handleSave}
									class="flex flex-col gap-4"
									novalidate
								>
									<label class="form-control">
										<span class="label">
											<span class="label-text">服务器名称</span>
										</span>
										<input
											id="guild-name"
											class="input input-bordered input-sm"
											value={name()}
											maxLength={100}
											aria-invalid={!!nameError()}
											aria-describedby={
												nameError() ? "guild-name-error" : undefined
											}
											onInput={(e) => {
												setName(e.currentTarget.value);
												if (e.currentTarget.value.trim()) {
													setNameError("");
												}
											}}
										/>
										<Show when={nameError()}>
											<span
												id="guild-name-error"
												class="mt-1 text-xs text-error"
											>
												{nameError()}
											</span>
										</Show>
									</label>
									<label class="form-control">
										<span class="label">
											<span class="label-text">描述</span>
										</span>
										<textarea
											id="guild-description"
											class="textarea textarea-bordered textarea-sm min-h-24"
											value={description()}
											onInput={(e) => setDescription(e.currentTarget.value)}
										/>
									</label>
									<label class="form-control">
										<span class="label">
											<span class="label-text">房间上限</span>
										</span>
										<input
											id="guild-max-rooms"
											class="input input-bordered input-sm"
											type="number"
											min={1}
											step={1}
											value={maxRoomsValue()}
											aria-invalid={!!maxRoomsError()}
											aria-describedby={
												maxRoomsError() ? "guild-max-rooms-error" : undefined
											}
											onInput={(e) => {
												const raw = e.currentTarget.value;
												setMaxRooms(raw === "" ? "" : Number(raw));
												if (
													raw !== "" &&
													Number.isInteger(Number(raw)) &&
													Number(raw) >= 1
												) {
													setMaxRoomsError("");
												}
											}}
										/>
										<Show when={maxRoomsError()}>
											<span
												id="guild-max-rooms-error"
												class="mt-1 text-xs text-error"
											>
												{maxRoomsError()}
											</span>
										</Show>
									</label>
									<label class="flex items-center justify-between rounded-lg border border-base-300 px-3 py-2.5 cursor-pointer">
										<span class="flex items-center gap-2 text-sm">
											{isPublic() ? (
												<Globe size={16} class="text-success" />
											) : (
												<Lock size={16} class="text-base-content/50" />
											)}
											公开服务器
										</span>
										<input
											type="checkbox"
											class="toggle toggle-primary toggle-sm"
											checked={isPublic()}
											disabled={saving()}
											onChange={(e) => setIsPublic(e.currentTarget.checked)}
										/>
									</label>
									<Show when={formError()}>
										<p role="alert" class="text-xs text-error">
											{formError()}
										</p>
									</Show>
									<button
										type="submit"
										class="btn btn-primary btn-sm"
										disabled={saving()}
									>
										{saving() ? (
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
						</section>

						<section class="rounded-lg border border-base-300 bg-base-100 min-w-0">
							<GuildMemberTable
								members={members()}
								ownerUUID={guild()?.owner_uuid}
								currentUserUUID={currentUser()?.uuid}
								canKick={canKick()}
								kickDisabled={kicking()}
								loading={memberLoading() && members().length === 0}
								refreshing={
									(memberLoading() || kicking()) && members().length > 0
								}
								error={memberError()}
								onRefresh={() => void loadMembers(uuid()).catch(() => {})}
								onKick={requestKick}
							/>
						</section>
					</div>
				</Show>

				<Show when={!guild() && guildError()}>
					<div class="flex flex-col items-center gap-4 py-16 text-center">
						<p role="alert" class="text-error">
							服务器加载失败：{guildError()}
						</p>
						<button
							type="button"
							class="btn btn-primary btn-sm"
							onClick={retryGuild}
						>
							重试
						</button>
					</div>
				</Show>
				<Show when={!guild() && !guildError()}>
					<div class="flex justify-center py-16">
						<span class="loading loading-spinner loading-md" />
					</div>
				</Show>
			</div>

			<ConfirmModal
				open={!!kickTarget()}
				title="移出成员"
				message={
					<span>
						确认将{" "}
						{kickTarget()?.nickname || kickTarget()?.user_uuid || "该成员"}{" "}
						移出服务器？
						<Show when={kickError()}>
							<span class="mt-2 block text-error">{kickError()}</span>
						</Show>
					</span>
				}
				confirmText="移出"
				confirmClass="btn btn-error"
				loading={kicking()}
				dialogRef={(el) => {
					kickDialogRef = el;
				}}
				onClose={closeKickModal}
				onConfirm={handleKick}
			/>
		</div>
	);
}
