export interface AudioDeviceInfo {
	deviceId: string;
	groupId: string;
	label: string;
	kind: "audioinput" | "audiooutput";
}

let audioPermissionPromise: Promise<boolean> | null = null;
/** 授权时浏览器实际打开的那个麦克风的 deviceId（不是 "default" 伪设备） */
let resolvedInputDeviceId = "";

/** 读取启动授权时解析到的真实输入设备 id，未授权时为空串 */
export function getResolvedInputDeviceId(): string {
	return resolvedInputDeviceId;
}

/**
 * 应用启动时申请一次麦克风权限。
 *
 * 授权后 `enumerateDevices()` 才会返回真实 deviceId / label，
 * 设备变化检测、设备列表、热切换都依赖这一点。
 * 幂等：多次调用共用同一个 Promise，不会重复弹权限框。
 *
 * 只缓存成功的结果：被拒绝/失败后再调用会重新申请。否则用户去系统设置里
 * 放行后点「重试」，拿到的仍是那个已失败的 Promise，按钮等于失效。
 */
export function requestAudioPermission(): Promise<boolean> {
	if (
		typeof navigator === "undefined" ||
		!navigator.mediaDevices?.getUserMedia
	) {
		return Promise.resolve(false);
	}
	if (!audioPermissionPromise) {
		audioPermissionPromise = navigator.mediaDevices
			.getUserMedia({ audio: true })
			.then((stream) => {
				const track = stream.getAudioTracks()[0];
				const actualId = track?.getSettings().deviceId ?? "";
				if (actualId && actualId !== "default") {
					resolvedInputDeviceId = actualId;
				}
				for (const t of stream.getTracks()) t.stop();
				return true;
			})
			.catch((error) => {
				console.warn("[audio] 麦克风权限申请失败", error);
				// 失败不留在缓存里，下次调用重新申请。
				audioPermissionPromise = null;
				return false;
			});
	}
	return audioPermissionPromise;
}

/** 把 getUserMedia 的失败原样抛出，供调用方区分权限与设备故障。 */
async function requireAudioPermission(): Promise<void> {
	if (
		typeof navigator === "undefined" ||
		!navigator.mediaDevices?.getUserMedia
	) {
		throw new Error("当前环境不支持麦克风采集");
	}
	if (audioPermissionPromise) {
		// 已经授权过：直接放行，不再申请。
		return;
	}
	const granted = await requestAudioPermission();
	if (granted) return;
	throw Object.assign(new Error("麦克风权限被拒绝"), {
		name: "NotAllowedError",
	});
}

/**
 * 只枚举设备，不申请麦克风权限。
 * 用于 devicechange 后台检测：避免在用户没进房时就弹出权限询问。
 * 未授权时浏览器会返回空 deviceId / 空 label，调用方需自行判断可用性。
 */
export async function enumerateAudioDevices(): Promise<{
	audioinputs: AudioDeviceInfo[];
	audiooutputs: AudioDeviceInfo[];
}> {
	const devices = await navigator.mediaDevices.enumerateDevices();
	return {
		audioinputs: devices
			.filter((d) => d.kind === "audioinput")
			.map((d) => ({
				deviceId: d.deviceId,
				groupId: d.groupId,
				label: d.label,
				kind: "audioinput" as const,
			})),
		audiooutputs: devices
			.filter((d) => d.kind === "audiooutput")
			.map((d) => ({
				deviceId: d.deviceId,
				groupId: d.groupId,
				label: d.label,
				kind: "audiooutput" as const,
			})),
	};
}

/**
 * 带上权限的设备枚举：未授权时先申请一次，拿到真实 label 再返回列表。
 *
 * 权限没拿到时抛错，而不是继续 enumerateDevices() —— 那样只会得到空 label
 * 的空壳设备，界面再把它显示成"读取音频设备失败"，掩盖真正的权限问题。
 */
export async function getAudioDevices(): Promise<{
	audioinputs: AudioDeviceInfo[];
	audiooutputs: AudioDeviceInfo[];
}> {
	await requireAudioPermission();
	return enumerateAudioDevices();
}
