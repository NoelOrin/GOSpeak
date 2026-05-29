import {
  type Room,
  type RemoteParticipant,
  type RemoteTrack,
  type RemoteTrackPublication,
  RemoteAudioTrack,
  RoomEvent,
  Track,
} from "livekit-client";

/** identity -> RemoteAudioTrack */
const tracks = new Map<string, RemoteAudioTrack>();

function onTrackSubscribed(
  track: RemoteTrack,
  publication: RemoteTrackPublication,
  participant: RemoteParticipant,
) {
  console.log("[Audio] track subscribed:", track.sid, "kind:", track.kind, "from:", participant.identity, "muted:", publication.isMuted);
  if (track.kind !== Track.Kind.Audio) return;
  const audioTrack = track as RemoteAudioTrack;
  tracks.set(participant.identity, audioTrack);

  // 确保音频轨道附加到播放元素
  const audioElement = audioTrack.attach();
  audioElement.style.display = "none";
  document.body.appendChild(audioElement);
  console.log("[Audio] audio element attached for:", participant.identity, "element:", audioElement);
}

function onTrackUnsubscribed(
  track: RemoteTrack,
  _publication: RemoteTrackPublication,
  participant: RemoteParticipant,
) {
  if (track.kind !== Track.Kind.Audio) return;
  console.log("[Audio] track unsubscribed:", track.sid, "from:", participant.identity);
  tracks.delete(participant.identity);
  // 清理对应的 audio 元素
  (track as RemoteAudioTrack).detach();
}

export function setupAudioHandler(room: Room) {
  console.log("[Audio] setting up audio handler for room:", room.name);
  room
    .on(RoomEvent.TrackSubscribed, onTrackSubscribed)
    .on(RoomEvent.TrackUnsubscribed, onTrackUnsubscribed)
    .on(RoomEvent.TrackPublished, (publication, participant) => {
      console.log("[Audio] track published:", publication.trackSid, "kind:", publication.kind, "by:", participant.identity, "muted:", publication.isMuted);
    })
    .on(RoomEvent.TrackMuted, (publication, participant) => {
      console.log("[Audio] track muted:", publication.trackSid, "by:", participant.identity);
    })
    .on(RoomEvent.TrackUnmuted, (publication, participant) => {
      console.log("[Audio] track unmuted:", publication.trackSid, "by:", participant.identity);
    })
    .on(RoomEvent.ActiveSpeakersChanged, (speakers) => {
      console.log("[Audio] active speakers:", speakers.map(s => s.identity));
    })
    .on(RoomEvent.AudioPlaybackStatusChanged, () => {
      console.log("[Audio] audio playback status changed, canPlayback:", room.canPlaybackAudio);
    });
}

export function cleanupAudioHandler() {
  tracks.clear();
}

export function setVolumeByIdentity(identity: string, volume: number) {
  const track = tracks.get(identity);
  if (track) {
    track.setVolume(volume);
    console.log("[Audio] set volume for", identity, "to", volume);
  } else {
    console.warn("[Audio] no track found for identity:", identity);
  }
}
