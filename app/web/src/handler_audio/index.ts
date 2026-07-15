import type {
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUClient,
} from "@gospeak/sfu-client/types";
import { setSpeakingIdentities } from "./speakingStore";

const tracks = new Map<string, RemoteAudioTrackLike>();
const audioElements = new Map<string, HTMLMediaElement>();
const mutedIdentities = new Set<string>();
const volumes = new Map<string, number>();

// 全局输出：masterVolume 0-1，masterMuted true=静音所有远端音频。
// 与 per-member volume/mute 叠乘：最终 = masterMuted ? 0 : masterVolume * memberVolume。
let masterVolume = 1;
let masterMuted = false;

function effectiveVolume(identity: string): number {
	if (masterMuted || mutedIdentities.has(identity)) return 0;
	return masterVolume * (volumes.get(identity) ?? 1);
}

function onTrackSubscribed({ identity, track }: RemoteTrackInfo) {
	if (tracks.get(identity) === track) return;
	tracks.set(identity, track);
	track.setVolume(effectiveVolume(identity));

	const audioElement = track.attach();
	audioElement.autoplay = true;
	audioElement.style.display = "none";
	document.body.appendChild(audioElement);
	audioElements.set(identity, audioElement);
	void applySinkId(audioElement);
	const playResult = audioElement.play?.();
	if (playResult && typeof (playResult as Promise<void>).catch === "function") {
		void (playResult as Promise<void>).catch(() => {
			// autoplay may be blocked until user gesture; volume/mute controls remain valid
		});
	}
}

function onTrackUnsubscribed(identity: string) {
	const track = tracks.get(identity);
	track?.detach();
	tracks.delete(identity);
	const el = audioElements.get(identity);
	if (el?.parentNode) {
		el.parentNode.removeChild(el);
	}
	audioElements.delete(identity);
}

let boundClient: SFUClient | null = null;

let preferredSinkId = "";

async function applySinkId(el: HTMLMediaElement) {
	const anyEl = el as HTMLMediaElement & {
		setSinkId?: (id: string) => Promise<void>;
	};
	if (!preferredSinkId || typeof anyEl.setSinkId !== "function") return;
	try {
		await anyEl.setSinkId(preferredSinkId);
	} catch (err) {
		console.warn("[audio] setSinkId failed", err);
	}
}

/** 设置远端音频输出设备（扬声器）。空字符串表示系统默认。 */
export function setAudioOutputDevice(deviceId: string) {
	preferredSinkId = deviceId || "";
	for (const el of audioElements.values()) {
		void applySinkId(el);
	}
}

export function setupAudioHandler(client: SFUClient) {
	if (boundClient === client) return;
	cleanupAudioHandler();
	boundClient = client;
	client.onRemoteAudioTrack(onTrackSubscribed);
	client.onRemoteAudioTrackRemoved(onTrackUnsubscribed);
	client.onActiveSpeakers(setSpeakingIdentities);
	// 补齐 join 竞态：joinRoom 已订阅但早于回调注册的远端 track。
	for (const info of client.getExistingRemoteAudioTracks()) {
		onTrackSubscribed(info);
	}
}

export function cleanupAudioHandler() {
	tracks.forEach((track) => {
		track.detach();
	});
	tracks.clear();
	audioElements.forEach((el) => {
		if (el.parentNode) el.parentNode.removeChild(el);
	});
	audioElements.clear();
	mutedIdentities.clear();
	volumes.clear();
	masterVolume = 1;
	masterMuted = false;
	boundClient = null;
}

export function setVolumeByIdentity(identity: string, volume: number) {
	volumes.set(identity, volume);
	const track = tracks.get(identity);
	if (track) {
		track.setVolume(effectiveVolume(identity));
	}
}

export function setMutedByIdentity(identity: string, muted: boolean) {
	if (muted) {
		mutedIdentities.add(identity);
	} else {
		mutedIdentities.delete(identity);
	}
	const track = tracks.get(identity);
	if (track) {
		track.setVolume(effectiveVolume(identity));
	}
}

// 全局输出音量（0-1），应用到所有已订阅远端 track。调用方需自行归一化（如 0-100 滑块 / 100）。
export function setMasterVolume(volume: number) {
	masterVolume = Math.max(0, Math.min(1, volume));
	for (const identity of tracks.keys()) {
		tracks.get(identity)?.setVolume(effectiveVolume(identity));
	}
}

// 全局输出静音，应用到所有已订阅远端 track。
export function setMasterMuted(muted: boolean) {
	masterMuted = muted;
	for (const identity of tracks.keys()) {
		tracks.get(identity)?.setVolume(effectiveVolume(identity));
	}
}
