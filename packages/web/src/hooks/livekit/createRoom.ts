import {
	type LocalParticipant,
	type LocalTrackPublication,
	type Participant,
	type RemoteParticipant,
	type RemoteTrack,
	type RemoteTrackPublication,
	Room,
	RoomEvent,
	Track,
	VideoPresets,
} from "livekit-client";
import { _joinRoom, _leaveRoom } from "./roomAction";

interface UseRoomProps {
	token: string;
	url: string;
}

const _useRoom = ({ token, url }: UseRoomProps) => {
	const room = new Room({
		// 启用自适应流：自动管理订阅的视频画质（根据带宽动态调整）
		adaptiveStream: true,

		// 启用 Dynacast：优化发布轨道的带宽和 CPU 使用（仅向需要的订阅者发送流）
		dynacast: true,

		stopLocalTrackOnUnpublish: true, // 自动清理资源
		disconnectOnPageLeave: true,

		// 默认的视频采集设置
		videoCaptureDefaults: {
			resolution: VideoPresets.h720.resolution, // 默认使用 720p 分辨率
		},

		// 音频采集默认设置
		audioCaptureDefaults: {
			echoCancellation: true, // 回音消除
			noiseSuppression: true, // 噪音抑制
			autoGainControl: true, // 自动增益控制
		},
		publishDefaults: {},
		webAudioMix: true,

		// 可配置重连策略
		reconnectPolicy: {
			nextRetryDelayInMs: () => 300,
		},
	});

	// 预热连接（可尽早调用，例如页面加载时）
	room.prepareConnection(url, token);

	// 设置事件监听器
	room
		.on(RoomEvent.TrackSubscribed, handleTrackSubscribed) // 订阅远程音视频轨道时触发
		.on(RoomEvent.TrackUnsubscribed, handleTrackUnsubscribed) // 取消订阅远程轨道时触发
		.on(RoomEvent.ActiveSpeakersChanged, handleActiveSpeakerChange) // 活跃说话者列表变化时触发
		.on(RoomEvent.Disconnected, handleDisconnect) // 与房间F断开连接时触发
		.on(RoomEvent.LocalTrackUnpublished, handleLocalTrackUnpublished); // 本地轨道停止发布时触发


	const joinRoom = () => _joinRoom(room, url, token);

	const leaveRoom = () => _leaveRoom(room);

	return { room, joinRoom, leaveRoom };
};

const createRoom = (props: UseRoomProps) => _useRoom(props);
export type RoomReturnType = Pick<
	ReturnType<typeof _useRoom>,
	"room" | "joinRoom" | "leaveRoom"
>;
// const useRoom = _useRoom;

// 处理订阅的远程轨道
function handleTrackSubscribed(
	track: RemoteTrack,
	publication: RemoteTrackPublication,
	participant: RemoteParticipant,
) {
	console.log("Subscribed to remote track:", track.sid, track.kind);
	if (track.kind === Track.Kind.Video || track.kind === Track.Kind.Audio) {
		// 将轨道绑定到新的 HTMLVideoElement 或 HTMLAudioElement 元素
	}
}

// 处理取消订阅的远程轨道
function handleTrackUnsubscribed(
	track: RemoteTrack,
	publication: RemoteTrackPublication,
	participant: RemoteParticipant,
) {
	console.log("Unsubscribed from remote track:", track.sid);
	// 从所有已绑定的 DOM 元素中移除该轨道
	track.detach();
}

// 处理本地轨道停止发布
function handleLocalTrackUnpublished(
	publication: LocalTrackPublication,
	participant: LocalParticipant,
) {
	console.log("Local track unpublished:", publication.trackSid);
	// 当本地轨道被关闭时，从 UI 中移除对应的渲染元素
	publication.track?.detach();
}

// 处理活跃说话者变化
function handleActiveSpeakerChange(speakers: Participant[]) {
	console.log(
		"Active speakers changed:",
		speakers.map((s) => s.identity),
	);
	// 可在此处更新 UI，例如高亮当前正在说话的参与者
}

// 处理断开连接
function handleDisconnect() {
	console.log("已与房间断开连接");
}

export default createRoom;
