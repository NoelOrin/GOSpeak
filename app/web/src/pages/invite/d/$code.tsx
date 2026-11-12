import { Button, Skeleton, toast } from "@gospeak/components";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { sendEmailCode } from "@/api/email";
import { LoadingRegion } from "@/components/common/Skeletons";
import {
	cancelJoinRequest,
	getJoinRequest,
	type InviteJoinResult,
	type InvitePreview,
	joinByCode,
	previewInvite,
	registerViaInvite,
} from "@/api/inviteJoin";
import {
	InviteRegisterForm,
	type InviteRegisterSubmit,
} from "@/components/invite/InviteRegisterForm";
import { JoinRequestStatusCard } from "@/components/invite/JoinRequestStatusCard";
import { domainStore, useDomainStore } from "@/stores/domainStore";
import { useUserStore } from "@/stores/userStore";
import { localizedApiMessage } from "@/utils/errorMessages";

export const Route = createFileRoute("/invite/d/$code")({
	component: InviteLandingPage,
	head: () => ({ meta: [{ title: "加入域 · GOSpeak" }] }),
});

type Phase = "loading" | "error" | "ready";

function formatSubmittedAt(raw: string): string {
	const date = new Date(raw);
	if (Number.isNaN(date.getTime())) return "";
	return date.toLocaleString("zh-CN", { hour12: false });
}

export function InviteLandingPage() {
	const { code } = Route.useParams();
	const navigate = useNavigate();

	const [phase, setPhase] = useState<Phase>("loading");
	const [preview, setPreview] = useState<InvitePreview | null>(null);
	const [error, setError] = useState("");
	const [submitting, setSubmitting] = useState(false);
	const [cancelling, setCancelling] = useState(false);
	const [requestStatus, setRequestStatus] = useState<
		"" | "pending" | "rejected"
	>("");
	const [requestedAt, setRequestedAt] = useState("");
	const [codeSending, setCodeSending] = useState(false);
	const [codeCountdown, setCodeCountdown] = useState(0);

	const user = useUserStore((s) => s.user);
	const myDomainUUIDs = useDomainStore((s) => s.myDomainUUIDs);
	const joined = !!preview && myDomainUUIDs.includes(preview.domain.uuid);

	useEffect(() => {
		let cancelled = false;
		void (async () => {
			try {
				const data = await previewInvite(code);
				if (cancelled) return;
				setPreview(data);
				setPhase("ready");
			} catch {
				if (cancelled) return;
				setError("邀请链接无效、已过期或已停用");
				setPhase("error");
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [code]);

	useEffect(() => {
		if (!preview || !user) return;
		let cancelled = false;
		void (async () => {
			try {
				const view = await getJoinRequest(preview.domain.uuid);
				if (cancelled) return;
				// approved 不是落地页的展示态：审核通过后用户已是成员，
				// 由 joined（myDomainUUIDs）驱动「进入域」，这里归零即可。
				setRequestStatus(
					view.status === "pending" || view.status === "rejected"
						? view.status
						: "",
				);
				setRequestedAt(
					view.request ? formatSubmittedAt(view.request.created_at) : "",
				);
			} catch {
				// 申请状态是附加信息：拿不到不影响主流程（加入/进入域仍可用）
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [preview, user]);

	function enterDomain(uuid: string) {
		domainStore.setCurrentDomain(uuid);
		void domainStore.loadMembers(uuid);
		navigate({ to: "/domain/$domainUUID", params: { domainUUID: uuid } });
	}

	// 加入成功后先刷新「我的域」列表再跳转：domainStore.addDomain 需要完整 Domain，
	// 而 /invite/join 只回摘要字段，直接塞会缺 id/owner_uuid 等必填项。
	async function applyResult(result: InviteJoinResult) {
		if (result.status === "joined") {
			await domainStore.loadMyDomains().catch(() => {});
			toast.success(`已加入「${result.domain.name}」`);
			enterDomain(result.domain.uuid);
			return;
		}
		setRequestStatus("pending");
		setRequestedAt(formatSubmittedAt(new Date().toISOString()));
	}

	async function handleJoin() {
		if (!preview || submitting) return;
		if (joined) {
			enterDomain(preview.domain.uuid);
			return;
		}
		setSubmitting(true);
		setError("");
		try {
			await applyResult(await joinByCode(code));
		} catch (e) {
			setError(localizedApiMessage(e, "加入失败，请稍后重试"));
		} finally {
			setSubmitting(false);
		}
	}

	async function handleRegister(values: InviteRegisterSubmit) {
		if (!preview || submitting) return;
		setSubmitting(true);
		setError("");
		try {
			const result = await registerViaInvite({
				code,
				username: values.username,
				password: values.password,
				nickname: values.nickname,
				email: values.email,
				email_code: values.emailCode,
			});
			if (result.user) {
				await useUserStore.getState().login(result.user, result.expires_in);
			}
			await applyResult(result);
		} catch (e) {
			setError(localizedApiMessage(e, "注册失败，请稍后重试"));
		} finally {
			setSubmitting(false);
		}
	}

	async function handleSendEmailCode(email: string) {
		if (!email || codeSending || codeCountdown > 0) return;
		setCodeSending(true);
		try {
			const res = await sendEmailCode(
				{ email, scene: "register" },
				{ skipErrorToast: true },
			);
			toast.success("验证码已发送，请查收邮箱");
			setCodeCountdown(res.expires_in || 60);
		} catch (e) {
			setError(localizedApiMessage(e, "验证码发送失败"));
		} finally {
			setCodeSending(false);
		}
	}

	async function handleCancel() {
		if (!preview || cancelling) return;
		setCancelling(true);
		try {
			await cancelJoinRequest(preview.domain.uuid);
			setRequestStatus("");
			setRequestedAt("");
			toast.success("申请已撤销");
		} catch (e) {
			setError(localizedApiMessage(e, "撤销失败，请稍后重试"));
		} finally {
			setCancelling(false);
		}
	}

	async function handleReapply() {
		await handleJoin();
	}

	return (
		<div className="bg-muted flex min-h-full w-full items-center justify-center px-4">
			<div className="bg-card border-border w-full max-w-md rounded-lg border p-6 shadow-xl">
				<div className="mb-5 flex items-center gap-2">
					<span className="text-primary text-lg font-semibold">GOSpeak</span>
					<span className="text-faint-foreground text-xs">邀请</span>
				</div>

				{phase === "loading" && (
					<LoadingRegion label="正在打开邀请">
						<div className="flex flex-col gap-3 py-2">
							<Skeleton className="h-5 w-32" />
							<Skeleton className="h-4 w-full" />
							<Skeleton className="h-4 w-2/3" />
							<Skeleton className="mt-2 h-10 w-full rounded-sm" />
						</div>
					</LoadingRegion>
				)}

				{phase === "error" && (
					<div className="flex flex-col gap-4">
						<p className="text-destructive text-sm">{error}</p>
						<p className="text-muted-foreground text-sm">
							请向对方确认邀请链接是否已过期或停用。
						</p>
						<div className="flex gap-2">
							<Button
								variant="outline"
								onClick={() => {
									setPhase("loading");
									setError("");
									void (async () => {
										try {
											setPreview(await previewInvite(code));
											setPhase("ready");
										} catch {
											setError("邀请链接无效、已过期或已停用");
											setPhase("error");
										}
									})();
								}}
							>
								重试
							</Button>
							<Link
								to="/discover"
								className="text-muted-foreground hover:text-foreground inline-flex items-center px-2 text-sm"
							>
								先去发现域
							</Link>
						</div>
					</div>
				)}

				{phase === "ready" && preview && (
					<div className="flex flex-col gap-5">
						<div className="flex items-center gap-3">
							{preview.domain.icon_url ? (
								<img
									src={preview.domain.icon_url}
									alt={preview.domain.name}
									className="size-14 shrink-0 rounded-xl object-cover"
								/>
							) : (
								<div className="bg-primary text-primary-foreground flex size-14 shrink-0 items-center justify-center rounded-xl text-xl font-bold">
									{(
										Array.from(preview.domain.name.trim())[0] ?? "?"
									).toUpperCase()}
								</div>
							)}
							<div className="min-w-0">
								<h1 className="truncate text-lg font-semibold">
									{preview.domain.name}
								</h1>
								<p className="text-muted-foreground text-xs">
									{joined ? "你已在这个域里" : "邀请你加入这个语音域"}
								</p>
							</div>
						</div>

						{preview.domain.description && (
							<p className="text-muted-foreground text-sm leading-6">
								{preview.domain.description}
							</p>
						)}

						{preview.requires_approval &&
							!joined &&
							requestStatus !== "pending" && (
								<p className="text-warning text-xs leading-relaxed">
									该域开启加入审核，加入后需等待管理员通过。
								</p>
							)}

						{requestStatus === "pending" && (
							<JoinRequestStatusCard
								domainName={preview.domain.name}
								iconUrl={preview.domain.icon_url}
								status="pending"
								submittedAt={requestedAt}
								cancelling={cancelling}
								onCancel={() => void handleCancel()}
								onReapply={() => void handleReapply()}
							/>
						)}

						{requestStatus === "rejected" && (
							<JoinRequestStatusCard
								domainName={preview.domain.name}
								iconUrl={preview.domain.icon_url}
								status="rejected"
								submittedAt={requestedAt}
								cancelling={cancelling}
								onCancel={() => void handleCancel()}
								onReapply={() => void handleReapply()}
							/>
						)}

						{requestStatus === "" && !user && (
							<>
								<InviteRegisterForm
									domainName={preview.domain.name}
									requiresApproval={preview.requires_approval}
									submitting={submitting}
									error={error}
									emailRequired={false}
									codeSending={codeSending}
									codeCountdown={codeCountdown}
									onSubmit={(values) => void handleRegister(values)}
									onSendEmailCode={(email) => void handleSendEmailCode(email)}
								/>
								<Link
									to="/login"
									className="text-muted-foreground hover:text-foreground text-center text-sm"
								>
									已有账号？登录
								</Link>
							</>
						)}

						{requestStatus === "" && user && (
							<div className="flex flex-col gap-3">
								{error && <p className="text-destructive text-sm">{error}</p>}
								<Button
									className="flex-1"
									disabled={submitting}
									onClick={() => void handleJoin()}
								>
									{submitting ? (
										<Loader2 className="size-4 animate-spin" />
									) : joined ? (
										"进入域"
									) : preview.requires_approval ? (
										"申请加入"
									) : (
										"加入"
									)}
								</Button>
								<Link
									to="/discover"
									className="text-muted-foreground hover:text-foreground text-center text-sm"
								>
									取消
								</Link>
							</div>
						)}
					</div>
				)}
			</div>
		</div>
	);
}
