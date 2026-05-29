import type { Room } from "livekit-client";

const _joinRoom = async (room: Room, url: string, token: string) => {
	// console.log("[Room] 尝试连接到房间", room.name);
	await room.connect(url, token);
	// console.log("[Room] 已成功连接到房间", room.name);

	// 发布本地麦克风音轨
	await room.localParticipant.setMicrophoneEnabled(true);
	// console.log("[Room] 麦克风已启用, localTracks:", room.localParticipant.trackPublications.size);

	// // 检查远端参与者及其轨道
	// room.remoteParticipants.forEach((participant, key) => {
	// 	console.log("[Room] 远端参与者:", key, participant.identity, "tracks:", participant.trackPublications.size);
	// 	participant.trackPublications.forEach((pub) => {
	// 		console.log("[Room]   远端轨道:", pub.trackSid, "kind:", pub.kind, "muted:", pub.isMuted, "subscribed:", pub.isSubscribed);
	// 	});
	// });
};

const _leaveRoom = async (room: Room) => {
	await room.disconnect();
};

export { _joinRoom, _leaveRoom };
