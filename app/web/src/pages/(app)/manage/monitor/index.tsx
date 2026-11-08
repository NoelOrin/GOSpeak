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
import { ManageHeader, ManagePage } from "@/components/manage/ManageShell";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";

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
				{badge || (
					<div
						className={`text-lg font-bold ${dim ? "text-faint-foreground" : ""}`}
					>
						{value}
					</div>
				)}
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
