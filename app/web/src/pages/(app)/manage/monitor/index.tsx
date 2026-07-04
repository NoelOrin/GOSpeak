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
	db_connected: boolean;
	redis_connected: boolean;
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
		eventSource = new EventSource("/api/v1/system/stream");
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
		<div class="flex h-full min-h-0 flex-col gap-4 p-4">
			<div class="flex items-center gap-2">
				<Activity size={18} />
				<h3 class="font-bold text-lg">服务监控</h3>
				<span class="text-xs text-base-content/50 ml-2">{status()}</span>
			</div>

			{/* Connection Error */}
			{error() && <div class="alert alert-warning text-sm py-2">{error()}</div>}

			{/* Metric Cards */}
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
