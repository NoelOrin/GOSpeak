import { createFileRoute, redirect } from "@tanstack/react-router";
import { domainStore } from "@/stores/domainStore";
import { resolveFirstScreen } from "@/utils/firstScreen";

/**
 * 首屏分流：
 * - 有已加入的域 → 直接回到上次所在的域（校验仍在成员列表内）
 * - 还没有域     → 去发现域页引导「创建 / 加入」
 *
 * 以前无条件跳 /discover，老用户每次刷新都要在侧栏再点一次自己的域。
 */
export const Route = createFileRoute("/(app)/")({
	beforeLoad: async () => {
		try {
			await domainStore.loadMyDomains();
		} catch {
			throw redirect({ to: "/discover" });
		}

		const { myDomainUUIDs, currentDomainUUID } = domainStore.state;
		const target = resolveFirstScreen(myDomainUUIDs, currentDomainUUID);

		if (!target) throw redirect({ to: "/discover" });
		throw redirect({
			to: "/domain/$domainUUID",
			params: { domainUUID: target },
		});
	},
	staticData: {
		title: "工作台",
	},
});
