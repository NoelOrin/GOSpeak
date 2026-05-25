import general from "./general"
import room from "./room"
import audio from "./audio"
import type { SettingTabConfig } from "./types"

const TABS: SettingTabConfig[] = [general, room, audio]

export default TABS
export type { SettingTabConfig }