import type { SettingTabConfig } from "./types"

const room: SettingTabConfig = {
  label: "房间",
  fields: [
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
  ],
  onSubmit: (value) => console.log("房间设置:", value),
}

export default room