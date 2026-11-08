import type {
	RemoteVideoTrackInfo,
	RemoteVideoTrackLike,
} from "@gospeak/sfu-client/types";
import { create } from "zustand";

/**
 * 屏幕共享状态。
 *
 * track 对象不放进 React state 之外的地方：这里只负责「谁在共享 / track 是谁」，
 * 真正的 <video> 附着由 ScreenShareView 完成。
 */
interface ScreenShareState {
	/** 自己是否正在共享 */
	localSharing: boolean;
	/** 自己的共享 track（用于自预览） */
	localTrack: RemoteVideoTrackLike | null;
	/** 远端共享（屏幕 / 摄像头） */
	remote: RemoteVideoTrackInfo[];
	setLocalSharing: (sharing: boolean) => void;
	setLocalTrack: (track: RemoteVideoTrackLike | null) => void;
	upsertRemote: (info: RemoteVideoTrackInfo) => void;
	removeRemote: (identity: string) => void;
	reset: () => void;
}

export const useScreenShareStore = create<ScreenShareState>((set) => ({
	localSharing: false,
	localTrack: null,
	remote: [],
	setLocalSharing: (localSharing) => set({ localSharing }),
	setLocalTrack: (localTrack) => set({ localTrack }),
	upsertRemote: (info) =>
		set((s) => {
			const next = s.remote.filter((item) => item.identity !== info.identity);
			next.push(info);
			return { remote: next };
		}),
	removeRemote: (identity) =>
		set((s) => ({
			remote: s.remote.filter((item) => item.identity !== identity),
		})),
	reset: () => set({ localSharing: false, localTrack: null, remote: [] }),
}));
