import { useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import { takePendingInvite } from "@/utils/pendingInvite";

/**
 * 服务器切换后重载页面时，把切换前暂存的邀请码接上。
 *
 * 只在桌面构建生效：这个槽只有桌面端切服务器时才会写。取走后立即清除，避免
 * 用户之后每次刷新都被拉回同一个邀请页。
 */
export function usePendingInvite() {
	const navigate = useNavigate();

	useEffect(() => {
		if (!__GOSPEAK_DESKTOP__) return;
		const code = takePendingInvite();
		if (!code) return;
		void navigate({ to: "/invite/d/$code", params: { code } });
	}, [navigate]);
}
