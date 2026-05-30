import type { Room } from "livekit-client";

const _joinRoom = async (room: Room, url: string, token: string) => {
	await room.connect(url, token);
	await room.localParticipant.setMicrophoneEnabled(true);
};

const _leaveRoom = async (room: Room) => {
	// 先停止所有本地轨道发布，释放麦克风/摄像头资源
	await room.localParticipant.setMicrophoneEnabled(false);
	await room.localParticipant.setCameraEnabled(false);
	room.localParticipant.trackPublications.forEach((pub) => {
		pub.track?.stop();
	});
	await room.disconnect();
};

export { _joinRoom, _leaveRoom };
