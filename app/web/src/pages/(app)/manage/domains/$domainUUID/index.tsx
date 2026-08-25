import { createFileRoute, Link } from "@tanstack/solid-router";
import ArrowLeft from "lucide-solid/icons/arrow-left";
import CirclePlus from "lucide-solid/icons/circle-plus";
import Settings2 from "lucide-solid/icons/settings-2";
import {
	createEffect,
	createMemo,
	createSignal,
	For,
	Show,
	untrack,
} from "solid-js";
import { showToast } from "solid-notifications";
import {
	type DomainMember,
	type DomainRole,
	createDomainRole,
	deleteDomainRole,
	kickDomainMember,
	listDomainRoles,
	updateDomain,
	updateDomainMemberRole,
	updateDomainRolePermissions,
} from "@/api/domain";
import { deleteRoom, listRooms, type RoomRecord } from "@/api/room";
import ConfirmModal from "@/components/common/ConfirmModal";
import DomainMemberTable, {
	executeKickMember,
	memberDisplayName,
} from "@/components/domain/DomainMemberTable";
import GuestAccessSettings from "@/components/domain/GuestAccessSettings";
import GuestManageTable from "@/components/domain/GuestManageTable";
import DomainRoomTable from "@/components/domain/DomainRoomTable";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import CreateRoomModal from "@/components/modal/createRoomModal";
import { DomainSettingsForm } from "./components/DomainSettingsForm";
import DomainRolePanel from "./components/DomainRolePanel";
import EditRoomModal from "@/components/modal/editRoomModal";
import {
	banGuest,
	getGuestConfig,
	listGuestBans,
	type GuestBanItem,
	type GuestDomain,
	unbanGuest,
	updateGuestConfig,
	cleanupInactiveGuests,
} from "@/api/guest";
import domainStore from "@/stores/domainStore";
import userStore from "@/stores/userStore";
import { hasDomainPermission } from "@/utils/domainPermissions";
import { hasAnyPermission, hasPermission } from "@/utils/permissions";
import { domainNameSchema, fieldError } from "@/utils/formSchemas";

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
	const message = fieldError(domainNameSchema, name);
	if (message) errors.name = message;
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
	const {
		state,
		setCurrentDomain,
		loadMembers,
		loadMyPermissions,
		updateCachedDomain,
	} = domainStore;
	const uuid = () => params().domainUUID;
	const currentUser = () => userStore.user();

	const domain = createMemo(() => state.domainCache[uuid()]);
	const members = createMemo(() => state.memberCache[uuid()] ?? []);
	const domainError = createMemo(() => state.domainErrors[uuid()] ?? "");
	const memberLoading = createMemo(
		() => state.memberLoading[uuid()] ?? !state.memberCache[uuid()],
	);
	const memberError = createMemo(() => state.memberErrors[uuid()] ?? "");
	const [domainRoles, setDomainRoles] = createSignal<DomainRole[]>([]);
	const [assignableCodes, setAssignableCodes] = createSignal<string[]>([]);
	const [rolesLoading, setRolesLoading] = createSignal(true);
	const [rolesError, setRolesError] = createSignal("");
	const [rolesSaving, setRolesSaving] = createSignal(false);

	async function fetchRoles() {
		setRolesLoading(true);
		setRolesError("");
		try {
			const data = await listDomainRoles(uuid());
			setDomainRoles(data.roles);
			setAssignableCodes(data.assignable);
		} catch (error) {
			setRolesError(apiErrorMessage(error));
		} finally {
			setRolesLoading(false);
		}
	}

	async function handleCreateRole(name: string, permissions: string[]) {
		setRolesSaving(true);
		setRolesError("");
		try {
			await createDomainRole(uuid(), name, permissions);
			showToast("角色已创建", { type: "success" });
			await fetchRoles();
		} catch (error) {
			setRolesError(apiErrorMessage(error));
		} finally {
			setRolesSaving(false);
		}
	}

	async function handleUpdateRole(roleName: string, permissions: string[]) {
		setRolesSaving(true);
		setRolesError("");
		try {
			await updateDomainRolePermissions(uuid(), roleName, permissions);
			showToast("权限已保存", { type: "success" });
			await fetchRoles();
			await loadMyPermissions(uuid());
		} catch (error) {
			setRolesError(apiErrorMessage(error));
		} finally {
			setRolesSaving(false);
		}
	}

	async function handleDeleteRole(roleName: string) {
		setRolesSaving(true);
		setRolesError("");
		try {
			await deleteDomainRole(uuid(), roleName);
			showToast("角色已删除", { type: "success" });
			await fetchRoles();
		} catch (error) {
			setRolesError(apiErrorMessage(error));
		} finally {
			setRolesSaving(false);
		}
	}

	async function handleMemberRoleChange(userUUID: string, roleName: string) {
		try {
			await updateDomainMemberRole(uuid(), userUUID, roleName);
			await loadMembers(uuid());
			showToast("成员角色已更新", { type: "success" });
		} catch (error) {
			const message = apiErrorMessage(error);
			showToast(message, { type: "error" });
		}
	}

	const currentRole = createMemo(
		() =>
			members().find((member) => member.user_uuid === currentUser()?.uuid)
				?.role_name ?? "",
	);
	const isOwner = createMemo(
		() => !!currentUser() && domain()?.owner_uuid === currentUser()?.uuid,
	);
	// ── 访客配置与封禁 ──
	const [guestConfig, setGuestConfig] = createSignal<GuestDomain | null>(null);
	const [guestConfigLoading, setGuestConfigLoading] = createSignal(false);
	const [guestSaving, setGuestSaving] = createSignal(false);
	const [guestBans, setGuestBans] = createSignal<GuestBanItem[]>([]);
	const [guestBansLoading, setGuestBansLoading] = createSignal(false);
	const [unbanPending, setUnbanPending] = createSignal<string | null>(null);
	const [banTarget, setBanTarget] = createSignal("");
	const [banReason, setBanReason] = createSignal("");
	const [banHours, setBanHours] = createSignal(0);

	const guestMembers = () => members().filter((m) => m.role_name === "guest");

	async function submitBan() {
		const target = banTarget();
		if (!target) {
			showToast("请选择要封禁的访客", { type: "error" });
			return;
		}
		try {
			await handleBanGuest(target, banReason(), banHours());
			setBanTarget("");
			setBanReason("");
			setBanHours(0);
		} catch (e) {
			showToast(apiErrorMessage(e), { type: "error" });
		}
	}

	async function loadGuestState(domainId: string) {
		setGuestConfigLoading(true);
		setGuestBansLoading(true);
		try {
			const [cfg, bans] = await Promise.all([
				getGuestConfig(domainId),
				listGuestBans(domainId),
			]);
			setGuestConfig(cfg);
			setGuestBans(bans);
		} catch (e) {
			showToast(apiErrorMessage(e), { type: "error" });
		} finally {
			setGuestConfigLoading(false);
			setGuestBansLoading(false);
		}
	}

	async function handleSaveGuestConfig(
		patch: Omit<Parameters<typeof updateGuestConfig>[0], "domain_uuid">,
	) {
		setGuestSaving(true);
		try {
			const cfg = await updateGuestConfig({ ...patch, domain_uuid: uuid() });
			setGuestConfig(cfg);
			showToast("访客配置已保存", { type: "success" });
		} catch (e) {
			showToast(apiErrorMessage(e), { type: "error" });
		} finally {
			setGuestSaving(false);
		}
	}

	async function handleBanGuest(
		userUUID: string,
		reason: string,
		hours: number,
	) {
		await banGuest({
			domain_uuid: uuid(),
			user_uuid: userUUID,
			reason,
			duration_hours: hours,
		});
		// 在线踢出由后端封禁接口联动 signalHub 完成
		await loadGuestState(uuid());
		showToast("已封禁该访客", { type: "success" });
	}

	async function handleUnbanGuest(userUUID: string) {
		setUnbanPending(userUUID);
		try {
			await unbanGuest({ domain_uuid: uuid(), user_uuid: userUUID });
			setGuestBans((prev) => prev.filter((b) => b.user_uuid !== userUUID));
			showToast("已解封", { type: "success" });
		} catch (e) {
			showToast(apiErrorMessage(e), { type: "error" });
		} finally {
			setUnbanPending(null);
		}
	}

	async function handleCleanupGuests() {
		try {
			const res = await cleanupInactiveGuests({
				domain_uuid: uuid(),
				days: 30,
			});
			showToast(`已清理 ${res.removed} 个不活跃访客`, { type: "success" });
		} catch (e) {
			showToast(apiErrorMessage(e), { type: "error" });
		}
	}

	const canManage = createMemo(
		() =>
			isOwner() ||
			currentRole() === "admin" ||
			hasPermission("domain:manage") ||
			hasDomainPermission(uuid(), "domain:manage"),
	);
	const canKick = createMemo(
		() =>
			isOwner() ||
			currentRole() === "admin" ||
			hasPermission("domain:kick") ||
			hasDomainPermission(uuid(), "domain:kick"),
	);
	const canManageRoles = createMemo(
		() =>
			isOwner() ||
			hasPermission("domain:role:manage") ||
			hasDomainPermission(uuid(), "domain:role:manage"),
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
			hasAnyPermission("room:update", "room:delete") ||
			hasDomainPermission(uuid(), "room:update") ||
			hasDomainPermission(uuid(), "room:delete"),
	);
	let createRoomDialogRef!: HTMLDialogElement;
	const [createRoomDomain, setCreateRoomDomain] = createSignal("");
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

	function openCreateRoom() {
		setCreateRoomDomain(uuid());
		createRoomDialogRef?.showModal?.();
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
		void loadGuestState(currentUUID);
		untrack(() => {
			resetRooms();
			void fetchRooms(1);
			void fetchRoles();
			void loadMyPermissions(currentUUID).catch(() => {});
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
							<DomainSettingsForm
								canManage={canManage()}
								domain={domain()}
								name={name}
								setName={setName}
								nameError={nameError}
								setNameError={setNameError}
								description={description}
								setDescription={setDescription}
								isPublic={isPublic}
								setIsPublic={setIsPublic}
								saving={saving}
								formError={formError}
								onSave={handleSave}
							/>{" "}
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
								roles={domainRoles().map((role) => role.name)}
								canChangeRole={canManageRoles()}
								onChangeRole={(userUUID, roleName) =>
									void handleMemberRoleChange(userUUID, roleName)
								}
								onRefresh={() => void loadMembers(uuid()).catch(() => {})}
								onKick={requestKick}
							/>
						</ManageSection>
					</div>

					<ManageSection
						title="访客访问"
						description="配置匿名访客的进入与能力开关"
						class="min-w-0"
					>
						<GuestAccessSettings
							config={guestConfig()}
							loading={guestConfigLoading()}
							canManage={canManage()}
							saving={guestSaving()}
							onSave={(patch) => void handleSaveGuestConfig(patch)}
						/>
					</ManageSection>

					<Show when={canKick()}>
						<ManageSection
							title="访客封禁管理"
							description="封禁在线或历史访客，解封立即生效"
							padded={false}
							class="min-w-0"
							actions={
								<button
									type="button"
									class="btn btn-ghost btn-xs"
									disabled={guestBansLoading()}
									onClick={() => void loadGuestState(uuid())}
								>
									刷新
								</button>
							}
						>
							<div class="flex flex-col gap-3 p-4 border-b border-base-200">
								<div class="flex flex-wrap items-center gap-2">
									<select
										class="select select-bordered select-sm w-56"
										value={banTarget()}
										onChange={(e) => setBanTarget(e.currentTarget.value)}
									>
										<option value="">选择要封禁的访客成员…</option>
										<For each={guestMembers()}>
											{(m) => (
												<option value={m.user_uuid}>
													{memberDisplayName(m)}（{m.user_uuid.slice(0, 8)}）
												</option>
											)}
										</For>
									</select>
									<input
										type="text"
										placeholder="封禁原因"
										class="input input-bordered input-sm w-48"
										value={banReason()}
										onInput={(e) => setBanReason(e.currentTarget.value)}
									/>
									<select
										class="select select-bordered select-sm"
										value={String(banHours())}
										onChange={(e) => setBanHours(Number(e.currentTarget.value))}
									>
										<option value="0">永久</option>
										<option value="1">1 小时</option>
										<option value="24">24 小时</option>
										<option value="168">7 天</option>
									</select>
									<button
										type="button"
										class="btn btn-error btn-sm"
										disabled={!banTarget()}
										onClick={() => void submitBan()}
									>
										封禁
									</button>
								</div>
								<button
									type="button"
									class="btn btn-ghost btn-xs self-start text-base-content/60"
									onClick={() => void handleCleanupGuests()}
								>
									清理 30 天未活跃访客
								</button>
							</div>
							<GuestManageTable
								bans={guestBans()}
								loading={guestBansLoading()}
								canKick={canKick()}
								unbanPending={unbanPending()}
								onUnban={(userUUID) => void handleUnbanGuest(userUUID)}
							/>
						</ManageSection>
					</Show>

					<Show when={canManageRoles()}>
						<ManageSection
							title="角色与权限"
							description="配置域内角色的独立权限"
							padded={false}
							class="min-w-0"
							actions={
								<button
									type="button"
									class="btn btn-ghost btn-xs"
									disabled={rolesLoading()}
									onClick={() => void fetchRoles()}
								>
									刷新
								</button>
							}
						>
							<DomainRolePanel
								roles={domainRoles()}
								assignable={assignableCodes()}
								loading={rolesLoading()}
								saving={rolesSaving()}
								error={rolesError()}
								onCreate={handleCreateRole}
								onUpdate={handleUpdateRole}
								onDelete={handleDeleteRole}
							/>
						</ManageSection>
					</Show>

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
										onClick={openCreateRoom}
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
				domainUUID={createRoomDomain()}
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
