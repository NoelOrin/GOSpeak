import { create } from "zustand";
import {
	enumerateAudioDevices,
	getResolvedInputDeviceId,
	requestAudioPermission,
} from "../hooks/media";
import { useAudioDeviceStore } from "./audioDeviceStore";

/** 单个设备维度的切换建议：从哪个设备换到哪个设备 */
export interface DeviceSwap {
	/** 变化前的设备；旧设备已拔出时为 null */
	from: { id: string; label: string } | null;
	/** 建议切换到的设备 */
	to: { id: string; label: string };
}

export interface PendingDeviceChange {
	inputs: DeviceSwap | null;
	outputs: DeviceSwap | null;
}

interface DeviceChangeState {
	/** 待用户确认的设备变化；null 表示当前无提示 */
	pending: PendingDeviceChange | null;
}

export const useDeviceChangeStore = create<DeviceChangeState>(() => ({
	pending: null,
}));

type DeviceEntry = { id: string; label: string };

interface DeviceSnapshot {
	inputs: DeviceEntry[];
	outputs: DeviceEntry[];
}

// 浏览器为“系统默认”额外生成的伪设备，永远存在，不参与新增/移除判定
const PSEUDO_DEVICE_IDS = new Set(["default", "communications"]);

let snapshot: DeviceSnapshot | null = null;
let removeListener: (() => void) | null = null;
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

function toEntries(list: { deviceId: string; label: string }[]): DeviceEntry[] {
	return list.map((d) => ({ id: d.deviceId, label: d.label }));
}

function realDevices(list: DeviceEntry[]): DeviceEntry[] {
	return list.filter((d) => !PSEUDO_DEVICE_IDS.has(d.id));
}

function labelFor(list: DeviceEntry[], id: string): string {
	return list.find((d) => d.id === id)?.label || "";
}

/**
 * 计算某个维度是否需要提示切换。
 * - 已选设备被拔出：建议切到系统默认 / 第一个可用设备
 * - 插入了新设备：建议切到新设备（不打断原有可用选择，但仍询问是否切换）
 * - 已选设备仍可用且无新设备：不提示
 */
function computeSwap(
	prev: DeviceEntry[],
	current: DeviceEntry[],
	selectedId: string,
): DeviceSwap | null {
	// 用户选的是“系统默认”（空值）时交给浏览器自动跟随，不打搅
	if (!selectedId) return null;

	const currentReal = realDevices(current);
	const prevIds = new Set(prev.map((d) => d.id));
	const currentIds = new Set(current.map((d) => d.id));

	const added = currentReal.filter((d) => !prevIds.has(d.id));
	const removed = prev.filter((d) => !currentIds.has(d.id));
	if (added.length === 0 && removed.length === 0) return null;

	const selectedAlive = current.some((d) => d.id === selectedId);
	if (!selectedAlive) {
		// 优先落到真实设备，避免回退到 "default" 伪设备后丢失后续跟随能力
		const fallback =
			currentReal[0] ?? current.find((d) => PSEUDO_DEVICE_IDS.has(d.id));
		if (!fallback || fallback.id === selectedId) return null;
		return {
			from: { id: selectedId, label: labelFor(prev, selectedId) },
			to: { id: fallback.id, label: fallback.label },
		};
	}

	// 已选设备还在，但有新设备接入：询问是否切到新设备
	const candidate = added[0];
	if (!candidate) return null;
	return {
		from: {
			id: selectedId,
			label: labelFor(current, selectedId) || labelFor(prev, selectedId),
		},
		to: { id: candidate.id, label: candidate.label },
	};
}

/** 已选设备失效时静默回退到第一个可用设备（不弹窗场景） */
function reconcileSelection(inputs: DeviceEntry[], outputs: DeviceEntry[]) {
	const audio = useAudioDeviceStore.getState();
	if (
		audio.selectedAudioInput &&
		!inputs.some((d) => d.id === audio.selectedAudioInput)
	) {
		audio.setSelectedAudioInput(fallbackId(inputs));
	}
	if (
		audio.selectedAudioOutput &&
		!outputs.some((d) => d.id === audio.selectedAudioOutput)
	) {
		audio.setSelectedAudioOutput(fallbackId(outputs));
	}
}

/** 回退目标：优先真实设备，其次才用 default / communications 伪设备 */
function fallbackId(list: DeviceEntry[]): string {
	return (realDevices(list)[0] ?? list[0])?.id ?? "";
}

async function detect() {
	// 等启动时那次权限申请落地：未授权时 deviceId 为空，diff 出来全是噪声
	await requestAudioPermission();

	let devices: Awaited<ReturnType<typeof enumerateAudioDevices>>;
	try {
		devices = await enumerateAudioDevices();
	} catch (error) {
		console.warn("[audio] 设备变化检测失败", error);
		return;
	}

	const current: DeviceSnapshot = {
		inputs: toEntries(devices.audioinputs),
		outputs: toEntries(devices.audiooutputs),
	};

	// 未授权麦克风时浏览器返回的是空 deviceId，无法可靠 diff：
	// 既不建立基线也不提示，等用户真正授权后的下一次变化再说。
	const usable =
		current.inputs.some((d) => d.id) || current.outputs.some((d) => d.id);
	if (!usable) return;

	const state = useAudioDeviceStore.getState();

	// 监听启动时设备列表可能还没被授权加载过：先用 store 里已知的设备补基线，
	// 否则“授权后的第一次插拔”会被当成建立基线而漏掉提示。
	if (!snapshot) {
		const seedInputs = toEntries(state.audioinputs).filter((d) => d.id);
		const seedOutputs = toEntries(state.audiooutputs).filter((d) => d.id);
		if (seedInputs.length || seedOutputs.length) {
			snapshot = { inputs: seedInputs, outputs: seedOutputs };
		}
	}

	// 仍然没有基线：只建快照 + 刷新列表，不做切换判断
	if (!snapshot) {
		snapshot = current;
		useAudioDeviceStore.getState().syncDeviceList(devices);
		return;
	}

	const prev = snapshot;
	snapshot = current;
	useAudioDeviceStore.getState().syncDeviceList(devices);

	const pending: PendingDeviceChange = {
		inputs: computeSwap(prev.inputs, current.inputs, state.selectedAudioInput),
		outputs: computeSwap(
			prev.outputs,
			current.outputs,
			state.selectedAudioOutput,
		),
	};

	if (!pending.inputs && !pending.outputs) {
		reconcileSelection(current.inputs, current.outputs);
		useDeviceChangeStore.setState({ pending: null });
		return;
	}

	// 用户勾选过“以后不再显示”：静默沿用旧选择，失效才回退
	if (!state.deviceChangeNotice) {
		reconcileSelection(current.inputs, current.outputs);
		useDeviceChangeStore.setState({ pending: null });
		return;
	}

	useDeviceChangeStore.setState({ pending });
}

function scheduleDetect() {
	if (debounceTimer) clearTimeout(debounceTimer);
	// devicechange 常常连续触发多次（一次插拔可能报两条），合并处理
	debounceTimer = setTimeout(() => {
		debounceTimer = null;
		void detect();
	}, 250);
}

/**
 * 启动全局设备变化监听。幂等：重复调用只会重绑一次。
 * 返回解绑函数。
 */
export function startDeviceChangeWatch() {
	if (typeof navigator === "undefined" || !navigator.mediaDevices) {
		return () => {};
	}

	if (!removeListener) {
		const md = navigator.mediaDevices;
		md.addEventListener("devicechange", scheduleDetect);
		removeListener = () =>
			md.removeEventListener("devicechange", scheduleDetect);
		// 权限落地后再建基线：此时 deviceId / label 才真实可用。
		// 顺带补一个默认设备选择，否则「系统默认」空值无法跟踪具体设备变化。
		void requestAudioPermission().then((granted) => {
			if (granted) {
				useAudioDeviceStore
					.getState()
					.ensureSelectedAudioInput(getResolvedInputDeviceId());
			}
			void detect();
		});
	}

	return () => {
		if (debounceTimer) {
			clearTimeout(debounceTimer);
			debounceTimer = null;
		}
		removeListener?.();
		removeListener = null;
		snapshot = null;
		useDeviceChangeStore.setState({ pending: null });
	};
}

const DeviceChangeStore = {
	/** 采用建议的设备（保留未变化的维度） */
	confirm: () => {
		const { pending } = useDeviceChangeStore.getState();
		if (!pending) return;
		const audio = useAudioDeviceStore.getState();
		if (pending.inputs) {
			audio.setSelectedAudioInput(pending.inputs.to.id);
		}
		if (pending.outputs) {
			audio.setSelectedAudioOutput(pending.outputs.to.id);
		}
		useDeviceChangeStore.setState({ pending: null });
	},
	/** 保持原选择；原设备已不存在时回退到可用设备 */
	cancel: () => {
		const state = useAudioDeviceStore.getState();
		reconcileSelection(
			toEntries(state.audioinputs),
			toEntries(state.audiooutputs),
		);
		useDeviceChangeStore.setState({ pending: null });
	},
};

export default DeviceChangeStore;
