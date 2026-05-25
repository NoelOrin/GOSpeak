import { createForm } from "@tanstack/solid-form"
import { Form, type FormFieldConfig } from "../../../form"
import type { SettingTabConfig } from "./types"

const GeneralForm = () => {
  const form = createForm(() => ({
    defaultValues: {
      nickname: "",
      language: "",
    },
    onSubmit: ({ value }) => console.log("通用设置:", value),
  }))

  const fields: FormFieldConfig[] = [
    {
      name: "nickname",
      label: "昵称",
      type: "text",
      placeholder: "请输入昵称",
    },
    {
      name: "language",
      label: "语言",
      type: "select",
      placeholder: "请选择语言",
      options: [
        { value: "zh", label: "中文" },
        { value: "en", label: "English" },
      ],
    },
  ]

  return (
    <div class="p-4">
      <h3 class="mb-4 font-bold text-lg">通用</h3>
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

const general: SettingTabConfig = {
  label: "通用",
  component: GeneralForm,
}

export default general