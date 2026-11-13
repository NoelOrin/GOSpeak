import { ReadonlyRow, Section } from "../shared";
import { useEffect, useState } from "react";
import { getPublicServerVersion } from "@/api/system";
import { createDesktop } from "@gospeak/desktop";
import { displayVersion } from "./display";

/** 前端版本：打包期从根 package.json 注入，tsc / vitest 环境下回退 dev。 */
function webVersion(): string {
	return typeof __GOSPEAK_APP_VERSION__ === "undefined"
		? "dev"
		: __GOSPEAK_APP_VERSION__;
}

/**
 * 版本清单：前端 / 后端 / 桌面端各一行。
 *
 * 三处版本来源互不相同，必须分别取：
 *   - 前端：构建期常量（与后端同一份根 package.json）
 *   - 后端：/system/version 公开端点（未登录也能拿到）
 *   - 桌面端：MyGo 宿主 CurrentVersion（仅宿主内可用，Web 构建不请求）
 */
export function VersionSection() {
	const [server, setServer] = useState<string>();
	const [serverCommit, setServerCommit] = useState<string>();
	const [serverFailed, setServerFailed] = useState(false);
	const [desktop, setDesktop] = useState<string>();

	const desktopDetected = createDesktop().isDesktop;

	useEffect(() => {
		let disposed = false;
		void getPublicServerVersion()
			.then((info) => {
				if (disposed) return;
				setServer(info.version);
				setServerCommit(info.commit);
			})
			.catch(() => {
				if (!disposed) setServerFailed(true);
			});
		return () => {
			disposed = true;
		};
	}, []);

	// 桌面端版本只在宿主内读取：Web 构建里 CurrentVersion 会 reject，
	// 没必要为此产生一次失败的网络/桥接调用。
	useEffect(() => {
		if (!desktopDetected) return;
		let disposed = false;
		void createDesktop()
			.updater.currentVersion()
			.then((v) => {
				if (!disposed) setDesktop(v);
			})
			.catch(() => {
				if (!disposed) setDesktop("unknown");
			});
		return () => {
			disposed = true;
		};
	}, [desktopDetected]);

	const serverVersion = serverFailed
		? "未知"
		: server === undefined
			? "…"
			: displayVersion(server);

	return (
		<Section
			title="版本"
			desc="前端与后端版本同源（仓库根 package.json），桌面端版本由安装包决定。"
		>
			<div className="divide-border/60 divide-y">
				<ReadonlyRow
					label="前端"
					desc="Web 界面构建版本"
					value={displayVersion(webVersion())}
				/>
				<ReadonlyRow
					label="后端"
					desc={
						serverCommit && serverCommit !== "unknown"
							? `commit ${serverCommit.slice(0, 7)}`
							: "服务端构建版本"
					}
					value={serverVersion}
				/>
				{desktopDetected && (
					<ReadonlyRow
						label="桌面端"
						desc="当前运行的原生客户端"
						value={desktop === undefined ? "…" : displayVersion(desktop)}
					/>
				)}
			</div>
		</Section>
	);
}
