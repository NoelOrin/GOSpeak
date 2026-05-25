import { createForm } from "@tanstack/solid-form"
import { Form, type FormFieldConfig } from "../../../form"
import type { SettingTabConfig } from "./types"

const RoomForm = () => {
  const form = createForm(() => ({
    defaultValues: {
      roomName: "",
      roomPassword: "",
      maxMembers: "",
    },
    onSubmit: ({ value }) => console.log("房间设置:", value),
  }))

  const fields: FormFieldConfig[] = [
    {
      name: "roomName",
      label: "房间名称",
      type: "text",
      placeholder: "请输入房间名称",
    },
    {
      name: "roomPassword",
      label: "房间密码",
      type: "password",
      placeholder: "请输入房间密码",
    },
    {
      name: "maxMembers",
      label: "最大人数",
      type: "number",
      placeholder: "请输入最大人数",
    },
  ]

  return (
    <div class="p-4">
      <h3 class="mb-4 font-bold text-lg">房间</h3>
      <Form
        form={form}
        fields={fields}
        showSubmitButton
        submitButtonText="保存"
        formClassName="grid grid-cols-2 gap-4 card"
      />
    </div>
  )
}

const room: SettingTabConfig = {
  label: "房间",
  component: RoomForm,
}

export default room