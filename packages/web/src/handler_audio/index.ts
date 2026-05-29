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
  _publication: RemoteTrackPublication,
  participant: RemoteParticipant,
) {
  if (track.kind !== Track.Kind.Audio) return;
  console.log("[Audio] track subscribed:", track.sid, participant.identity);
  tracks.set(participant.identity, track as RemoteAudioTrack);
}

function onTrackUnsubscribed(
  track: RemoteTrack,
  _publication: RemoteTrackPublication,
  participant: RemoteParticipant,
) {
  if (track.kind !== Track.Kind.Audio) return;
  tracks.delete(participant.identity);
}

export function setupAudioHandler(room: Room) {
  console.log("[Audio] setting up audio handler for room");
  room
    .on(RoomEvent.TrackSubscribed, onTrackSubscribed)
    .on(RoomEvent.TrackUnsubscribed, onTrackUnsubscribed);
}

export function cleanupAudioHandler() {
  tracks.clear();
}

export function setVolumeByIdentity(identity: string, volume: number) {
  const track = tracks.get(identity);
  if (track) {
    track.setVolume(volume);
  }
}
