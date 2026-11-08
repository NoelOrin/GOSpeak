import { getInitialDeepLink, onDeepLink } from "@gospeak/desktop";
import { useRouter } from "@tanstack/react-router";
import { useEffect } from "react";

/**
 * 把宿主收到的深链（gospeak://invite/d/CODE，宿主已归一化为 /invite/d/CODE）
 * 变成应用内跳转。
 *
 * 冷启动时链接早于页面订阅，所以先订阅再取 Initial —— 顺序反了会丢掉启动链接。
 * 只处理应用内相对路径，避免宿主传来的任意 URL 把页面带走。
 */
export function useDeepLinkNavigation() {
	const router = useRouter();

	useEffect(() => {
		if (!__GOSPEAK_DESKTOP__) return;

		const go = (target: unknown) => {
			if (typeof target !== "string" || !target.startsWith("/")) return;
			const current = `${window.location.pathname}${window.location.search}`;
			if (current === target) return;
			void router.navigate({ to: target });
		};

		const unsubscribe = onDeepLink(go);
		void getInitialDeepLink()
			.then((initial) => {
				if (initial) go(initial);
			})
			.catch(() => {});

		return unsubscribe;
	}, [router]);
}
