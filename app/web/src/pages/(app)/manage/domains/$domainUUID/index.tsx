import { createFileRoute, Link } from "@tanstack/solid-router";
import ArrowLeft from "lucide-solid/icons/arrow-left";
import CirclePlus from "lucide-solid/icons/circle-plus";
import Globe from "lucide-solid/icons/globe";
import Lock from "lucide-solid/icons/lock";
import Save from "lucide-solid/icons/save";
import Settings2 from "lucide-solid/icons/settings-2";
import {
	createEffect,
	createMemo,
	createSignal,
	Show,
	untrack,
} from "solid-js";
import { showToast } from "solid-notifications";
import type { DomainMember } from "@/api/domain";
import { kickDomainMember, updateDomain } from "@/api/domain";
import { deleteRoom, listRooms, type RoomRecord } from "@/api/room";
import ConfirmModal from "@/components/common/ConfirmModal";
import DomainMemberTable, {
	executeKickMember,
	memberDisplayName,
} from "@/components/domain/DomainMemberTable";
import DomainRoomTable from "@/components/domain/DomainRoomTable";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import CreateRoomModal from "@/components/modal/createRoomModal";
import EditRoomModal from "@/components/modal/editRoomModal";
import domainStore from "@/stores/domainStore";
import userStore from "@/stores/userStore";
import { hasAnyPermission, hasPermission } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/manage/domains/$domainUUID/")({
	component: RouteComponent,
	staticData: {
		title: "域管理",
		icon: "icon-manage",
	},
});

export interface DomainFormErrors {
	name?: string;
}

export function validateDomainForm(name: string): DomainFormErrors {
	const errors: DomainFormErrors = {};
	if (!name.trim()) {
		errors.name = "域名称不能为空";
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
	const { state, setCurrentDomain, loadMembers, updateCachedDomain } =
		domainStore;
	const uuid = () => params().domainUUID;
	const currentUser = () => userStore.user();

	const domain = createMemo(() => state.domainCache[uuid()]);
	const members = createMemo(() => state.memberCache[uuid()] ?? []);
	const domainError = createMemo(() => state.domainErrors[uuid()] ?? "");
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
		() => !!currentUser() && domain()?.owner_uuid === currentUser()?.uuid,
	);
	const canManage = createMemo(
		() =>
			isOwner() || currentRole() === "admin" || hasPermission("domain:manage"),
	);
	const canKick = createMemo(
		() =>
			isOwner() || currentRole() === "admin" || hasPermission("domain:kick"),
	);

	const ROOM_PAGE_SIZE = 10;
	const [rooms, setRooms] = createSignal<RoomRecord[]>([]);
	const [roomTotal, setRoomTotal] = createSignal(0);
	const [roomPage, setRoomPage] = createSignal(1);
	const [roomLoading, setRoomLoading] = createSignal(true);
	const [roomRefreshing, setRoomRefreshing] = createSignal(false);
	const [roomError, setRoomError] = createSignal("");
	const [editingRoom, setEditingRoom] = createSignal<RoomRecord | null>(null);
	const [deletingRoom, setDeletingRoom] = createSignal<RoomRecord | null>(null);
	const [deleting, setDeleting] = createSignal(false);
	const [deleteError, setDeleteError] = createSignal("");
	const totalRoomPages = createMemo(() =>
		Math.max(1, Math.ceil(roomTotal() / ROOM_PAGE_SIZE)),
	);
	const canManageRooms = createMemo(
		() =>
			isOwner() ||
			currentRole() === "admin" ||
			hasAnyPermission("room:update", "room:delete"),
	);
	let createRoomDialogRef!: HTMLDialogElement;
	let editRoomDialogRef!: HTMLDialogElement;
	let deleteDialogRef!: HTMLDialogElement;

	const [name, setName] = createSignal("");
	const [description, setDescription] = createSignal("");
	const [isPublic, setIsPublic] = createSignal(false);
	const [nameError, setNameError] = createSignal("");
	const [formError, setFormError] = createSignal("");
	const [saving, setSaving] = createSignal(false);
	const [kickTarget, setKickTarget] = createSignal<DomainMember | null>(null);
	const kickTargetName = () => {
		const target = kickTarget();
		return target ? memberDisplayName(target) : "该成员";
	};
	const [kicking, setKicking] = createSignal(false);
	const [kickError, setKickError] = createSignal("");
	let formUUID = "";
	let kickDialogRef!: HTMLDialogElement;

	async function fetchRooms(page: number) {
		setRoomError("");
		if (rooms().length > 0) setRoomRefreshing(true);
		else setRoomLoading(true);
		try {
			const result = await listRooms(page, ROOM_PAGE_SIZE, uuid());
			setRooms(result.rooms);
			setRoomTotal(result.total);
			setRoomPage(result.page);
		} catch (error) {
			setRoomError(apiErrorMessage(error));
		} finally {
			setRoomLoading(false);
			setRoomRefreshing(false);
		}
	}

	function resetRooms() {
		setRooms([]);
		setRoomTotal(0);
		setRoomPage(1);
		setRoomError("");
	}

	function openEditRoom(room: RoomRecord) {
		setEditingRoom(room);
		queueMicrotask(() => editRoomDialogRef?.showModal?.());
	}

	function requestDeleteRoom(room: RoomRecord) {
		setDeleteError("");
		setDeletingRoom(room);
		queueMicrotask(() => deleteDialogRef?.showModal?.());
	}

	function closeDeleteModal() {
		deleteDialogRef?.close();
		setDeletingRoom(null);
		setDeleteError("");
	}

	async function handleDeleteRoom() {
		const room = deletingRoom();
		if (!room) return;
		setDeleting(true);
		setDeleteError("");
		try {
			await deleteRoom(room.id);
			closeDeleteModal();
			showToast("房间已删除", { type: "success" });
			void fetchRooms(roomPage());
		} catch (error) {
			const message = apiErrorMessage(error);
			setDeleteError(message);
			showToast(message, { type: "error" });
		} finally {
			setDeleting(false);
		}
	}

	createEffect(() => {
		const currentUUID = uuid();
		setCurrentDomain(currentUUID);
		void loadMembers(currentUUID).catch(() => {});
		untrack(() => {
			resetRooms();
			void fetchRooms(1);
		});
	});

	createEffect(() => {
		const current = domain();
		if (current) {
			document.title = `${current.name} 管理 | GOSpeak`;
		}
	});

	createEffect(() => {
		const current = domain();
		if (!current || formUUID === current.uuid) return;
		formUUID = current.uuid;
		setName(current.name);
		setDescription(current.description ?? "");
		setIsPublic(current.is_public);
		setNameError("");
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

		const errors = validateDomainForm(name());
		setNameError(errors.name ?? "");
		if (errors.name) {
			setFormError("请修正表单错误");
			return;
		}

		setSaving(true);
		setFormError("");
		try {
			const updated = await updateDomain({
				domain_uuid: uuid(),
				name: name().trim(),
				description: description().trim(),
				is_public: isPublic(),
			});
			updateCachedDomain(updated);
			setName(updated.name);
			setDescription(updated.description ?? "");
			setIsPublic(updated.is_public);
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
				kickDomainMember,
				async (domainUUID) => {
					try {
						await loadMembers(domainUUID);
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

	function retryDomain() {
		setCurrentDomain(uuid());
	}

	return (
		<div class="flex-1 min-w-0 h-full overflow-y-auto">
			<ManagePage class="min-h-full w-full">
				<ManageHeader
					icon={<Settings2 size={18} />}
					title={domain()?.name || "域管理"}
					description={domain()?.description || "管理域设置与成员"}
					actions={
						<Link
							to="/domain/$domainUUID"
							params={{ domainUUID: uuid() }}
							class="btn btn-ghost btn-sm"
						>
							<ArrowLeft size={16} />
							返回
						</Link>
					}
				/>

				<Show when={domain()}>
					<div class="grid min-w-0 gap-5 lg:grid-cols-[minmax(360px,400px)_minmax(0,1fr)]">
						<ManageSection
							title="域设置"
							description="调整域名称、描述与公开状态"
							class="min-w-0"
						>
							<Show
								when={canManage()}
								fallback={
									<div class="grid gap-3 text-sm">
										<div class="text-base-content/60">当前账号无编辑权限</div>
										<dl class="grid gap-2">
											<div>
												<dt class="text-xs text-base-content/50">域名称</dt>
												<dd>{domain()?.name}</dd>
											</div>
											<div>
												<dt class="text-xs text-base-content/50">描述</dt>
												<dd>{domain()?.description || "-"}</dd>
											</div>
											<div>
												<dt class="text-xs text-base-content/50">公开状态</dt>
												<dd>{domain()?.is_public ? "公开" : "私有"}</dd>
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
											<span class="label-text text-xs font-medium text-base-content/70">
												域名称
											</span>
										</span>
										<input
											id="domain-name"
											class="input input-bordered input-sm"
											value={name()}
											maxLength={100}
											aria-invalid={!!nameError()}
											aria-describedby={
												nameError() ? "domain-name-error" : undefined
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
												id="domain-name-error"
												class="mt-1 text-xs text-error"
											>
												{nameError()}
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
											value={description()}
											onInput={(e) => setDescription(e.currentTarget.value)}
										/>
									</label>
									<label class="flex items-center justify-between rounded-lg border border-base-300 px-3 py-2.5 cursor-pointer">
										<span class="flex items-center gap-2 text-sm">
											{isPublic() ? (
												<Globe size={16} class="text-success" />
											) : (
												<Lock size={16} class="text-base-content/50" />
											)}
											公开域
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
						</ManageSection>

						<ManageSection
							title="成员管理"
							description={`${members().length} 位成员`}
							class="min-w-0"
							padded={false}
							actions={
								<button
									type="button"
									class="btn btn-ghost btn-xs"
									disabled={memberLoading() || kicking()}
									onClick={() => void loadMembers(uuid()).catch(() => {})}
								>
									{memberLoading() ? (
										<span class="loading loading-spinner loading-xs" />
									) : null}
									刷新
								</button>
							}
						>
							<DomainMemberTable
								members={members()}
								ownerUUID={domain()?.owner_uuid}
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
						</ManageSection>
					</div>

					<ManageSection
						title="房间管理"
						description={`${roomTotal()} 个房间`}
						padded={false}
						actions={
							<>
								<button
									type="button"
									class="btn btn-ghost btn-xs"
									disabled={roomLoading() || roomRefreshing()}
									onClick={() => {
										resetRooms();
										void fetchRooms(1);
									}}
								>
									{roomLoading() || roomRefreshing() ? (
										<span class="loading loading-spinner loading-xs" />
									) : null}
									刷新
								</button>
								<Show when={canManageRooms()}>
									<button
										type="button"
										class="btn btn-primary btn-xs"
										onClick={() => createRoomDialogRef?.showModal?.()}
									>
										<CirclePlus size={14} />
										新建房间
									</button>
								</Show>
							</>
						}
					>
						<DomainRoomTable
							rooms={rooms()}
							currentUserName={currentUser()?.name}
							canManage={canManageRooms()}
							loading={roomLoading()}
							refreshing={roomRefreshing()}
							error={roomError()}
							page={roomPage()}
							totalPages={totalRoomPages()}
							onPageChange={(page) => void fetchRooms(page)}
							onRefresh={() => {
								resetRooms();
								void fetchRooms(1);
							}}
							onEdit={openEditRoom}
							onDelete={requestDeleteRoom}
						/>
					</ManageSection>
				</Show>

				<Show when={!domain() && domainError()}>
					<div class="flex flex-col items-center gap-4 py-16 text-center">
						<p role="alert" class="text-error">
							域加载失败：{domainError()}
						</p>
						<button
							type="button"
							class="btn btn-primary btn-sm"
							onClick={retryDomain}
						>
							重试
						</button>
					</div>
				</Show>
				<Show when={!domain() && !domainError()}>
					<div class="flex justify-center py-16">
						<span class="loading loading-spinner loading-md" />
					</div>
				</Show>
			</ManagePage>
			<ConfirmModal
				open={!!kickTarget()}
				title="移出成员"
				message={
					<span>
						确认将 {kickTargetName()} 移出域？
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
			<CreateRoomModal
				ref={createRoomDialogRef}
				domainUUID={uuid()}
				onClose={() => createRoomDialogRef?.close?.()}
				onCreated={() => {
					resetRooms();
					void fetchRooms(1);
				}}
			/>
			<Show when={editingRoom()}>
				{(room) => (
					<EditRoomModal
						ref={editRoomDialogRef}
						room={room()}
						onClose={() => editRoomDialogRef?.close?.()}
						onSaved={() => {
							void fetchRooms(roomPage());
						}}
					/>
				)}
			</Show>
			<ConfirmModal
				open={!!deletingRoom()}
				title="删除房间"
				message={
					<span>
						确认删除房间 {deletingRoom()?.name || "该房间"}？删除后不可恢复。
						<Show when={deleteError()}>
							<span class="mt-2 block text-error">{deleteError()}</span>
						</Show>
					</span>
				}
				confirmText="删除"
				confirmClass="btn btn-error"
				loading={deleting()}
				dialogRef={(el) => {
					deleteDialogRef = el;
				}}
				onClose={closeDeleteModal}
				onConfirm={handleDeleteRoom}
			/>
		</div>
	);
}
