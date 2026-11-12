import { useDeepLinkNavigation } from "@/hooks/useDeepLinkNavigation";
import { usePendingInvite } from "@/hooks/usePendingInvite";

/**
 * 挂载桌面端的邀请入口（深链 + 切服务器后的待处理邀请码）。
 *
 * 渲染 null，挂在根路由里：两个 hook 在 Web 构建中由条件编译直接返回，
 * 不会触碰桌面桥接；桌面构建（或 dev server 被宿主打开）才真正生效。
 */
export default function DesktopInviteBridge() {
	useDeepLinkNavigation();
	usePendingInvite();
	return null;
}
