import type { SettingTabConfig } from "./types"

const general: SettingTabConfig = {
  label: "通用",
  fields: [
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
  ],
  onSubmit: (value) => console.log("通用设置:", value),
}

export default general