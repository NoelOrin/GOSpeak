import {
  type LocalParticipant,
  type LocalTrackPublication,
  type Participant,
  Room,
  RoomEvent,
  VideoPresets,
} from "livekit-client";
import { _joinRoom, _leaveRoom } from "./roomAction";
import AudioDeviceStore from "@/stores/audioDeviceStore";

interface UseRoomProps {
  token: string;
  url: string;
  onConnected?: () => void;
}

const _useRoom = ({ token, url, onConnected }: UseRoomProps) => {
  const s = AudioDeviceStore.state;

  const room = new Room({
    adaptiveStream: true,
    dynacast: true,
    stopLocalTrackOnUnpublish: true,
    disconnectOnPageLeave: true,
    videoCaptureDefaults: {
      resolution: VideoPresets.h720.resolution,
    },
    audioCaptureDefaults: {
      echoCancellation: s.echoCancellation,
      noiseSuppression: s.noiseSuppression,
      autoGainControl: s.autoGainControl,
      voiceIsolation: s.voiceIsolation,
      sampleRate: s.sampleRate,
      channelCount: s.stereo ? 2 : 1,
    },
    publishDefaults: {
      audioPreset: { maxBitrate: s.audioBitrate },
      dtx: s.dtx,
      red: s.red,
      forceStereo: s.stereo,
    },
    webAudioMix: true,
    reconnectPolicy: {
      nextRetryDelayInMs: () => 300,
    },
  });

  if (url) {
    room.prepareConnection(url, token);
  }

  let notified = false;
  room
    .on(RoomEvent.LocalTrackPublished, () => {
      // 本地轨道首次发布成功后触发一次，代表真正进入房间
      if (!notified) {
        notified = true;
        onConnected?.();
      }
    })
    .on(RoomEvent.ActiveSpeakersChanged, handleActiveSpeakerChange)
    .on(RoomEvent.Disconnected, () => {
      notified = false;
      handleDisconnect();
    })
    .on(RoomEvent.LocalTrackUnpublished, handleLocalTrackUnpublished);

  const joinRoom = () => _joinRoom(room, url, token);
  const leaveRoom = () => _leaveRoom(room);

  return { room, joinRoom, leaveRoom };
};

const createRoom = (props: UseRoomProps) => _useRoom(props);
export type RoomReturnType = Pick<
  ReturnType<typeof _useRoom>,
  "room" | "joinRoom" | "leaveRoom"
>;

function handleLocalTrackUnpublished(
  publication: LocalTrackPublication,
  participant: LocalParticipant,
) {
  console.log("Local track unpublished:", publication.trackSid);
  publication.track?.detach();
}

function handleActiveSpeakerChange(speakers: Participant[]) {
  console.log(
    "Active speakers changed:",
    speakers.map((s) => s.identity),
  );
}

function handleDisconnect() {
  console.log("已与房间断开连接");
}

export default createRoom;
