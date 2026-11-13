import {
	Activity,
	CheckCircle2,
	Clock,
	Cpu,
	Database,
	HardDrive,
	Hash,
	MemoryStick,
	Server,
	Users,
	XCircle,
} from "lucide-react";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Button, Skeleton } from "@gospeak/components";
import { ManageHeader, ManagePage } from "@/components/manage/ManageShell";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";
import { useUserStore } from "@/stores/userStore";
import {
	checkServerUpdate,
	getServerVersion,
	type ServerUpdateInfo,
} from "@/api/system";

interface HealthSnapshot {
	timestamp: string;
	uptime: string;
	num_goroutine: number;
	num_cpu: number;
	alloc_mb: number;
	total_alloc_mb: number;
	sys_mb: number;
	num_gc: number;
	heap_objects: number;
	gc_pause_ms: number;
	gc_cpu_fraction: number;
	pid: number;
	cpu_percent: number;
	disk_used_mb: number;
	disk_total_mb: number;
	disk_percent: number;
	hub_room_count: number;
	hub_participant_count: number;
	hub_online_user_count: number;
	db_connected: boolean;
	db_in_use: number;
	db_idle: number;
	db_max_open: number;
	db_wait_count: number;
	db_wait_duration_ms: number;
}

export const Route = createFileRoute("/(app)/manage/monitor/")({
	beforeLoad: () => {
		if (!hasPermission("role:manage")) {
			denyAccess();
		}
	},
	component: MonitorPage,
	staticData: {
		title: "服务监控",
	},
});

function StatusBadge({ ok, label }: { ok: boolean; label: string }) {
	return (
		<div className="flex items-center gap-1.5">
			{ok ? (
				<CheckCircle2 size={14} className="text-success" />
			) : (
				<XCircle size={14} className="text-destructive" />
			)}
			<span className="text-xs">{label}</span>
		</div>
	);
}

function MetricCard({
	icon,
	label,
	value,
	dim,
	badge,
}: {
	icon: ReactNode;
	label: string;
	value: string | null;
	dim?: boolean;
	badge?: ReactNode;
}) {
	return (
		<div className="border-border/80 bg-card rounded-lg border shadow-sm">
			<div className="p-3">
				<div className="text-muted-foreground mb-1 flex items-center gap-1.5 text-xs">
					{icon}
					{label}
				</div>
				{badge ||
					(value == null || value === "—" ? (
						<Skeleton className="h-6 w-16" />
					) : (
						<div
							className={`text-lg font-bold ${dim ? "text-faint-foreground" : ""}`}
						>
							{value}
						</div>
					))}
			</div>
		</div>
	);
}

function SectionTitle({ children }: { children: ReactNode }) {
	return (
		<div className="text-muted-foreground mt-1 mb-1 text-xs font-semibold tracking-wide">
			{children}
		</div>
	);
}

type UpdateStatus = "idle" | "checking" | "latest" | "available" | "error";

/** 只允许 http/https 的发布页链接：后端字段来自 GitHub API，不能直接信任。 */
export function safeHttpUrl(raw: string | undefined): string | null {
	if (!raw) return null;
	try {
		const url = new URL(raw);
		return url.protocol === "http:" || url.protocol === "https:"
			? url.href
			: null;
	} catch {
		return null;
	}
}

/**
 * 服务端更新检查卡片。仅平台管理员（role === "admin"）可见：
 * 路由本身按 role:manage 门控，但更新检查端点与卡片都只对 admin 开放，
 * 与后端 /system/update/* 的 RequireRole("admin") 保持一致。
 */
function ServerUpdateCard() {
	const isAdmin = useUserStore((s) => s.user?.role === "admin");

	const [version, setVersion] = useState("");
	const [versionFailed, setVersionFailed] = useState(false);
	const [status, setStatus] = useState<UpdateStatus>("idle");
	const [info, setInfo] = useState<ServerUpdateInfo | null>(null);
	const [error, setError] = useState("");

	useEffect(() => {
		if (!isAdmin) return;
		void getServerVersion()
			.then((v) => setVersion(v.version))
			// 拉取失败不能让「当前版本」永远停在「…」：明确回退成「未知」。
			.catch(() => setVersionFailed(true));
	}, [isAdmin]);

	if (!isAdmin) return null;

	const releaseUrl = safeHttpUrl(info?.url);

	const check = async () => {
		setStatus("checking");
		setError("");
		try {
			const result = await checkServerUpdate();
			setInfo(result);
			if (result.current) setVersion(result.current);
			setStatus(result.available ? "available" : "latest");
		} catch (e) {
			setStatus("error");
			setError(e instanceof Error ? e.message : String(e));
		}
	};

	return (
		<>
			<SectionTitle>软件更新</SectionTitle>
			<div className="border-border/80 bg-card rounded-lg border shadow-sm">
				<div className="flex items-center justify-between gap-4 p-3">
					<div className="min-w-0">
						<div className="text-sm font-medium">
							当前版本
							<span className="text-faint-foreground ml-2 font-normal">
								{version || (versionFailed ? "未知" : "…")}
							</span>
						</div>
						{status === "checking" && (
							<div className="text-faint-foreground text-xs">
								正在检查 GitHub Releases…
							</div>
						)}
						{status === "latest" && <div className="text-xs">已是最新版本</div>}
						{status === "error" && (
							<div className="text-destructive text-xs">
								检查失败：{error || "网络异常"}
							</div>
						)}
					</div>
					<Button
						size="sm"
						variant="outline"
						disabled={status === "checking"}
						onClick={() => void check()}
					>
						检查更新
					</Button>
				</div>

				{status === "available" && info && (
					<div className="border-t border-border px-3 py-2">
						<div className="text-sm">
							发现新版本
							<span className="ml-2 font-medium">{info.latest}</span>
							{releaseUrl && (
								<a
									href={releaseUrl}
									target="_blank"
									rel="noreferrer"
									className="text-primary ml-3 text-xs hover:underline"
								>
									查看发布页 ↗
								</a>
							)}
						</div>
						{info.notes ? (
							<div className="text-faint-foreground mt-1 max-h-32 overflow-y-auto whitespace-pre-wrap text-xs">
								{info.notes}
							</div>
						) : null}
					</div>
				)}
			</div>
		</>
	);
}

function MonitorPage() {
	const [status, setStatus] = useState("连接中...");
	const [data, setData] = useState<HealthSnapshot | null>(null);
	const [error, setError] = useState<string | null>(null);
	const abortRef = useRef<AbortController | null>(null);

	useEffect(() => {
		abortRef.current = new AbortController();
		void (async () => {
			try {
				const res = await fetch("/api/v1/system/stream", {
					credentials: "include",
					signal: abortRef.current?.signal,
				});
				if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`);
				setStatus("已连接");
				const reader = res.body.getReader();
				const decoder = new TextDecoder();
				let buffer = "";
				for (;;) {
					const { done, value } = await reader.read();
					if (done) break;
					buffer += decoder.decode(value, { stream: true });
					const events = buffer.split("\n\n");
					buffer = events.pop() || "";
					for (const raw of events) {
						const line = raw.split("\n").find((l) => l.startsWith("data: "));
						if (!line) continue;
						try {
							setData(JSON.parse(line.slice(6)));
							setError(null);
						} catch {
							// malformed message, skip
						}
					}
				}
				setStatus("连接已关闭");
			} catch {
				setStatus("连接中断");
				setError("SSE 连接失败");
			}
		})();
		return () => {
			abortRef.current?.abort();
			abortRef.current = null;
		};
	}, []);

	const snap = data;

	return (
		<ManagePage>
			<ManageHeader
				icon={<Activity size={20} />}
				title="服务监控"
				description={status}
			/>

			{error && (
				<div className="border-warning/30 bg-warning/10 text-warning flex items-center rounded-sm px-3 py-2 text-sm">
					{error}
				</div>
			)}

			<ServerUpdateCard />

			<SectionTitle>业务实时</SectionTitle>
			<div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
				<MetricCard
					icon={<Users size={16} />}
					label="在线用户"
					value={
						snap?.hub_online_user_count != null
							? String(snap.hub_online_user_count)
							: "—"
					}
				/>
				<MetricCard
					icon={<Server size={16} />}
					label="活跃房间"
					value={
						snap?.hub_room_count != null ? String(snap.hub_room_count) : "—"
					}
				/>
				<MetricCard
					icon={<Hash size={16} />}
					label="信令连接"
					value={
						snap?.hub_participant_count != null
							? String(snap.hub_participant_count)
							: "—"
					}
				/>
			</div>

			<SectionTitle>Go Runtime</SectionTitle>
			<div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
				<MetricCard
					icon={<Clock size={16} />}
					label="运行时长"
					value={snap?.uptime || "—"}
					dim
				/>
				<MetricCard
					icon={<Cpu size={16} />}
					label="CPU 核心"
					value={snap?.num_cpu != null ? String(snap.num_cpu) : "—"}
				/>
				<MetricCard
					icon={<Hash size={16} />}
					label="Goroutines"
					value={snap?.num_goroutine != null ? String(snap.num_goroutine) : "—"}
				/>
				<MetricCard
					icon={<MemoryStick size={16} />}
					label="堆内存"
					value={snap?.alloc_mb != null ? `${snap.alloc_mb} MB` : "—"}
				/>
				<MetricCard
					icon={<HardDrive size={16} />}
					label="系统内存"
					value={snap?.sys_mb != null ? `${snap.sys_mb} MB` : "—"}
				/>
				<MetricCard
					icon={<Activity size={16} />}
					label="GC 次数"
					value={snap?.num_gc != null ? String(snap.num_gc) : "—"}
				/>
				<MetricCard
					icon={<Activity size={16} />}
					label="GC 暂停"
					value={
						snap?.gc_pause_ms != null
							? `${snap.gc_pause_ms.toFixed(2)} ms`
							: "—"
					}
				/>
				<MetricCard
					icon={<Hash size={16} />}
					label="堆对象数"
					value={snap?.heap_objects != null ? String(snap.heap_objects) : "—"}
				/>
			</div>

			<SectionTitle>系统</SectionTitle>
			<div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
				<MetricCard
					icon={<Hash size={16} />}
					label="进程 PID"
					value={snap?.pid != null ? String(snap.pid) : "—"}
				/>
				<MetricCard
					icon={<Cpu size={16} />}
					label="CPU 使用率"
					value={
						snap?.cpu_percent != null ? `${snap.cpu_percent.toFixed(1)}%` : "—"
					}
				/>
				<MetricCard
					icon={<HardDrive size={16} />}
					label="磁盘使用"
					value={
						snap?.disk_total_mb != null
							? `${snap.disk_used_mb} / ${snap.disk_total_mb} MB`
							: "—"
					}
					badge={
						snap?.disk_percent != null ? (
							<StatusBadge
								ok={(snap.disk_percent ?? 100) < 90}
								label={`${(snap.disk_percent ?? 0).toFixed(1)}%`}
							/>
						) : undefined
					}
				/>
			</div>

			<SectionTitle>数据库连接池</SectionTitle>
			<div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
				<MetricCard
					icon={<Database size={16} />}
					label="数据库"
					value={null}
					badge={
						<StatusBadge
							ok={!!snap?.db_connected}
							label={snap?.db_connected ? "正常" : "离线"}
						/>
					}
				/>
				<MetricCard
					icon={<Database size={16} />}
					label="活跃连接"
					value={snap?.db_in_use != null ? String(snap.db_in_use) : "—"}
				/>
				<MetricCard
					icon={<Database size={16} />}
					label="空闲连接"
					value={snap?.db_idle != null ? String(snap.db_idle) : "—"}
				/>
				<MetricCard
					icon={<Database size={16} />}
					label="最大连接数"
					value={snap?.db_max_open != null ? String(snap.db_max_open) : "—"}
				/>
				<MetricCard
					icon={<Clock size={16} />}
					label="等待次数"
					value={snap?.db_wait_count != null ? String(snap.db_wait_count) : "—"}
				/>
				<MetricCard
					icon={<Clock size={16} />}
					label="等待耗时"
					value={
						snap?.db_wait_duration_ms != null
							? `${snap.db_wait_duration_ms} ms`
							: "—"
					}
				/>
			</div>

			<div className="text-faint-foreground text-right text-xs">
				{snap?.timestamp ? `最近更新: ${snap.timestamp}` : "无数据"}
			</div>
		</ManagePage>
	);
}
