import {
  type Room,
  type RemoteParticipant,
  type RemoteTrack,
  type RemoteTrackPublication,
  RoomEvent,
  Track,
} from "livekit-client";

let audioCtx: AudioContext | null = null;

/** trackSid -> { source, gain } */
const nodes = new Map<
  string,
  { source: MediaStreamAudioSourceNode; gain: GainNode }
>();

/** identity -> trackSid */
const identityToTrack = new Map<string, string>();

function getAudioContext(): AudioContext {
  if (!audioCtx) {
    audioCtx = new AudioContext();
    console.log("[Audio] AudioContext created, state:", audioCtx.state);
  }
  if (audioCtx.state === "suspended") {
    console.log("[Audio] AudioContext suspended, attempting resume");
    audioCtx.resume().then(() => {
      console.log("[Audio] AudioContext resumed, state:", audioCtx?.state);
    });
  }
  return audioCtx;
}

function onTrackSubscribed(
  track: RemoteTrack,
  publication: RemoteTrackPublication,
  participant: RemoteParticipant,
) {
  console.log(
    "[Audio] track subscribed:",
    track.kind,
    track.sid,
    "participant:",
    participant.identity,
  );
  if (track.kind !== Track.Kind.Audio) return;

  const ctx = getAudioContext();
  console.log(
    "[Audio] creating source for",
    participant.identity,
    "ctx state:",
    ctx.state,
  );

  const stream = new MediaStream([track.mediaStreamTrack]);
  const source = ctx.createMediaStreamSource(stream);
  const gain = ctx.createGain();
  gain.gain.value = 1;

  source.connect(gain);
  gain.connect(ctx.destination);
  nodes.set(track.sid, { source, gain });
  identityToTrack.set(participant.identity, track.sid);
  console.log("[Audio] audio pipeline ready for", participant.identity);
}

function onTrackUnsubscribed(
  track: RemoteTrack,
  _publication: RemoteTrackPublication,
) {
  if (track.kind !== Track.Kind.Audio) return;

  const entry = nodes.get(track.sid);
  if (entry) {
    entry.source.disconnect();
    entry.gain.disconnect();
    nodes.delete(track.sid);
  }
  for (const [id, sid] of identityToTrack) {
    if (sid === track.sid) {
      identityToTrack.delete(id);
      break;
    }
  }
}

export function setupAudioHandler(room: Room) {
  console.log("[Audio] setting up audio handler for room");
  // 预创建 AudioContext，确保后续可用
  getAudioContext();
  room
    .on(RoomEvent.TrackSubscribed, onTrackSubscribed)
    .on(RoomEvent.TrackUnsubscribed, onTrackUnsubscribed);
}

/** 在用户手势中调用，确保 AudioContext 不被浏览器阻止 */
export function resumeAudioContext() {
  const ctx = getAudioContext();
  if (ctx.state === "suspended") {
    ctx.resume().then(() => {
      console.log("[Audio] AudioContext resumed from gesture, state:", audioCtx?.state);
    });
  }
}

export function cleanupAudioHandler() {
  nodes.forEach(({ source, gain }) => {
    source.disconnect();
    gain.disconnect();
  });
  nodes.clear();
  identityToTrack.clear();
  if (audioCtx) {
    audioCtx.close();
    audioCtx = null;
  }
}

export function setVolume(trackSid: string, volume: number) {
  const entry = nodes.get(trackSid);
  if (entry) {
    entry.gain.gain.value = volume;
  }
}

export function setVolumeByIdentity(identity: string, volume: number) {
  const trackSid = identityToTrack.get(identity);
  if (trackSid) {
    setVolume(trackSid, volume);
  }
}