import { createForm } from "@tanstack/solid-form"
import { onMount } from "solid-js"
import { Form } from "../../../form"
import AudioDeviceStore from "../../../../stores/audioDeviceStore"
import type { SettingTabConfig } from "./types"

const AudioForm = () => {
  onMount(() => {
    AudioDeviceStore.fetchAudioDevices()
  })

  const form = createForm(() => ({
    defaultValues: {
      inputDevice: "default",
      outputDevice: "default",
      noiseReduction: false,
    },
    onSubmit: ({ value }) => console.log("音频设置:", value),
  }))

  const fields = () => [
    {
      name: "inputDevice",
      label: "输入设备",
      type: "select" as const,
      options: [
        { value: "default", label: "系统默认" },
        ...AudioDeviceStore.state.audioinputs.map((d) => ({
          value: d.deviceId,
          label: d.label || `麦克风 (${d.deviceId.slice(0, 8)})`,
        })),
      ],
    },
    {
      name: "outputDevice",
      label: "输出设备",
      type: "select" as const,
      options: [
        { value: "default", label: "系统默认" },
        ...AudioDeviceStore.state.audiooutputs.map((d) => ({
          value: d.deviceId,
          label: d.label || `扬声器 (${d.deviceId.slice(0, 8)})`,
        })),
      ],
    },
    {
      name: "noiseReduction",
      label: "降噪",
      type: "checkbox" as const,
    },
  ]

  return (
    <div class="p-4">
      <h3 class="mb-4 font-bold text-lg">音频</h3>
      <Form
        form={form}
        fields={fields()}
        showSubmitButton
        submitButtonText="保存"
        formClassName="grid grid-cols-2 gap-4 card"
      />
    </div>
  )
}

const audio: SettingTabConfig = {
  label: "音频",
  component: AudioForm,
}

export default audio