import type { Meta, StoryObj } from "@storybook/react";
import { Button } from "./button";

const meta: Meta<typeof Button> = {
	title: "UI/Button",
	component: Button,
};
export default meta;

export const Default: StoryObj<typeof Button> = { args: { children: "确认" } };
export const Outline: StoryObj<typeof Button> = {
	args: { children: "取消", variant: "outline" },
};
export const Ghost: StoryObj<typeof Button> = {
	args: { children: "幽灵", variant: "ghost" },
};
export const Destructive: StoryObj<typeof Button> = {
	args: { children: "踢出", variant: "destructive" },
};
export const Small: StoryObj<typeof Button> = {
	args: { children: "小按钮", size: "sm" },
};
export const Disabled: StoryObj<typeof Button> = {
	args: { children: "禁用", disabled: true },
};
