import { type AudioDeviceInfo } from "../../../../hooks/media"
import type { FieldsType } from "../../../form"

export function buildAudioFields(devices: {
  audioinputs: AudioDeviceInfo[]
  audiooutputs: AudioDeviceInfo[]
}): FieldsType {
  return [
    {
      name: "inputDevice",
      label: "输入设备",
      type: "select",
      options: [
        { value: "default", label: "系统默认" },
        ...devices.audioinputs.map((d) => ({
          value: d.deviceId,
          label: d.label || `麦克风 (${d.deviceId.slice(0, 8)})`,
        })),
      ],
    },
    {
      name: "outputDevice",
      label: "输出设备",
      type: "select",
      options: [
        { value: "default", label: "系统默认" },
        ...devices.audiooutputs.map((d) => ({
          value: d.deviceId,
          label: d.label || `扬声器 (${d.deviceId.slice(0, 8)})`,
        })),
      ],
    },
    {
      name: "noiseReduction",
      label: "降噪",
      type: "checkbox",
    },
  ]
}

export function getDefaultDeviceValues(devices: {
  audioinputs: AudioDeviceInfo[]
  audiooutputs: AudioDeviceInfo[]
}): Record<string, string | boolean> {
  return {
    inputDevice: devices.audioinputs[0]?.deviceId || "default",
    outputDevice: devices.audiooutputs[0]?.deviceId || "default",
    noiseReduction: false,
  }
}

const audio: { label: string; fields: FieldsType; onSubmit: (value: Record<string, any>) => void } = {
  label: "音频",
  fields: [
    { name: "inputDevice", label: "输入设备", type: "select", options: [] },
    { name: "outputDevice", label: "输出设备", type: "select", options: [] },
    { name: "noiseReduction", label: "降噪", type: "checkbox" },
  ],
  onSubmit: (value: Record<string, any>) => console.log("音频设置:", value),
}

export default audio