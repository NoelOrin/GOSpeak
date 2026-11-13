import { Button } from "@gospeak/components";
import type { UpdateInfo } from "@gospeak/desktop";
import { createDesktop } from "@gospeak/desktop";
import { useEffect, useState } from "react";
import { Section } from "../shared";

type UpdateStatus =
	| "idle"
	| "checking"
	| "latest"
	| "available"
	| "downloading"
	| "restarting"
	| "unsupported"
	| "error";

/**
 * 桌面构建的「软件更新」区块：MyGo 内建的签名更新（校验公钥后原地替换并
 * 重启）。版本号本身由上方的 VersionSection 展示，这里只负责检查与安装。
 *
 * 与 serverUpdate.tsx 二选一（about/index.tsx 按 `__GOSPEAK_DESKTOP__` 选择），
 * Web 构建中本模块只在死分支里，连同 @gospeak/desktop 的更新能力一起被摇掉。
 */
export function DesktopUpdateSection() {
	const desktop = createDesktop();

	const [status, setStatus] = useState<UpdateStatus>("idle");
	const [update, setUpdate] = useState<UpdateInfo | null>(null);
	const [progress, setProgress] = useState(0);
	const [error, setError] = useState("");
	const [releaseError, setReleaseError] = useState("");

	// 安装期间宿主按已下载/总量广播 0..1。
	// biome-ignore lint/correctness/useExhaustiveDependencies: 只在宿主可用性变化时重订阅，desktop 门面对象稳定
	useEffect(() => {
		if (!desktop.isDesktop) return;
		return desktop.updater.onProgress(setProgress);
	}, [desktop.isDesktop]);

	const check = async () => {
		setStatus("checking");
		setError("");
		setUpdate(null);
		try {
			const info = await desktop.updater.check();
			if (!info.supported) {
				setStatus("unsupported");
			} else if (info.available) {
				setUpdate(info);
				setStatus("available");
			} else {
				setStatus("latest");
			}
		} catch (e) {
			setStatus("error");
			setError(e instanceof Error ? e.message : String(e));
		}
	};

	const install = async () => {
		setStatus("downloading");
		setProgress(0);
		try {
			await desktop.updater.install();
			// 安装成功后宿主重启进新版本，页面随之关闭；走到这里说明重启未发生。
			setStatus("restarting");
		} catch (e) {
			setStatus("error");
			setError(e instanceof Error ? e.message : String(e));
		}
	};

	// 打开发布页同样可能失败（宿主未绑定 / 系统浏览器拒绝），不处理就是死按钮。
	const openRelease = async () => {
		setReleaseError("");
		try {
			await desktop.updater.openReleasePage();
		} catch (e) {
			setReleaseError(e instanceof Error ? e.message : String(e));
		}
	};

	// 纯浏览器里没有桌面宿主，本区块不渲染（版本区块仍会展示 Web 版本）。
	if (!desktop.isDesktop) return null;

	return (
		<Section
			title="软件更新"
			desc="检查 GitHub Releases 上的新版本；安装会校验签名并替换当前应用后重启。"
		>
			<div className="flex flex-col gap-2">
				<div className="flex items-center justify-between gap-4">
					<div className="min-w-0">
						<div className="text-foreground text-sm font-medium">
							{status === "checking"
								? "正在检查更新…"
								: status === "latest"
									? "已是最新版本"
									: status === "available" && update?.version
										? `发现新版本 ${update.version}`
										: status === "unsupported"
											? "当前构建不支持自动更新"
											: "未检查"}
						</div>
						{status === "unsupported" && (
							<div className="text-faint-foreground mt-0.5 text-xs">
								dev 构建或旧版本安装包无法自更新，请前往发布页手动下载。
							</div>
						)}
						{status === "downloading" && (
							<div className="text-faint-foreground mt-0.5 text-xs">
								正在下载更新… {Math.round(progress * 100)}%
							</div>
						)}
						{status === "restarting" && (
							<div className="mt-0.5 text-xs">更新完成，正在重启…</div>
						)}
						{status === "error" && (
							<div className="text-destructive mt-0.5 text-xs" title={error}>
								检查/安装失败：{error}
							</div>
						)}
					</div>
					<div className="flex shrink-0 items-center gap-2">
						{status === "available" ? (
							<Button size="sm" onClick={() => void install()}>
								下载并安装
							</Button>
						) : status === "downloading" ? null : (
							<Button
								size="sm"
								variant="outline"
								disabled={status === "checking"}
								onClick={() => void check()}
							>
								检查更新
							</Button>
						)}
						{status === "unsupported" && (
							<Button size="sm" onClick={() => void openRelease()}>
								前往发布页
							</Button>
						)}
					</div>
				</div>

				{releaseError && (
					<div className="text-destructive text-xs" title={releaseError}>
						无法打开发布页：{releaseError}
					</div>
				)}

				{status === "downloading" && (
					<div className="bg-border h-1.5 w-full overflow-hidden rounded-sm">
						<div
							className="bg-primary h-full transition-[width]"
							style={{ width: `${Math.round(progress * 100)}%` }}
						/>
					</div>
				)}

				{status === "available" && update?.notes ? (
					<div className="border-border bg-muted/40 rounded-sm border px-3 py-2">
						<div className="text-faint-foreground max-h-32 overflow-y-auto whitespace-pre-wrap text-xs">
							{update.notes}
						</div>
					</div>
				) : null}
			</div>
		</Section>
	);
}
