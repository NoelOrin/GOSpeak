import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { showToast } from "solid-notifications";
import Link from "lucide-solid/icons/link";
import Copy from "lucide-solid/icons/copy";
import Check from "lucide-solid/icons/check";
import ArrowRight from "lucide-solid/icons/arrow-right";
import RefreshCw from "lucide-solid/icons/refresh-cw";
import { createResource, createSignal, For, Show, onCleanup } from "solid-js";
import {
	type DomainDetail,
	getMyDomainsDetailed,
	resetDomainInviteCode,
} from "@/api/domain";
import { domainInviteUrl, extractDomainInviteCode } from "@/utils/domainInvite";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import DomainIcon from "@/components/domain/DomainIcon";
import InviteShareModal from "@/components/domain/InviteShareModal";
import ConfirmModal from "@/components/common/ConfirmModal";
import domainStore from "@/stores/domainStore";

export const Route = createFileRoute("/(app)/link/")({
	validateSearch: (search: Record<string, unknown>) => ({
		domain: typeof search.domain === "string" ? search.domain : undefined,
	}),
	component: RouteComponent,
	staticData: { title: "分享链接" },
});

function RouteComponent() {
	const navigate = useNavigate();
	const search = Route.useSearch();
	const { loadMyDomains, updateCachedDomain } = domainStore;
	void loadMyDomains().catch(() => {});

	const [domains, { refetch }] = createResource<DomainDetail[]>(() =>
		getMyDomainsDetailed(),
	);
	const [copiedCode, setCopiedCode] = createSignal<string | null>(null);
	const [copiedLink, setCopiedLink] = createSignal<string | null>(null);
	const [shareDomain, setShareDomain] = createSignal<DomainDetail | null>(null);
	const [shareRef, setShareRef] = createSignal<HTMLDialogElement>();

	const [resetTarget, setResetTarget] = createSignal<DomainDetail | null>(null);
	const [resetting, setResetting] = createSignal(false);
	let confirmDialogRef!: HTMLDialogElement;

	const inviteCode = (d: DomainDetail) =>
		extractDomainInviteCode(d.invite_code || "") || "";
	const inviteUrl = (d: DomainDetail) => domainInviteUrl(inviteCode(d));

	let copyCodeTimer: ReturnType<typeof setTimeout> | undefined;
	let copyLinkTimer: ReturnType<typeof setTimeout> | undefined;
	onCleanup(() => {
		if (copyCodeTimer !== undefined) clearTimeout(copyCodeTimer);
		if (copyLinkTimer !== undefined) clearTimeout(copyLinkTimer);
	});
	const copy = async (text: string, kind: "code" | "link", id: string) => {
		try {
			await navigator.clipboard.writeText(text);
			if (kind === "code") {
				setCopiedCode(id);
				if (copyCodeTimer !== undefined) clearTimeout(copyCodeTimer);
				copyCodeTimer = setTimeout(
					() => setCopiedCode((c) => (c === id ? null : c)),
					2000,
				);
			} else {
				setCopiedLink(id);
				if (copyLinkTimer !== undefined) clearTimeout(copyLinkTimer);
				copyLinkTimer = setTimeout(
					() => setCopiedLink((c) => (c === id ? null : c)),
					2000,
				);
			}
		} catch {
			// 剪贴板不可用时由用户手动复制，不阻断操作
		}
	};

	const openShare = (d: DomainDetail) => {
		setShareDomain(d);
		requestAnimationFrame(() => {
			const dlg = shareRef();
			if (dlg) dlg.showModal();
			else requestAnimationFrame(() => shareRef()?.showModal());
		});
	};

	const shareUrl = () => {
		const d = shareDomain();
		return d ? inviteUrl(d) : "";
	};

	const apiErrorMessage = (error: unknown): string => {
		const msg = (error as { response?: { data?: { msg?: string } } })?.response
			?.data?.msg;
		if (msg) return msg;
		if (error instanceof Error) return error.message;
		return "重置邀请码失败";
	};

	const requestReset = (d: DomainDetail) => {
		setResetTarget(d);
		requestAnimationFrame(() => {
			if (confirmDialogRef) confirmDialogRef.showModal();
			else requestAnimationFrame(() => confirmDialogRef?.showModal());
		});
	};

	const closeResetModal = () => {
		if (resetting()) return;
		confirmDialogRef?.close();
		setResetTarget(null);
	};

	const confirmReset = async () => {
		const d = resetTarget();
		if (!d) return;
		setResetting(true);
		try {
			const updated = await resetDomainInviteCode(d.uuid);
			updateCachedDomain(updated);
			await refetch();
			showToast("邀请码已重置，旧链接已失效", { type: "success" });
			confirmDialogRef?.close();
			setResetTarget(null);
		} catch (e) {
			showToast(apiErrorMessage(e), { type: "error" });
		} finally {
			setResetting(false);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Link size={18} />}
				title="分享链接"
				description="管理你所在域的邀请链接，复制后分享给好友即可加入对应语音域。"
			/>

			<Show when={domains.loading}>
				<ManageLoading label="加载域列表..." />
			</Show>

			<Show when={domains.error}>
				<div role="alert" class="alert alert-error">
					加载域列表失败，请稍后重试
				</div>
			</Show>

			<Show when={domains()}>
				<Show
					when={(domains()?.length ?? 0) > 0}
					fallback={
						<ManageSection>
							<div class="flex flex-col items-center justify-center gap-2 py-10 text-base-content/50">
								<Link size={28} />
								<p class="text-sm">你还没有加入任何域，暂无邀请链接可分享。</p>
							</div>
						</ManageSection>
					}
				>
					<For each={domains()}>
						{(d) => (
							<ManageSection
								class={
									search().domain === d.uuid ? "ring-2 ring-primary/40" : ""
								}
							>
								<div class="flex items-center gap-3">
									<DomainIcon name={d.name} iconUrl={d.icon_url} />
									<div class="min-w-0 flex-1">
										<div class="truncate font-semibold">{d.name}</div>
										<div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-base-content/50">
											<Show when={d.is_public}>
												<span class="badge badge-outline badge-xs">公开</span>
											</Show>
											<span>成员 {d.member_count}</span>
											<Show when={inviteCode(d)}>
												<span>
													邀请码{" "}
													<code class="text-base-content/70">
														{inviteCode(d)}
													</code>
												</span>
											</Show>
										</div>
									</div>
								</div>

								<div class="mt-4 flex flex-wrap items-center gap-2">
									<Show when={inviteCode(d)}>
										<button
											type="button"
											class="btn btn-sm btn-outline"
											onClick={() => copy(inviteCode(d), "code", d.uuid)}
										>
											{copiedCode() === d.uuid ? (
												<Check size={15} />
											) : (
												<Copy size={15} />
											)}
											{copiedCode() === d.uuid ? "已复制" : "复制邀请码"}
										</button>
										<button
											type="button"
											class="btn btn-sm btn-outline"
											onClick={() => copy(inviteUrl(d), "link", d.uuid)}
										>
											{copiedLink() === d.uuid ? (
												<Check size={15} />
											) : (
												<Copy size={15} />
											)}
											{copiedLink() === d.uuid ? "已复制" : "复制邀请链接"}
										</button>
										<button
											type="button"
											class="btn btn-sm btn-primary"
											onClick={() => openShare(d)}
										>
											<Link size={15} />
											分享
										</button>
									</Show>
									<button
										type="button"
										class="btn btn-sm"
										onClick={() =>
											navigate({
												to: "/domain/$domainUUID",
												params: { domainUUID: d.uuid },
											})
										}
									>
										进入域
										<ArrowRight size={15} />
									</button>
									<button
										type="button"
										class="btn btn-sm btn-outline"
										onClick={() => requestReset(d)}
									>
										<RefreshCw size={15} />
										重置邀请码
									</button>
								</div>
							</ManageSection>
						)}
					</For>
				</Show>

				<InviteShareModal
					ref={setShareRef}
					inviteUrl={shareUrl()}
					onClose={() => {
						shareRef()?.close();
						setShareDomain(null);
					}}
				/>

				<ConfirmModal
					open={resetTarget() !== null}
					title="重置邀请码"
					message={
						<>
							确定要重置{" "}
							<span class="font-semibold">{resetTarget()?.name}</span>{" "}
							的邀请码吗？旧邀请链接将立即失效，已分享的链接无法再使用。
						</>
					}
					confirmText="重置"
					confirmClass="btn btn-warning"
					loading={resetting()}
					dialogRef={(el) => {
						confirmDialogRef = el;
					}}
					onClose={closeResetModal}
					onConfirm={confirmReset}
				/>
			</Show>
		</ManagePage>
	);
}

export default RouteComponent;
