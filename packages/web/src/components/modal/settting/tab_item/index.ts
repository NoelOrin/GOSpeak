import general from "./general.tsx"
import room from "./room.tsx"
import audio from "./audio.tsx"
import type { SettingTabConfig } from "./types"

const TABS: SettingTabConfig[] = [general, room, audio]

export default TABS
export type { SettingTabConfig }