import type { Room } from "livekit-client";

const _joinRoom = async (room: Room, url: string, token: string) => {
	console.log("尝试连接到房间", room.name);
	await room.connect(url, token);
	console.log("已成功连接到房间", room.name);
	// 发布本地摄像头和麦克风音视频轨道
	// await room.localParticipant.setCameraEnabled(true);
	await room.localParticipant.setMicrophoneEnabled(true);
};

const _leaveRoom = async (room: Room) => {
	await room.disconnect();
};

export { _joinRoom, _leaveRoom };
