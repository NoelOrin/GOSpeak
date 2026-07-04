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
	audioElement.style.display = "none";
	document.body.appendChild(audioElement);
	audioElements.set(identity, audioElement);
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

export function setupAudioHandler(client: SFUClient) {
	cleanupAudioHandler();
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
