import {
	Button,
	Dialog,
	DialogContent,
	DialogTitle,
	toast,
} from "@gospeak/components";
import { useNavigate } from "@tanstack/react-router";
import {
	Copy,
	LogOut,
	MoreHorizontal,
	Settings,
	Share2,
	Trash2,
} from "lucide-react";
import { useEffect, useState } from "react";
import ConfirmModal from "@/components/common/ConfirmModal";
import { listInvites } from "@/api/invite";
import InviteShareModal from "@/components/domain/InviteShareModal";
import { domainStore, useDomainStore } from "@/stores/domainStore";
import { useUserStore } from "@/stores/userStore";
import { domainInviteUrl } from "@/utils/domainInvite";
import { usePublicBaseUrl } from "@/utils/publicBaseUrl";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";

/**
 * 域级操作菜单（邀请 / 管理 / 离开 / 删除）。
 *
 * 这些操作原本只渲染在 `pages/(app)/domain.$domainUUID.tsx` 的 children 里，
 * 而布局在域路由下把 children 隐藏了（桌面 display:none、移动端不渲染），
 * 导致「离开域 / 删除域」在界面上完全没有入口。这里把它们挂到房间列表头部。
 */
export default function DomainMenu({ domainUUID }: { domainUUID: string }) {
	const navigate = useNavigate();
	const [open, setOpen] = useState(false);
	const [shareOpen, setShareOpen] = useState(false);
	const [confirm, setConfirm] = useState<"leave" | "delete" | null>(null);
	const [busy, setBusy] = useState(false);
	const [actionError, setActionError] = useState("");

	const user = useUserStore((s) => s.user);
	const domainCache = useDomainStore((s) => s.domainCache);
	const memberCache = useDomainStore((s) => s.memberCache);
	const myDomainUUIDs = useDomainStore((s) => s.myDomainUUIDs);

	const domain = domainCache[domainUUID];
	const isOwner = !!user && domain?.owner_uuid === user.uuid;
	const isJoined = myDomainUUIDs.includes(domainUUID);
	const currentRole = memberCache[domainUUID]?.find(
		(m) => m.user_uuid === user?.uuid,
	)?.role_name;
	const canManage =
		isOwner || currentRole === "admin" || hasPermission("domain:manage");

	const [inviteCode, setInviteCode] = useState("");
	const publicBase = usePublicBaseUrl();

	// 邀请码已从 domains 表迁到 domain_invites 表：菜单打开时按需拉取该域第一个可用码。
	useEffect(() => {
		if (!open || !domainUUID) return;
		let cancelled = false;
		void listInvites(domainUUID)
			.then((list) => {
				if (cancelled) return;
				const usable = list.find((item) => !item.disabled_at);
				setInviteCode(usable?.code ?? "");
			})
			.catch(() => {
				if (!cancelled) setInviteCode("");
			});
		return () => {
			cancelled = true;
		};
	}, [open, domainUUID]);
	const inviteUrl = inviteCode ? domainInviteUrl(inviteCode, publicBase) : "";

	async function copyInvite() {
		if (!inviteCode) return;
		try {
			await navigator.clipboard.writeText(inviteCode);
			toast.success("邀请码已复制");
		} catch {
			toast.error("复制失败，请手动复制邀请码");
		}
	}

	async function handleLeave() {
		setBusy(true);
		setActionError("");
		try {
			await domainStore.leaveAndClear(domainUUID);
			setConfirm(null);
			setOpen(false);
			navigate({ to: "/discover" });
			toast.success("已离开该域");
		} catch (e) {
			setActionError(toastActionError(e, "离开域失败"));
		} finally {
			setBusy(false);
		}
	}

	async function handleDelete() {
		setBusy(true);
		setActionError("");
		try {
			await domainStore.deleteAndClear(domainUUID);
			setConfirm(null);
			setOpen(false);
			navigate({ to: "/discover" });
			toast.success("域已删除");
		} catch (e) {
			setActionError(toastActionError(e, "删除域失败"));
		} finally {
			setBusy(false);
		}
	}

	return (
		<>
			<Button
				variant="ghost"
				size="sm"
				className="size-7 p-0"
				title="域设置"
				aria-label="域设置"
				onClick={() => setOpen(true)}
			>
				<MoreHorizontal size={16} />
			</Button>

			<Dialog open={open} onOpenChange={setOpen}>
				<DialogContent className="max-w-md">
					<DialogTitle className="text-lg font-bold">
						{domain?.name ?? "域"}
					</DialogTitle>
					{domain?.description && (
						<p className="text-muted-foreground -mt-2 text-sm leading-6">
							{domain.description}
						</p>
					)}

					{inviteCode && (
						<div className="border-border flex items-center gap-2 rounded-sm border px-3 py-2">
							<span className="text-muted-foreground text-xs">邀请码</span>
							<code className="flex-1 truncate text-sm">{inviteCode}</code>
							<Button
								variant="ghost"
								size="icon-xs"
								title="复制邀请码"
								aria-label="复制邀请码"
								onClick={() => void copyInvite()}
							>
								<Copy size={14} />
							</Button>
						</div>
					)}

					<div className="flex flex-col gap-1.5">
						{inviteUrl && (
							<Button
								variant="outline"
								className="justify-start gap-2"
								onClick={() => {
									setOpen(false);
									setShareOpen(true);
								}}
							>
								<Share2 size={16} />
								分享邀请
							</Button>
						)}
						{canManage && (
							<Button
								variant="outline"
								className="justify-start gap-2"
								onClick={() => {
									setOpen(false);
									navigate({
										to: "/manage/domains/$domainUUID",
										params: { domainUUID },
									});
								}}
							>
								<Settings size={16} />
								域管理
							</Button>
						)}
						{isJoined && !isOwner && (
							<Button
								variant="outline"
								className="text-destructive justify-start gap-2"
								onClick={() => {
									setActionError("");
									setConfirm("leave");
								}}
							>
								<LogOut size={16} />
								离开域
							</Button>
						)}
						{isOwner && (
							<Button
								variant="outline"
								className="text-destructive justify-start gap-2"
								onClick={() => {
									setActionError("");
									setConfirm("delete");
								}}
							>
								<Trash2 size={16} />
								删除域
							</Button>
						)}
					</div>
				</DialogContent>
			</Dialog>

			{inviteCode && (
				<InviteShareModal
					open={shareOpen}
					inviteUrl={inviteUrl}
					onClose={() => setShareOpen(false)}
				/>
			)}

			<ConfirmModal
				open={confirm === "leave"}
				title="离开域"
				message={
					<span>
						确认离开「{domain?.name}」？离开后需要重新通过邀请码加入。
					</span>
				}
				confirmText="离开"
				confirmVariant="destructive"
				loading={busy}
				onClose={() => setConfirm(null)}
				onConfirm={handleLeave}
			/>

			<ConfirmModal
				open={confirm === "delete"}
				title="删除域"
				message={
					<span>
						删除「{domain?.name}
						」会永久移除该域的所有房间、消息、成员与角色配置， 此操作不可撤销。
						{actionError && (
							<span className="text-destructive mt-2 block">{actionError}</span>
						)}
					</span>
				}
				confirmText="永久删除"
				confirmPhrase={domain?.name}
				loading={busy}
				onClose={() => setConfirm(null)}
				onConfirm={handleDelete}
			/>
		</>
	);
}
