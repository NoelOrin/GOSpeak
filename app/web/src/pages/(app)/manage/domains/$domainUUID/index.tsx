import { Skeleton, toast } from "@gospeak/components";
import { LoadingRegion } from "@/components/common/Skeletons";
import { createFileRoute, Link } from "@tanstack/react-router";
import { ArrowLeft, CirclePlus, Settings2 } from "lucide-react";
import { useEffect, useState } from "react";
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
import {
	type JoinRequestListItem,
	listJoinRequests,
	reviewJoinRequest,
	setJoinApproval,
} from "@/api/inviteJoin";
import { deleteRoom, listRooms, type RoomRecord } from "@/api/room";
import {
	JoinRequestTable,
	type JoinRequestStatusFilter,
} from "@/components/domain/JoinRequestTable";
import ConfirmModal from "@/components/common/ConfirmModal";
import DomainMemberTable, {
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
import { domainStore, useDomainStore } from "@/stores/domainStore";
import { useUserStore } from "@/stores/userStore";
import { hasDomainPermission } from "@/utils/domainPermissions";
import { hasAnyPermission, hasPermission } from "@/utils/permissions";
import { runOptimistic } from "@/utils/optimistic";
import { domainNameSchema, fieldError } from "@/utils/formSchemas";
import { apiErrorMessage } from "@/utils/apiError";
import { toastActionError } from "@/utils/toast";
import { DomainSettingsForm } from "./components/DomainSettingsForm";
import DomainRolePanel from "./components/DomainRolePanel";

export const Route = createFileRoute("/(app)/manage/domains/$domainUUID/")({
	component: RouteComponent,
	staticData: {
		title: "域管理",
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

const ROOM_PAGE_SIZE = 10;

const btnGhostXs =
	"hover:bg-muted inline-flex h-7 cursor-pointer items-center gap-1 rounded-sm px-2 text-xs transition-colors disabled:pointer-events-none disabled:opacity-50";

function RouteComponent() {
	const { domainUUID: uuid } = Route.useParams();

	const domainCache = useDomainStore((s) => s.domainCache);
	const memberCache = useDomainStore((s) => s.memberCache);
	const domainErrors = useDomainStore((s) => s.domainErrors);
	const memberLoadingMap = useDomainStore((s) => s.memberLoading);
	const memberErrorMap = useDomainStore((s) => s.memberErrors);
	const myPermissionsErrorMap = useDomainStore((s) => s.myPermissionsErrors);

	const domain = domainCache[uuid];
	const members = memberCache[uuid] ?? [];
	const domainError = domainErrors[uuid] ?? "";
	const memberLoading = memberLoadingMap[uuid] ?? !memberCache[uuid];
	const memberError = memberErrorMap[uuid] ?? "";
	const myPermissionsError = myPermissionsErrorMap[uuid] ?? "";

	const currentUser = useUserStore((s) => s.user);

	// 本页列表数据来自 zustand / useState（不在 React Query 缓存里），
	// 因此乐观更新直接对状态容器走「快照 → 改 → 失败回滚 → 与服务端对账」。
	const memberSnapshot = () => useDomainStore.getState().memberCache[uuid];
	const writeMembers = (value: DomainMember[] | undefined) =>
		domainStore.setMemberCache(uuid, value);
	const reconcileMembers = () =>
		domainStore.loadMembers(uuid).catch(() => {
			toast.warning("操作已生效，但成员列表刷新失败");
		});

	const [domainRoles, setDomainRoles] = useState<DomainRole[]>([]);
	const [assignableCodes, setAssignableCodes] = useState<string[]>([]);
	const [rolesLoading, setRolesLoading] = useState(true);
	const [rolesError, setRolesError] = useState("");
	const [rolesSaving, setRolesSaving] = useState(false);

	async function fetchRoles() {
		setRolesLoading(true);
		setRolesError("");
		try {
			const data = await listDomainRoles(uuid);
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
		const previous = domainRoles;
		try {
			await runOptimistic({
				snapshot: () => previous,
				apply: () =>
					setDomainRoles((prev) =>
						prev.some((r) => r.name === name)
							? prev
							: [...prev, { name, is_system: false, permissions }],
					),
				restore: (snap) => setDomainRoles(snap),
				request: () => createDomainRole(uuid, name, permissions),
				settle: fetchRoles,
				onError: (error) =>
					setRolesError(toastActionError(error, "创建角色失败")),
				onSuccess: () => toast.success(`角色「${name}」已创建`),
			});
		} finally {
			setRolesSaving(false);
		}
	}

	async function handleUpdateRole(roleName: string, permissions: string[]) {
		setRolesSaving(true);
		setRolesError("");
		const previous = domainRoles;
		try {
			await runOptimistic({
				snapshot: () => previous,
				apply: () =>
					setDomainRoles((prev) =>
						prev.map((r) => (r.name === roleName ? { ...r, permissions } : r)),
					),
				restore: (snap) => setDomainRoles(snap),
				request: () => updateDomainRolePermissions(uuid, roleName, permissions),
				settle: async () => {
					await fetchRoles();
					try {
						await domainStore.loadMyPermissions(uuid);
					} catch {
						// 权限刷新失败不覆盖角色保存成功的结果
					}
				},
				onError: (error) =>
					setRolesError(toastActionError(error, "保存角色权限失败")),
				onSuccess: () => toast.success("角色权限已保存"),
			});
		} finally {
			setRolesSaving(false);
		}
	}

	async function handleDeleteRole(roleName: string) {
		// 删除角色会丢掉该角色的全部权限配置，先确认（原先点一下就删）
		setPendingDeleteRole(roleName);
	}

	async function confirmDeleteRole() {
		const roleName = pendingDeleteRole;
		if (!roleName) return;
		setRolesSaving(true);
		setRolesError("");
		const previous = domainRoles;
		try {
			await runOptimistic({
				snapshot: () => previous,
				apply: () =>
					setDomainRoles((prev) => prev.filter((r) => r.name !== roleName)),
				restore: (snap) => setDomainRoles(snap),
				request: () => deleteDomainRole(uuid, roleName),
				settle: fetchRoles,
				onError: (error) =>
					setRolesError(toastActionError(error, "删除角色失败")),
				onSuccess: () => {
					toast.success(`角色「${roleName}」已删除`);
					setPendingDeleteRole(null);
				},
			});
		} finally {
			setRolesSaving(false);
		}
	}

	async function handleMemberRoleChange(userUUID: string, roleName: string) {
		await runOptimistic({
			snapshot: memberSnapshot,
			apply: () =>
				writeMembers(
					(memberSnapshot() ?? []).map((m) =>
						m.user_uuid === userUUID ? { ...m, role_name: roleName } : m,
					),
				),
			restore: writeMembers,
			request: () => updateDomainMemberRole(uuid, userUUID, roleName),
			settle: reconcileMembers,
			onError: (error) => toastActionError(error, "成员角色更新失败"),
			onSuccess: () => toast.success("成员角色已更新"),
		});
	}

	const currentRole =
		members.find((member) => member.user_uuid === currentUser?.uuid)
			?.role_name ?? "";
	const isOwner = !!currentUser && domain?.owner_uuid === currentUser.uuid;
	const canManage =
		isOwner ||
		currentRole === "admin" ||
		hasPermission("domain:manage") ||
		hasDomainPermission(uuid, "domain:manage");
	const canKick =
		isOwner ||
		currentRole === "admin" ||
		hasPermission("domain:kick") ||
		hasDomainPermission(uuid, "domain:kick");
	const canReviewRequests =
		isOwner ||
		currentRole === "admin" ||
		hasPermission("domain:member:review") ||
		hasDomainPermission(uuid, "domain:member:review");

	const canManageRoles =
		isOwner ||
		hasPermission("domain:role:manage") ||
		hasDomainPermission(uuid, "domain:role:manage");

	const [pendingDeleteRole, setPendingDeleteRole] = useState<string | null>(
		null,
	);

	const canManageRooms =
		isOwner ||
		currentRole === "admin" ||
		hasAnyPermission("room:update", "room:delete") ||
		hasDomainPermission(uuid, "room:update") ||
		hasDomainPermission(uuid, "room:delete");

	// ── 房间管理 ──
	const [rooms, setRooms] = useState<RoomRecord[]>([]);
	const [roomTotal, setRoomTotal] = useState(0);
	const [roomPage, setRoomPage] = useState(1);
	const [roomLoading, setRoomLoading] = useState(true);
	const [roomRefreshing, setRoomRefreshing] = useState(false);
	const [roomError, setRoomError] = useState("");
	const [editingRoom, setEditingRoom] = useState<RoomRecord | null>(null);
	const [deletingRoom, setDeletingRoom] = useState<RoomRecord | null>(null);
	const [deleting, setDeleting] = useState(false);
	const [deleteError, setDeleteError] = useState("");
	const totalRoomPages = Math.max(1, Math.ceil(roomTotal / ROOM_PAGE_SIZE));
	const [createRoomOpen, setCreateRoomOpen] = useState(false);

	// ── 域设置表单 ──
	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [isPublic, setIsPublic] = useState(false);
	const [nameError, setNameError] = useState("");
	const [formError, setFormError] = useState("");
	const [saving, setSaving] = useState(false);
	const [requireJoinApproval, setRequireJoinApproval] = useState(false);
	const [_approvalSaving, setApprovalSaving] = useState(false);

	// ── 加入申请 ──
	const [joinRequests, setJoinRequests] = useState<JoinRequestListItem[]>([]);
	const [joinRequestTotal, setJoinRequestTotal] = useState(0);
	const [joinRequestCounts, setJoinRequestCounts] = useState<
		Record<string, number>
	>({ pending: 0, approved: 0, rejected: 0 });
	const [joinRequestStatus, setJoinRequestStatus] =
		useState<JoinRequestStatusFilter>("pending");
	const [joinRequestKeyword, setJoinRequestKeyword] = useState("");
	const [joinRequestPage, setJoinRequestPage] = useState(1);
	const [joinRequestLoading, setJoinRequestLoading] = useState(false);
	const [joinRequestError, setJoinRequestError] = useState("");
	const [reviewingRequestID, setReviewingRequestID] = useState(0);

	const [kickTarget, setKickTarget] = useState<DomainMember | null>(null);
	const [kicking, setKicking] = useState(false);
	const [kickError, setKickError] = useState("");

	async function fetchRooms(page: number) {
		setRoomError("");
		if (rooms.length > 0) setRoomRefreshing(true);
		else setRoomLoading(true);
		try {
			const result = await listRooms(page, ROOM_PAGE_SIZE, uuid);
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

	async function handleDeleteRoom() {
		const room = deletingRoom;
		if (!room) return;
		setDeleting(true);
		setDeleteError("");
		const previous = { rooms, total: roomTotal };
		try {
			await runOptimistic({
				snapshot: () => previous,
				apply: () => {
					setRooms((prev) => prev.filter((r) => r.id !== room.id));
					setRoomTotal((prev) => Math.max(0, prev - 1));
				},
				restore: (snap) => {
					setRooms(snap.rooms);
					setRoomTotal(snap.total);
				},
				request: () => deleteRoom(room.id),
				settle: () => fetchRooms(roomPage),
				onError: (error) =>
					setDeleteError(toastActionError(error, "删除房间失败")),
				onSuccess: () => {
					setDeletingRoom(null);
					toast.success("房间已删除");
				},
			});
		} finally {
			setDeleting(false);
		}
	}

	// 域切换：加载成员/访客/房间/角色（对应 Solid createEffect）
	const [loadedUUID, setLoadedUUID] = useState("");
	// biome-ignore lint/correctness/useExhaustiveDependencies: 按 uuid/loadedUUID 触发一次数据加载；内部 fetch 函数每次渲染新建，加入依赖会造成重复请求
	useEffect(() => {
		if (loadedUUID === uuid) return;
		setLoadedUUID(uuid);
		domainStore.setCurrentDomain(uuid);
		void domainStore.loadMembers(uuid).catch(() => {});
		resetRooms();
		void fetchRooms(1);
		void fetchRoles();
		if (canReviewRequests) {
			void fetchJoinRequests("pending", "", 1);
		}
		void domainStore.loadMyPermissions(uuid).catch(() => {});
	}, [uuid, loadedUUID]);

	useEffect(() => {
		if (domain) {
			document.title = `${domain.name} 管理 | GOSpeak`;
		}
	}, [domain]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: 仅在 domain 对象变化时用非空值初始化表单，setter 稳定且不应覆盖用户已输入内容
	useEffect(() => {
		if (!domain || loadedUUID !== domain.uuid) return;
		setName((prev) => prev || domain.name);
		setDescription((prev) => prev || domain.description || "");
		setIsPublic(domain.is_public);
		setRequireJoinApproval(!!domain.require_join_approval);
	}, [domain]);

	async function fetchJoinRequests(
		status: JoinRequestStatusFilter,
		keyword: string,
		page: number,
	) {
		if (!canReviewRequests) return;
		setJoinRequestLoading(true);
		setJoinRequestError("");
		try {
			const result = await listJoinRequests({
				domain_uuid: uuid,
				status,
				keyword,
				page,
				page_size: 10,
			});
			setJoinRequests(result.items);
			setJoinRequestTotal(result.total);
			setJoinRequestCounts(result.counts);
			setJoinRequestPage(page);
		} catch (error) {
			setJoinRequestError(toastActionError(error, "加入申请加载失败"));
		} finally {
			setJoinRequestLoading(false);
		}
	}

	async function handleReviewRequest(requestID: number, approve: boolean) {
		setReviewingRequestID(requestID);
		try {
			await reviewJoinRequest(uuid, requestID, approve);
			toast.success(approve ? "已通过加入申请" : "已拒绝加入申请");
			await fetchJoinRequests(
				joinRequestStatus,
				joinRequestKeyword,
				joinRequestPage,
			);
			if (approve) {
				await domainStore.loadMembers(uuid).catch(() => {});
			}
		} catch (error) {
			toastActionError(error, "审核失败，请稍后重试");
		} finally {
			setReviewingRequestID(0);
		}
	}

	async function handleToggleApproval(next: boolean) {
		setApprovalSaving(true);
		try {
			const result = await setJoinApproval(uuid, next);
			setRequireJoinApproval(result.require_join_approval);
			const cached = useDomainStore.getState().domainCache[uuid];
			if (cached) {
				domainStore.updateCachedDomain({
					...cached,
					require_join_approval: result.require_join_approval,
				});
			}
			toast.success(
				result.require_join_approval ? "已开启加入审核" : "已关闭加入审核",
			);
		} catch (error) {
			toastActionError(error, "切换加入审核失败");
		} finally {
			setApprovalSaving(false);
		}
	}

	async function handleSave(e: React.FormEvent) {
		e.preventDefault();
		if (!canManage) {
			setFormError("无管理权限");
			return;
		}

		const errors = validateDomainForm(name);
		setNameError(errors.name ?? "");
		if (errors.name) {
			setFormError("请修正表单错误");
			return;
		}

		setSaving(true);
		setFormError("");
		try {
			const updated = await updateDomain({
				domain_uuid: uuid,
				name: name.trim(),
				description: description.trim(),
				is_public: isPublic,
			});
			domainStore.updateCachedDomain(updated);
			setName(updated.name);
			setDescription(updated.description ?? "");
			setIsPublic(updated.is_public);
			toast.success("域设置已保存");
		} catch (error) {
			const message = toastActionError(error, "保存域设置失败");
			setFormError(message);
		} finally {
			setSaving(false);
		}
	}

	async function handleKick() {
		const target = kickTarget;
		if (!target) return;
		setKicking(true);
		setKickError("");
		try {
			await runOptimistic({
				snapshot: memberSnapshot,
				apply: () =>
					writeMembers(
						(memberSnapshot() ?? []).filter(
							(m) => m.user_uuid !== target.user_uuid,
						),
					),
				restore: writeMembers,
				request: () => kickDomainMember(uuid, target.user_uuid),
				settle: reconcileMembers,
				onError: (error) =>
					setKickError(toastActionError(error, "移出成员失败")),
				onSuccess: () => {
					setKickTarget(null);
					toast.success("成员已移出域");
				},
			});
		} finally {
			setKicking(false);
		}
	}

	function retryDomain() {
		domainStore.setCurrentDomain(uuid);
	}

	return (
		<div className="min-h-full w-full flex-1 overflow-y-auto">
			<ManagePage className="min-h-full w-full">
				<ManageHeader
					icon={<Settings2 size={20} />}
					title={domain?.name || "域管理"}
					description={domain?.description || "管理域设置与成员"}
					actions={
						<Link
							to="/domain/$domainUUID"
							params={{ domainUUID: uuid }}
							className="hover:bg-muted inline-flex h-8 cursor-pointer items-center gap-1.5 rounded-sm px-2 text-sm transition-colors"
						>
							<ArrowLeft size={16} />
							返回
						</Link>
					}
				/>

				{myPermissionsError && (
					<div
						role="alert"
						className="border-warning/40 bg-warning/10 text-warning mb-1 flex flex-wrap items-center gap-2 rounded-sm border px-3 py-2 text-xs"
					>
						<span>{myPermissionsError}</span>
						<button
							type="button"
							className="underline underline-offset-2"
							onClick={() =>
								void domainStore.loadMyPermissions(uuid).catch(() => {})
							}
						>
							重试
						</button>
					</div>
				)}

				{domain && (
					<>
						<div className="grid min-w-0 gap-5 lg:grid-cols-[minmax(360px,400px)_minmax(0,1fr)]">
							<ManageSection
								title="域设置"
								description="调整域名称、描述与公开状态"
								className="min-w-0"
							>
								<DomainSettingsForm
									canManage={canManage}
									domain={domain}
									name={name}
									setName={setName}
									nameError={nameError}
									setNameError={setNameError}
									description={description}
									setDescription={setDescription}
									isPublic={isPublic}
									setIsPublic={setIsPublic}
									requireJoinApproval={requireJoinApproval}
									setRequireJoinApproval={(next) =>
										void handleToggleApproval(next)
									}
									saving={saving}
									formError={formError}
									onSave={handleSave}
								/>
							</ManageSection>

							<ManageSection
								title="成员管理"
								description={`${members.length} 位成员`}
								className="min-w-0"
								padded={false}
								actions={
									<button
										type="button"
										className={btnGhostXs}
										disabled={memberLoading || kicking}
										onClick={() =>
											void domainStore.loadMembers(uuid).catch(() => {})
										}
									>
										刷新
									</button>
								}
							>
								<DomainMemberTable
									members={members}
									ownerUUID={domain?.owner_uuid}
									currentUserUUID={currentUser?.uuid}
									canKick={canKick}
									kickDisabled={kicking}
									loading={memberLoading && members.length === 0}
									refreshing={(memberLoading || kicking) && members.length > 0}
									error={memberError}
									roles={domainRoles.map((role) => role.name)}
									canChangeRole={canManageRoles}
									onChangeRole={(userUUID, roleName) =>
										void handleMemberRoleChange(userUUID, roleName).catch(
											(error) => toastActionError(error, "成员角色更新失败"),
										)
									}
									onRefresh={() =>
										void domainStore.loadMembers(uuid).catch(() => {})
									}
									onKick={(userUUID) => {
										const target = members.find(
											(member) => member.user_uuid === userUUID,
										);
										if (!target) return;
										setKickError("");
										setKickTarget(target);
									}}
								/>
							</ManageSection>
						</div>

						{canReviewRequests && (
							<ManageSection
								title="加入申请"
								description={`${joinRequestCounts.pending ?? 0} 条待审核`}
								className="min-w-0"
							>
								<JoinRequestTable
									items={joinRequests}
									total={joinRequestTotal}
									counts={joinRequestCounts}
									status={joinRequestStatus}
									keyword={joinRequestKeyword}
									page={joinRequestPage}
									pageSize={10}
									loading={joinRequestLoading}
									error={joinRequestError}
									reviewing={reviewingRequestID}
									onStatusChange={(status) => {
										setJoinRequestStatus(status);
										void fetchJoinRequests(status, joinRequestKeyword, 1);
									}}
									onKeywordChange={setJoinRequestKeyword}
									onSearch={() =>
										void fetchJoinRequests(
											joinRequestStatus,
											joinRequestKeyword,
											1,
										)
									}
									onPageChange={(page) =>
										void fetchJoinRequests(
											joinRequestStatus,
											joinRequestKeyword,
											page,
										)
									}
									onReview={(id, approve) =>
										void handleReviewRequest(id, approve)
									}
								/>
							</ManageSection>
						)}

						{canManageRoles && (
							<ManageSection
								title="角色与权限"
								description="配置域内角色的独立权限"
								padded={false}
								className="min-w-0"
								actions={
									<button
										type="button"
										className={btnGhostXs}
										disabled={rolesLoading}
										onClick={() => void fetchRoles()}
									>
										刷新
									</button>
								}
							>
								<DomainRolePanel
									roles={domainRoles}
									assignable={assignableCodes}
									loading={rolesLoading}
									saving={rolesSaving}
									error={rolesError}
									onCreate={handleCreateRole}
									onUpdate={handleUpdateRole}
									onDelete={handleDeleteRole}
								/>
							</ManageSection>
						)}

						<ManageSection
							title="房间管理"
							description={`${roomTotal} 个房间`}
							padded={false}
							actions={
								<>
									<button
										type="button"
										className={btnGhostXs}
										disabled={roomLoading || roomRefreshing}
										onClick={() => {
											resetRooms();
											void fetchRooms(1);
										}}
									>
										刷新
									</button>
									{canManageRooms && (
										<button
											type="button"
											className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-7 cursor-pointer items-center gap-1 rounded-sm px-2 text-xs transition-opacity"
											onClick={() => setCreateRoomOpen(true)}
										>
											<CirclePlus size={14} />
											新建房间
										</button>
									)}
								</>
							}
						>
							<DomainRoomTable
								rooms={rooms}
								currentUserName={currentUser?.name}
								canManage={canManageRooms}
								loading={roomLoading}
								refreshing={roomRefreshing}
								error={roomError}
								page={roomPage}
								totalPages={totalRoomPages}
								onPageChange={(page) => void fetchRooms(page)}
								onRefresh={() => {
									resetRooms();
									void fetchRooms(1);
								}}
								onEdit={setEditingRoom}
								onDelete={setDeletingRoom}
							/>
						</ManageSection>
					</>
				)}

				{!domain && domainError && (
					<div className="flex flex-col items-center gap-4 py-16 text-center">
						<p role="alert" className="text-destructive">
							域加载失败：{domainError}
						</p>
						<button
							type="button"
							className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-8 cursor-pointer items-center rounded-sm px-3 text-sm transition-opacity"
							onClick={retryDomain}
						>
							重试
						</button>
					</div>
				)}
				{!domain && !domainError && (
					<LoadingRegion label="正在加载域">
						<div className="flex flex-col gap-4 p-2">
							<Skeleton className="h-8 w-56" />
							<Skeleton className="h-28 w-full rounded-lg" />
							<Skeleton className="h-28 w-full rounded-lg" />
						</div>
					</LoadingRegion>
				)}
			</ManagePage>

			<ConfirmModal
				open={!!kickTarget}
				title="移出成员"
				message={
					<span>
						确认将 {kickTarget ? memberDisplayName(kickTarget) : "该成员"}{" "}
						移出域？
						{kickError && (
							<span className="text-destructive mt-2 block">{kickError}</span>
						)}
					</span>
				}
				confirmText="移出"
				loading={kicking}
				onClose={() => {
					setKickTarget(null);
					setKickError("");
				}}
				onConfirm={handleKick}
			/>
			<CreateRoomModal
				open={createRoomOpen}
				domainUUID={uuid}
				onClose={() => setCreateRoomOpen(false)}
				onCreated={() => {
					resetRooms();
					void fetchRooms(1);
				}}
			/>
			{editingRoom && (
				<EditRoomModal
					open
					room={editingRoom}
					onClose={() => setEditingRoom(null)}
					onSaved={() => {
						void fetchRooms(roomPage);
					}}
				/>
			)}
			<ConfirmModal
				open={!!deletingRoom}
				title="删除房间"
				message={
					<span>
						确认删除房间 {deletingRoom?.name || "该房间"}？删除后不可恢复。
						{deleteError && (
							<span className="text-destructive mt-2 block">{deleteError}</span>
						)}
					</span>
				}
				confirmText="删除"
				loading={deleting}
				onClose={() => {
					setDeletingRoom(null);
					setDeleteError("");
				}}
				onConfirm={handleDeleteRoom}
			/>
			<ConfirmModal
				open={!!pendingDeleteRole}
				title="删除角色"
				message={
					<span>
						确认删除角色「{pendingDeleteRole}」？该角色已配置的权限会一并移除。
					</span>
				}
				confirmText="删除"
				loading={rolesSaving}
				onClose={() => setPendingDeleteRole(null)}
				onConfirm={confirmDeleteRole}
			/>
		</div>
	);
}
