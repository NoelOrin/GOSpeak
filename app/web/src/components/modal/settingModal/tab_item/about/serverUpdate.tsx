import { Button } from "@gospeak/components";
import { checkServerUpdate, type ServerUpdateInfo } from "@/api/system";
import { useUserStore } from "@/stores/userStore";
import { safeHttpUrl } from "@/utils/safeHttpUrl";
import { useState } from "react";
import { Section } from "../shared";

const RELEASES_PAGE = "https://github.com/NoelOrin/GOSpeak/releases";

type Status = "idle" | "checking" | "latest" | "available" | "error";

/**
 * Web 构建的「软件更新」区块。
 *
 * 管理员：可在线检查主体服务端 v* 发布（/system/update/check，后端带 5 分钟
 * 缓存），比对到新版本后跳发布页手动升级——Web 端没有自更新能力，不做安装。
 * 普通用户：只读，只给一个发布页入口（后端该端点本身也是 admin-only）。
 *
 * 桌面构建用的是 desktopUpdate.tsx（宿主签名自更新），两者由 about.tsx 按
 * 编译期常量二选一，本模块在桌面构建中不进包。
 */
export function ServerUpdateSection() {
	const isAdmin = useUserStore((s) => s.user?.role === "admin");

	const [status, setStatus] = useState<Status>("idle");
	const [info, setInfo] = useState<ServerUpdateInfo | null>(null);
	const [error, setError] = useState("");

	const check = async () => {
		setStatus("checking");
		setError("");
		try {
			const result = await checkServerUpdate();
			setInfo(result);
			setStatus(result.available ? "available" : "latest");
		} catch (e) {
			setStatus("error");
			setError(e instanceof Error ? e.message : String(e));
		}
	};

	const releaseUrl = safeHttpUrl(info?.url) ?? RELEASES_PAGE;

	return (
		<Section
			title="软件更新"
			desc={
				isAdmin
					? "检查 GitHub Releases 上是否有新的服务端版本。Web 端不支持自更新，发现新版本后请前往发布页下载替换。"
					: "Web 端不支持自动更新。如需升级，请由部署者前往发布页下载新版本。"
			}
		>
			<div className="flex flex-col gap-2">
				<div className="flex items-center justify-between gap-4">
					<div className="min-w-0">
						<div className="text-foreground text-sm font-medium">
							服务端
							<span className="text-faint-foreground ml-2 font-normal">
								{status === "checking"
									? "检查中…"
									: status === "latest"
										? "已是最新版本"
										: status === "available" && info?.latest
											? `发现新版本 ${info.latest}`
											: "未检查"}
							</span>
						</div>
						{status === "error" && (
							<div className="text-destructive mt-0.5 text-xs" title={error}>
								检查失败：{error || "网络异常"}
							</div>
						)}
						{status === "available" && info?.notes ? (
							<div className="text-faint-foreground mt-1 max-h-32 overflow-y-auto whitespace-pre-wrap text-xs">
								{info.notes}
							</div>
						) : null}
					</div>
					<div className="flex shrink-0 items-center gap-2">
						{isAdmin && (
							<Button
								size="sm"
								variant="outline"
								disabled={status === "checking"}
								onClick={() => void check()}
							>
								检查更新
							</Button>
						)}
						<Button size="sm" variant="outline" asChild>
							<a href={releaseUrl} target="_blank" rel="noreferrer">
								前往发布页
							</a>
						</Button>
					</div>
				</div>
			</div>
		</Section>
	);
}
