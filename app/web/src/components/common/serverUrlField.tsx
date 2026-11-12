import { Button, Input } from "@gospeak/components";
import { createServerConfig, inDesktop } from "@gospeak/desktop";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { parseInviteLink, type ParsedInviteLink } from "@/utils/domainInvite";
import { clearPendingInvite, setPendingInvite } from "@/utils/pendingInvite";
import { setPublicBaseUrl } from "@/utils/publicBaseUrl";

/**
 * 桌面端「服务器地址」输入框。
 *
 * 保存后宿主会持久化该地址、把本地回环代理的转发目标切到新服务器，并自动
 * 刷新页面 —— 页面与代理始终同源，不存在跨域。仅在桌面端显示
 * （父组件用 inDesktop() 控制，平台隔离）。
 *
 * 也可以直接喂一个邀请链接：挂载时读一次剪贴板，粘贴/回车时解析输入框内容。
 * 链接里带服务器（https://host/invite/d/CODE）就切过去再进邀请页；只有邀请码
 * （gospeak://、回环地址、裸码）则不改服务器，直接跳。
 */
export default function ServerUrlField({ className }: { className?: string }) {
	const navigate = useNavigate();
	// 先用本地镜像（localStorage）填充，首帧就能显示已保存的地址；
	// 随后 effect 向宿主要权威值校正（桌面端宿主文件才是权威）。
	const readMirror = () => createServerConfig().getBaseUrlSync();
	const [value, setValue] = useState(readMirror);
	const [saved, setSaved] = useState(readMirror);
	const [status, setStatus] = useState<"idle" | "saving" | "error">("idle");
	const [invite, setInvite] = useState<ParsedInviteLink | null>(null);

	useEffect(() => {
		void createServerConfig()
			.getBaseUrl()
			.then((v) => {
				setValue(v);
				setSaved(v);
			})
			.catch(() => {});
		// 与「发现域」页同款：进入页面读一次剪贴板，识别到邀请链接就提示，
		// 但不自动切服务器（切服务器会清会话并重载，必须用户确认）。
		void (async () => {
			try {
				if (!navigator.clipboard?.readText) return;
				const text = await navigator.clipboard.readText();
				const parsed = parseInviteLink(text);
				if (!parsed) return;
				setInvite(parsed);
				if (parsed.server) setValue(parsed.server);
			} catch {
				// 未授权读剪贴板：静默跳过，用户可手动粘贴。
			}
		})();
	}, []);

	const enterInvite = async (parsed: ParsedInviteLink) => {
		setStatus("saving");
		setInvite(null);
		const current = saved.trim().replace(/\/+$/, "");
		const target = parsed.server?.trim().replace(/\/+$/, "");
		if (!target || target === current) {
			navigate({ to: "/invite/d/$code", params: { code: parsed.code } });
			return;
		}
		// 切换服务器会重载页面：邀请码先落到 localStorage，重载后由根路由取走。
		setPendingInvite(parsed.code);
		try {
			setPublicBaseUrl(target);
			await createServerConfig().setBaseUrl(target);
			// 宿主通常会重载窗口（此时页面已卸载，下面的跳转不会执行，pending 由
			// usePendingInvite 消费）；没重载时直接跳，并稍后清掉 pending 免得下次
			// 启动又跳一遍。
			navigate({ to: "/invite/d/$code", params: { code: parsed.code } });
			setTimeout(() => clearPendingInvite(), 2000);
		} catch {
			clearPendingInvite();
			setStatus("error");
		}
	};

	const persist = (viaBlur = false) => {
		const next = value.trim();
		if (!next || status === "saving") return;
		// 有识别到的邀请时，焦点离开输入框（例如去点「连接并加入」）不能顺手
		// 保存地址 —— 那样会先切服务器并重载，把邀请码弄丢。交给用户显式确认。
		if (invite) {
			if (viaBlur) return;
			void enterInvite(invite);
			return;
		}
		const parsed = parseInviteLink(next);
		if (parsed) {
			void enterInvite(parsed);
			return;
		}
		setStatus("saving");
		// 成功时宿主会刷新窗口（当前页面随之卸载），无需恢复状态；
		// 失败则回到可编辑态让用户重试。
		void createServerConfig()
			.setBaseUrl(next)
			.then(() => setPublicBaseUrl(next))
			.catch(() => setStatus("error"));
	};

	return (
		<div className={className}>
			<label className="block">
				<span className="mb-1 block text-sm">服务器地址</span>
				<Input
					type="text"
					value={value}
					placeholder="https://your-server.com"
					autoComplete="off"
					spellCheck={false}
					onChange={(e) => {
						setValue(e.target.value);
						setInvite(null);
					}}
					onBlur={() => persist(true)}
					onKeyDown={(e) => {
						if (e.key === "Enter") {
							e.preventDefault();
							persist();
						}
					}}
				/>
			</label>

			{invite && (
				<div className="border-primary/40 bg-primary/5 mt-2 flex flex-col gap-2 rounded-sm border px-3 py-2">
					<p className="text-sm">
						检测到邀请链接
						<code className="text-primary mx-1 font-semibold">
							{invite.code}
						</code>
						{invite.server ? `（服务器 ${invite.server}）` : ""}
					</p>
					<div className="flex gap-2">
						<Button size="sm" onClick={() => void enterInvite(invite)}>
							连接并加入
						</Button>
						<Button
							size="sm"
							variant="outline"
							onClick={() => {
								setInvite(null);
								setValue(saved);
							}}
						>
							忽略
						</Button>
					</div>
				</div>
			)}

			{status === "saving" ? (
				<p className="text-muted-foreground mt-1 text-xs">
					正在连接服务器并刷新…
				</p>
			) : status === "error" ? (
				<p className="text-destructive mt-1 text-xs">保存失败，请重试。</p>
			) : (
				<p className="text-faint-foreground mt-1 text-xs">
					{inDesktop()
						? "登录前填写要连接的 GOSpeak 服务器，也可以直接粘贴邀请链接；保存在本机，保存后自动连接并刷新。"
						: "登录前填写要连接的 GOSpeak 服务器。"}
				</p>
			)}
		</div>
	);
}
