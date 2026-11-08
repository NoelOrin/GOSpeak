import type { Meta, StoryObj } from "@storybook/react";
import { Avatar, AvatarFallback, AvatarImage } from "./avatar";

const meta: Meta<typeof Avatar> = {
	title: "UI/Avatar",
	component: Avatar,
};
export default meta;

export const WithImage: StoryObj = {
	args: {},
	render: () => (
		<Avatar>
			<AvatarImage src="https://placeholder.dev/a.png" alt="alice" />
			<AvatarFallback>A</AvatarFallback>
		</Avatar>
	),
};

export const FallbackOnly: StoryObj = {
	render: () => (
		<Avatar>
			<AvatarFallback>管</AvatarFallback>
		</Avatar>
	),
};
