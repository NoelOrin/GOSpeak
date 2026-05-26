import to from "await-to-js";
import { get, set } from "idb-keyval";
import { debounce } from "lodash-es";
import { createEffect, on } from "solid-js";
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

const initialState: AudioDeviceState = {
  audioinputs: [],
  audiooutputs: [],
  loaded: false,
}

const loadPersistedState = async (): Promise<AudioDeviceState> => {
  const [err, savedState] = await to(get<AudioDeviceState>("audioDeviceStore"))
  if (!err) {
    return savedState ? { ...initialState, ...savedState } : initialState
  }
  throw err
}

const [audioDeviceStore, setAudioDeviceStore] = createStore<AudioDeviceState>(
  await loadPersistedState(),
)

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

const debouncedPersist = debounce((state: AudioDeviceState) => {
  try {
    const cleanState = JSON.parse(JSON.stringify(state))
    set("audioDeviceStore", cleanState)
  } catch (error) {
    console.error("Failed to serialize audio device state:", error)
  }
}, 200)

createEffect(
  on(
    () => [audioDeviceStore.audioinputs, audioDeviceStore.audiooutputs],
    () => {
      debouncedPersist(audioDeviceStore)
    },
  ),
)

const AudioDeviceStore = {
  state: audioDeviceStore,
  fetchAudioDevices,
}

export default AudioDeviceStore