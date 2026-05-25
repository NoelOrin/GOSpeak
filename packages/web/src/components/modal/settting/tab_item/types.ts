import type { FieldsType } from "../../../form";

export interface SettingTabConfig {
  label: string
  fields: FieldsType
  onSubmit: (values: Record<string, any>) => void
}