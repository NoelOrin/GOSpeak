import { Button, Input, toast } from "@gospeak/components";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { guestJoin, type GuestDomain } from "@/api/guest";
import { listPublicDomains } from "@/api/domain";
import { useUserStore } from "@/stores/userStore";
import { setGuestCaps } from "@/stores/guestStore";
import { toastActionError } from "@/utils/toast";

export const Route = createFileRoute("/guest")({
	validateSearch: (search: Record<string, unknown>) => ({
		code:
			typeof search.code === "string" && search.code ? search.code : undefined,
		domain:
			typeof search.domain === "string" && search.domain
				? search.domain
				: undefined,
	}),
	component: GuestPage,
});

function applyGuestDomainCaps(domain?: GuestDomain | null) {
	if (!domain) return;
	setGuestCaps({
		listen: domain.guest_can_listen ?? true,
		speak: domain.guest_can_speak ?? true,
		message: domain.guest_can_message ?? false,
	});
}

function GuestPage() {
	const navigate = useNavigate();
	const search = Route.useSearch();
	const inviteCode = search.code ?? "";
	const domainUUID = search.domain ?? "";

	const user = useUserStore((s) => s.user);
	const isLoggedIn = !!user;

	const [publicDomains, setPublicDomains] = useState<GuestDomain[]>([]);
	const [nickname, setNickname] = useState("");
	const [selected, setSelected] = useState<GuestDomain | null>(null);
	const [submitting, setSubmitting] = useState(false);

	useEffect(() => {
		if (inviteCode || domainUUID) return;
		void (async () => {
			try {
				const page = await listPublicDomains(1, 50);
				setPublicDomains((page.domains ?? []).filter((d) => d.allow_guest));
			} catch {
				setPublicDomains([]);
			}
		})();
	}, [inviteCode, domainUUID]);

	async function enter() {
		// 已有访客身份的"继续"场景没有输入框，回退到会话里的昵称
		const name = (nickname.trim() || user?.display_name || "").trim();
		if (!name) {
			toast.error("请输入昵称");
			return;
		}
		setSubmitting(true);
		try {
			// 该接口的失败文案要按业务码定制中文提示，交给本页统一弹，避免拦截器再弹一条英文
			const data = await guestJoin(
				{
					nickname: name,
					invite_code: inviteCode || undefined,
					domain_uuid: domainUUID || (selected?.uuid as string | undefined),
				},
				{ skipErrorToast: true },
			);
			await useUserStore.getState().login({
				id: data.user.id,
				uuid: data.user.uuid,
				name: data.user.name,
				display_name: data.user.display_name,
				avatar: data.user.avatar,
				role: data.user.role,
				is_guest: true,
			});
			applyGuestDomainCaps(data.domain);
			toast.success(`欢迎，${data.user.display_name}`);
			// 访客的意图是"进这个域"，不是"进首页"：直接落到目标域，
			// 否则会被 / 的默认重定向甩到发现页，丢掉邀请上下文。
			if (data.domain?.uuid) {
				navigate({
					to: "/domain/$domainUUID",
					params: { domainUUID: data.domain.uuid },
				});
				return;
			}
			navigate({ to: "/discover" });
		} catch (e) {
			const code = (
				e as { response?: { data?: { code?: number; msg?: string } } }
			)?.response?.data;
			if (code?.code === 1013) {
				toast.error("该域未开放访客进入");
			} else if (code?.code === 1017) {
				toast.error(code.msg || "访客人数已达上限或操作过于频繁");
			} else {
				toastActionError(e, "进入失败，请稍后重试");
			}
		} finally {
			setSubmitting(false);
		}
	}

	return (
		<div className="bg-muted flex h-full w-full items-center justify-center">
			<div className="bg-card mx-4 w-full max-w-md rounded-lg shadow-xl">
				<div className="flex flex-col gap-4 p-6">
					<h1 className="text-xl font-semibold">以访客身份进入</h1>
					<p className="text-faint-foreground -mt-2 text-xs leading-relaxed">
						域是一个独立的语音空间，有自己的语音频道、文字频道和成员。访客无需注册即可加入，可用权限由该域管理员设置。
					</p>

					{isLoggedIn && user?.is_guest && (
						<>
							<Button disabled={submitting} onClick={() => void enter()}>
								以 {user?.display_name} 继续
							</Button>
							<div className="text-faint-foreground flex items-center gap-2 text-xs">
								<span className="bg-border h-px flex-1" />
								或换一个身份
								<span className="bg-border h-px flex-1" />
							</div>
						</>
					)}

					<label className="block">
						<span className="mb-1 block">昵称（最多 24 字符）</span>
						<Input
							className="w-full"
							maxLength={24}
							value={nickname}
							placeholder="输入昵称"
							onChange={(e) => setNickname(e.target.value)}
							onKeyDown={(e) => {
								if (e.key === "Enter") void enter();
							}}
						/>
					</label>

					{!inviteCode &&
						(domainUUID ? (
							<div className="text-muted-foreground text-sm">
								将通过公开域加入：{domainUUID}
							</div>
						) : (
							publicDomains.length > 0 && (
								<div className="flex flex-wrap gap-2">
									{publicDomains.map((d) => (
										<Button
											key={d.uuid}
											size="sm"
											variant={
												selected?.uuid === d.uuid ? "default" : "outline"
											}
											onClick={() => setSelected(d)}
										>
											{d.name}
										</Button>
									))}
								</div>
							)
						))}
					{inviteCode && (
						<div className="text-muted-foreground text-sm">
							使用邀请链接加入
						</div>
					)}

					<Button
						className="mt-2"
						disabled={submitting}
						onClick={() => void enter()}
					>
						{submitting ? (
							<Loader2 className="size-4 animate-spin" />
						) : (
							<span>进入</span>
						)}
					</Button>

					{/* 拿着邀请链接的往往本来就是成员：给一条明确的登录回跳路径，
					    否则只能自己猜去点浏览器地址栏 */}
					{inviteCode && (
						<button
							type="button"
							className="text-muted-foreground hover:text-foreground text-xs underline underline-offset-4"
							onClick={() => {
								sessionStorage.setItem(
									"gospeak_redirect",
									`/invite/d/${inviteCode}`,
								);
								navigate({ to: "/login" });
							}}
						>
							我是该域成员，用账号登录
						</button>
					)}
				</div>
			</div>
		</div>
	);
}
