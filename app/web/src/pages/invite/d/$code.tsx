import { Button, toast } from "@gospeak/components";
import {
	createFileRoute,
	Link,
	redirect,
	useNavigate,
} from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import type { Domain } from "@/api/domain";
import { joinDomain, previewDomainInvite } from "@/api/domain";
import { domainStore, useDomainStore } from "@/stores/domainStore";
import { useUserStore } from "@/stores/userStore";
import { toastActionError } from "@/utils/toast";

/**
 * 邀请链接落地页：/invite/d/<CODE>
 *
 * 这是对外分享的唯一入口（domainInviteUrl 生成的就是这个地址），必须能处理三种人：
 *   1. 未登录的陌生人  → 送去访客页，带着邀请码直接进目标域
 *   2. 已登录的域成员  → 直接进域
 *   3. 已登录但不在域里 → 预览 + 一键加入
 * 访客（is_guest）不能成为域成员，同样送回访客页重新以该码进入。
 */
export const Route = createFileRoute("/invite/d/$code")({
	beforeLoad: async () => {
		const ok = await useUserStore.getState().ensureSession();
		const user = useUserStore.getState().user;
		if (!ok || !user) {
			throw redirect({ to: "/login" });
		}
	},
	component: InviteLandingPage,
	head: () => ({ meta: [{ title: "加入域 · GOSpeak" }] }),
});

type Phase = "loading" | "error" | "ready";

function InviteLandingPage() {
	const { code } = Route.useParams();
	const navigate = useNavigate();

	const [phase, setPhase] = useState<Phase>("loading");
	const [domain, setDomain] = useState<Domain | null>(null);
	const [error, setError] = useState("");
	const [joining, setJoining] = useState(false);

	const myDomainUUIDs = useDomainStore((s) => s.myDomainUUIDs);
	const joined = !!domain && myDomainUUIDs.includes(domain.uuid);

	useEffect(() => {
		let cancelled = false;
		void (async () => {
			try {
				const d = await previewDomainInvite(code);
				if (cancelled) return;
				setDomain(d);
				setPhase("ready");
			} catch (e) {
				if (cancelled) return;
				// 邀请码失效是高频可预期错误：就地展示，不交给拦截器弹英文 toast
				setError("邀请链接已失效或邀请码不正确");
				setPhase("error");
				toastActionError(e, "");
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [code]);

	function enterDomain(uuid: string) {
		domainStore.setCurrentDomain(uuid);
		void domainStore.loadMembers(uuid);
		navigate({ to: "/domain/$domainUUID", params: { domainUUID: uuid } });
	}

	async function handleJoin() {
		if (!domain || joining) return;
		if (joined) {
			enterDomain(domain.uuid);
			return;
		}
		setJoining(true);
		try {
			const d = await joinDomain(code);
			domainStore.addDomain(d);
			toast.success(`已加入「${d.name}」`);
			enterDomain(d.uuid);
		} catch (e) {
			setError(toastActionError(e, "加入失败，请稍后重试"));
		} finally {
			setJoining(false);
		}
	}

	return (
		<div className="bg-muted flex min-h-full w-full items-center justify-center px-4">
			<div className="bg-card border-border w-full max-w-md rounded-lg border p-6 shadow-xl">
				<div className="mb-5 flex items-center gap-2">
					<span className="text-primary text-lg font-semibold">GOSpeak</span>
					<span className="text-faint-foreground text-xs">邀请</span>
				</div>

				{phase === "loading" && (
					<div className="flex flex-col items-center gap-3 py-10">
						<Loader2 className="text-muted-foreground size-6 animate-spin" />
						<p className="text-muted-foreground text-sm">正在打开邀请…</p>
					</div>
				)}

				{phase === "error" && (
					<div className="flex flex-col gap-4">
						<p className="text-destructive text-sm">{error}</p>
						<p className="text-muted-foreground text-sm">
							请向对方确认邀请链接是否已重置。
						</p>
						<div className="flex gap-2">
							<Button
								variant="outline"
								onClick={() => {
									setPhase("loading");
									setError("");
									void (async () => {
										try {
											const d = await previewDomainInvite(code);
											setDomain(d);
											setPhase("ready");
										} catch {
											setError("邀请链接已失效或邀请码不正确");
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

				{phase === "ready" && domain && (
					<div className="flex flex-col gap-5">
						<div className="flex items-center gap-3">
							{domain.icon_url ? (
								<img
									src={domain.icon_url}
									alt={domain.name}
									className="size-14 shrink-0 rounded-xl object-cover"
								/>
							) : (
								<div className="bg-primary text-primary-foreground flex size-14 shrink-0 items-center justify-center rounded-xl text-xl font-bold">
									{(Array.from(domain.name.trim())[0] ?? "?").toUpperCase()}
								</div>
							)}
							<div className="min-w-0">
								<h1 className="truncate text-lg font-semibold">
									{domain.name}
								</h1>
								<p className="text-muted-foreground text-xs">
									{joined ? "你已在这个域里" : "邀请你加入这个语音域"}
								</p>
							</div>
						</div>

						{domain.description && (
							<p className="text-muted-foreground text-sm leading-6">
								{domain.description}
							</p>
						)}

						{!joined && (
							<p className="text-faint-foreground text-xs leading-relaxed">
								域是一个独立的语音空间：加入后可以进入它的语音频道、文字频道，和域内成员实时交流。你随时可以退出。
							</p>
						)}

						{error && <p className="text-destructive text-sm">{error}</p>}

						<div className="flex gap-2">
							<Button
								className="flex-1"
								disabled={joining}
								onClick={() => void handleJoin()}
							>
								{joining ? (
									<Loader2 className="size-4 animate-spin" />
								) : joined ? (
									"进入域"
								) : (
									"加入并进入"
								)}
							</Button>
							<Link
								to="/discover"
								className="border-border hover:bg-muted inline-flex h-9 items-center rounded-sm border px-3 text-sm transition-colors"
							>
								取消
							</Link>
						</div>
					</div>
				)}
			</div>
		</div>
	);
}
