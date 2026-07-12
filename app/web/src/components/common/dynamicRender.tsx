import type { JSX } from "solid-js";
import { useLocation } from "@tanstack/solid-router";
import { createMemo } from "solid-js";
import { Dynamic } from "solid-js/web";
import HomePage from "@/components/home/homePage";
import RoomList from "@/components/room/roomList";
import ManageNav from "@/components/manage/manageNav";

// 路径前缀 -> 左侧面板组件（长前缀优先匹配）
const PREFIX_MAP: [string, (...args: any[]) => JSX.Element][] = [
	["/manage", ManageNav],
	["/channel", RoomList],
	["/", HomePage],
];

const DynamicRender = () => {
	const location = useLocation();

	const CurrentComponent = createMemo(() => {
		const path = location().pathname;
		for (const [prefix, component] of PREFIX_MAP) {
			if (path === prefix || path.startsWith(`${prefix}/`)) {
				return component;
			}
		}
		return HomePage;
	});

	return <Dynamic component={CurrentComponent()} />;
};

export default DynamicRender;
