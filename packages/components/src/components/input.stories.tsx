import type { Meta, StoryObj } from "@storybook/react";
import { Input } from "./input";

const meta: Meta<typeof Input> = {
	title: "UI/Input",
	component: Input,
};
export default meta;

export const Default: StoryObj = { args: { placeholder: "请输入用户名" } };
export const Password: StoryObj = {
	args: { type: "password", placeholder: "请输入密码" },
};
export const Disabled: StoryObj = {
	args: { placeholder: "禁用", disabled: true },
};
