import { createFileRoute } from "@tanstack/react-router";
import { domainStore } from "@/stores/domainStore";

/**
 * 域路由仅承担"激活当前域"的职责。
 *
 * 域名/描述/邀请/管理/离开/删除这些内容曾渲染在本页，但布局在域路由下会隐藏
 * children（桌面 display:none、移动端不渲染），那些操作等于没有入口。
 * 现在统一由 `components/domain/DomainMenu.tsx` 挂在房间列表头部，
 * 桌面与移动共用一份，本页不再输出任何 UI。
 */
export const Route = createFileRoute("/(app)/domain/$domainUUID")({
	beforeLoad: ({ params }) => {
		domainStore.activateDomain(params.domainUUID);
	},
	component: () => null,
	staticData: { title: "语音域" },
});
