import { createFileRoute, redirect } from "@tanstack/solid-router";
import Activity from "lucide-solid/icons/activity";
import CheckCircle2 from "lucide-solid/icons/check-circle";
import Clock from "lucide-solid/icons/clock";
import Cpu from "lucide-solid/icons/cpu";
import Database from "lucide-solid/icons/database";
import HardDrive from "lucide-solid/icons/hard-drive";
import Hash from "lucide-solid/icons/hash";
import MemoryStick from "lucide-solid/icons/memory-stick";
import Radio from "lucide-solid/icons/radio";
import Server from "lucide-solid/icons/server";
import Users from "lucide-solid/icons/users";
import XCircle from "lucide-solid/icons/x-circle";
import { createSignal, onCleanup, onMount } from "solid-js";
import userStore from "@/stores/userStore";

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
	redis_connected: boolean;
	redis_ping_ms: number;
	redis_db_size: number;
	redis_used_memory_mb: number;
	redis_used_memory_peak_mb: number;
	redis_connected_clients: number;
}

export const Route = createFileRoute("/(app)/manage/monitor/")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
			throw redirect({ to: "/" });
		}
	},
	component: MonitorPage,
	staticData: {
		title: "服务监控",
		icon: "icon-manage",
	},
});

function StatusBadge(props: { ok: boolean; label: string }) {
	return (
		<div class="flex items-center gap-1.5">
			{props.ok ? (
				<CheckCircle2 size={14} class="text-success" />
			) : (
				<XCircle size={14} class="text-error" />
			)}
			<span class="text-xs">{props.label}</span>
		</div>
	);
}

function MonitorPage() {
	const [status, setStatus] = createSignal<string>("连接中...");
	const [data, setData] = createSignal<HealthSnapshot | null>(null);
	const [error, setError] = createSignal<string | null>(null);
	let eventSource: EventSource | null = null;

	onMount(() => {
		const token = userStore.accessToken();
		eventSource = new EventSource(
			`/api/v1/system/stream?token=${encodeURIComponent(token)}`,
		);
		eventSource.onopen = () => setStatus("已连接");
		eventSource.onmessage = (e) => {
			try {
				setData(JSON.parse(e.data));
				setError(null);
			} catch {
				// malformed message, skip
			}
		};
		eventSource.onerror = () => {
			setStatus("连接中断");
			setError("SSE 连接失败，正在重连...");
		};
	});

	onCleanup(() => {
		eventSource?.close();
	});

	const snap = () => data();

	return (
		<div class="flex h-full min-h-0 flex-col gap-4 overflow-y-auto p-4">
			<div class="flex items-center gap-2">
				<Activity size={18} />
				<h3 class="font-bold text-lg">服务监控</h3>
				<span class="text-xs text-base-content/50 ml-2">{status()}</span>
			</div>

			{/* Connection Error */}
			{error() && <div class="alert alert-warning text-sm py-2">{error()}</div>}

			{/* 业务指标 */}
			<SectionTitle>业务实时</SectionTitle>
			<div class="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
				<MetricCard
					icon={<Users size={16} />}
					label="在线用户"
					value={
						snap()?.hub_online_user_count != null
							? String(snap()?.hub_online_user_count)
							: "—"
					}
				/>
				<MetricCard
					icon={<Server size={16} />}
					label="活跃房间"
					value={
						snap()?.hub_room_count != null
							? String(snap()?.hub_room_count)
							: "—"
					}
				/>
				<MetricCard
					icon={<Hash size={16} />}
					label="信令连接"
					value={
						snap()?.hub_participant_count != null
							? String(snap()?.hub_participant_count)
							: "—"
					}
				/>
			</div>

			{/* Go runtime */}
			<SectionTitle>Go Runtime</SectionTitle>
			<div class="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
				<MetricCard
					icon={<Clock size={16} />}
					label="运行时长"
					value={snap()?.uptime || "—"}
					dim
				/>
				<MetricCard
					icon={<Cpu size={16} />}
					label="CPU 核心"
					value={snap()?.num_cpu != null ? String(snap()?.num_cpu) : "—"}
				/>
				<MetricCard
					icon={<Hash size={16} />}
					label="Goroutines"
					value={
						snap()?.num_goroutine != null ? String(snap()?.num_goroutine) : "—"
					}
				/>
				<MetricCard
					icon={<MemoryStick size={16} />}
					label="堆内存"
					value={snap()?.alloc_mb != null ? `${snap()?.alloc_mb} MB` : "—"}
				/>
				<MetricCard
					icon={<HardDrive size={16} />}
					label="系统内存"
					value={snap()?.sys_mb != null ? `${snap()?.sys_mb} MB` : "—"}
				/>
				<MetricCard
					icon={<Activity size={16} />}
					label="GC 次数"
					value={snap()?.num_gc != null ? String(snap()?.num_gc) : "—"}
				/>
				<MetricCard
					icon={<Activity size={16} />}
					label="GC 暂停"
					value={
						snap()?.gc_pause_ms != null
							? `${snap()?.gc_pause_ms.toFixed(2)} ms`
							: "—"
					}
				/>
				<MetricCard
					icon={<Hash size={16} />}
					label="堆对象数"
					value={
						snap()?.heap_objects != null ? String(snap()?.heap_objects) : "—"
					}
				/>
			</div>

			{/* 系统级 */}
			<SectionTitle>系统</SectionTitle>
			<div class="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
				<MetricCard
					icon={<Hash size={16} />}
					label="进程 PID"
					value={snap()?.pid != null ? String(snap()?.pid) : "—"}
				/>
				<MetricCard
					icon={<Cpu size={16} />}
					label="CPU 使用率"
					value={
						snap()?.cpu_percent != null
							? `${snap()?.cpu_percent.toFixed(1)}%`
							: "—"
					}
				/>
				<MetricCard
					icon={<HardDrive size={16} />}
					label="磁盘使用"
					value={
						snap()?.disk_total_mb != null
							? `${snap()?.disk_used_mb} / ${snap()?.disk_total_mb} MB`
							: "—"
					}
					badge={
						snap()?.disk_percent != null ? (
							<StatusBadge
								ok={(snap()?.disk_percent ?? 100) < 90}
								label={`${(snap()?.disk_percent ?? 0).toFixed(1)}%`}
							/>
						) : undefined
					}
				/>
			</div>

			{/* 数据库连接池 */}
			<SectionTitle>数据库连接池</SectionTitle>
			<div class="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
				<MetricCard
					icon={<Database size={16} />}
					label="数据库"
					value={null}
					badge={
						<StatusBadge
							ok={!!snap()?.db_connected}
							label={snap()?.db_connected ? "正常" : "离线"}
						/>
					}
				/>
				<MetricCard
					icon={<Database size={16} />}
					label="活跃连接"
					value={snap()?.db_in_use != null ? String(snap()?.db_in_use) : "—"}
				/>
				<MetricCard
					icon={<Database size={16} />}
					label="空闲连接"
					value={snap()?.db_idle != null ? String(snap()?.db_idle) : "—"}
				/>
				<MetricCard
					icon={<Database size={16} />}
					label="最大连接数"
					value={
						snap()?.db_max_open != null ? String(snap()?.db_max_open) : "—"
					}
				/>
				<MetricCard
					icon={<Clock size={16} />}
					label="等待次数"
					value={
						snap()?.db_wait_count != null ? String(snap()?.db_wait_count) : "—"
					}
				/>
				<MetricCard
					icon={<Clock size={16} />}
					label="等待耗时"
					value={
						snap()?.db_wait_duration_ms != null
							? `${snap()?.db_wait_duration_ms} ms`
							: "—"
					}
				/>
			</div>

			{/* Redis */}
			<SectionTitle>Redis</SectionTitle>
			<div class="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
				<MetricCard
					icon={<Radio size={16} />}
					label="Redis"
					value={null}
					badge={
						<StatusBadge
							ok={!!snap()?.redis_connected}
							label={snap()?.redis_connected ? "连接" : "未配置"}
						/>
					}
				/>
				<MetricCard
					icon={<Clock size={16} />}
					label="Ping 延迟"
					value={
						snap()?.redis_ping_ms != null ? `${snap()?.redis_ping_ms} ms` : "—"
					}
				/>
				<MetricCard
					icon={<Hash size={16} />}
					label="Key 总数"
					value={
						snap()?.redis_db_size != null ? String(snap()?.redis_db_size) : "—"
					}
				/>
				<MetricCard
					icon={<MemoryStick size={16} />}
					label="内存占用"
					value={
						snap()?.redis_used_memory_mb != null
							? `${snap()?.redis_used_memory_mb} MB`
							: "—"
					}
				/>
				<MetricCard
					icon={<MemoryStick size={16} />}
					label="内存峰值"
					value={
						snap()?.redis_used_memory_peak_mb != null
							? `${snap()?.redis_used_memory_peak_mb} MB`
							: "—"
					}
				/>
				<MetricCard
					icon={<Users size={16} />}
					label="客户端连接"
					value={
						snap()?.redis_connected_clients != null
							? String(snap()?.redis_connected_clients)
							: "—"
					}
				/>
			</div>

			<div class="text-xs text-base-content/40 text-right">
				{snap()?.timestamp ? `最近更新: ${snap()?.timestamp}` : "无数据"}
			</div>
		</div>
	);
}

function MetricCard(props: {
	icon: any;
	label: string;
	value: string | null;
	dim?: boolean;
	badge?: any;
}) {
	return (
		<div class="card bg-base-200/50 shadow-sm">
			<div class="card-body p-3">
				<div class="flex items-center gap-1.5 text-base-content/60 text-xs mb-1">
					{props.icon}
					{props.label}
				</div>
				{props.badge || (
					<div
						class="text-lg font-bold"
						classList={{ "text-base-content/40": props.dim }}
					>
						{props.value}
					</div>
				)}
			</div>
		</div>
	);
}

function SectionTitle(props: { children: any }) {
	return (
		<div class="mt-2 mb-1 flex items-center gap-2 text-base-content/70 text-xs font-semibold tracking-wide uppercase">
			<span class="bg-base-content/30 h-3 w-1 rounded-full" />
			{props.children}
		</div>
	);
}
