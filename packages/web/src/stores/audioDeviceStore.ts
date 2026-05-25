import { createStore } from "solid-js/store"
import { getAudioDevices } from "../hooks/media"

interface AudioDeviceInfo {
  deviceId: string
  label: string
}

interface AudioDeviceState {
  audioinputs: AudioDeviceInfo[]
  audiooutputs: AudioDeviceInfo[]
  loaded: boolean
}

const [audioDeviceStore, setAudioDeviceStore] = createStore<AudioDeviceState>({
  audioinputs: [],
  audiooutputs: [],
  loaded: false,
})

async function fetchAudioDevices() {
  const devices = await getAudioDevices()
  setAudioDeviceStore({
    audioinputs: devices.audioinputs.map((d) => ({
      deviceId: d.deviceId,
      label: d.label,
    })),
    audiooutputs: devices.audiooutputs.map((d) => ({
      deviceId: d.deviceId,
      label: d.label,
    })),
    loaded: true,
  })
}

const AudioDeviceStore = {
  state: audioDeviceStore,
  fetchAudioDevices,
}

export default AudioDeviceStore