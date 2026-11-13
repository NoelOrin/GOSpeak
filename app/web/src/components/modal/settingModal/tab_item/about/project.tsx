import { ExternalLink } from "lucide-react";
import { Section } from "../shared";

/**
 * 项目链接。
 *
 * 用普通外链而非原生「打开浏览器」能力：MyGo 宿主的默认窗口打开策略会把
 * http(s) 链接交给系统浏览器（app/desktop/windows.go 未覆盖该策略），Web
 * 构建里就是普通新标签页——两端同一份代码，不需要平台分支。
 */
const LINKS: { label: string; href: string; desc: string }[] = [
	{
		label: "项目主页",
		href: "https://github.com/NoelOrin/GOSpeak",
		desc: "源码、Issue 与开发文档",
	},
	{
		label: "发布页",
		href: "https://github.com/NoelOrin/GOSpeak/releases",
		desc: "服务端二进制与桌面端安装包",
	},
	{
		label: "演示站",
		href: "https://gospeak.noelorin.top",
		desc: "在线体验，无需自建",
	},
];

export function ProjectSection() {
	return (
		<Section
			title="项目"
			desc="GOSpeak 是自托管的开源项目，基于 LiveKit 提供音视频能力。"
		>
			<div className="divide-border/60 divide-y">
				{LINKS.map((link) => (
					<a
						key={link.href}
						href={link.href}
						target="_blank"
						rel="noreferrer"
						className="hover:bg-muted/50 group flex items-center justify-between gap-4 rounded-sm px-1 py-2 transition-colors"
					>
						<span className="min-w-0">
							<span className="text-foreground block text-sm font-medium">
								{link.label}
							</span>
							<span className="text-faint-foreground block text-xs">
								{link.desc}
							</span>
						</span>
						<ExternalLink
							className="text-faint-foreground group-hover:text-foreground shrink-0 transition-colors"
							size={14}
						/>
					</a>
				))}
			</div>
			<p className="text-faint-foreground text-xs">
				GOSpeak 以 Apache-2.0 许可证开源。
			</p>
		</Section>
	);
}
