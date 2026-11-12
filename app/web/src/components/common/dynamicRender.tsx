import { useLocation } from "@tanstack/react-router";
import type { ComponentType } from "react";
import { ErrorBoundary } from "@/components/common/errorBoundary";
import ConversationList from "@/components/chat/conversationList";
import HomePage from "@/components/home/homePage";
import ManageNav from "@/components/manage/manageNav";
import RoomList from "@/components/room/roomList";

// 路径前缀 -> 左侧面板组件（长前缀优先匹配）
const PREFIX_MAP: [string, ComponentType][] = [
	["/manage", ManageNav],
	["/domain", RoomList],
	["/chat", ConversationList],
	["/discover", HomePage],
];

export default function DynamicRender() {
	const location = useLocation();

	let Current: ComponentType = HomePage;
	const path = location.pathname;
	for (const [prefix, component] of PREFIX_MAP) {
		if (path === prefix || path.startsWith(`${prefix}/`)) {
			Current = component;
			break;
		}
	}

	// 组件级边界：左侧面板按路径切换，任一面板崩溃只影响这一栏，
	// 且切到别的路径时自动恢复。
	return (
		<ErrorBoundary
			name="side-panel"
			resetKeys={[path]}
			title="侧边面板加载失败"
		>
			<Current />
		</ErrorBoundary>
	);
}
