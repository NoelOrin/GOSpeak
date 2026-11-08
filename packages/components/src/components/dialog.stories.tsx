import type { Meta, StoryObj } from "@storybook/react";
import { Button } from "./button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "./dialog";

const meta: Meta<typeof Dialog> = {
	title: "UI/Dialog",
	component: Dialog,
};
export default meta;

export const Open: StoryObj = {
	render: () => (
		<Dialog open>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>确认操作</DialogTitle>
					<DialogDescription>此操作不可撤销，确定继续吗？</DialogDescription>
				</DialogHeader>
				<div className="flex justify-end gap-2">
					<Button variant="outline">取消</Button>
					<Button>确认</Button>
				</div>
			</DialogContent>
		</Dialog>
	),
};

export const WithTrigger: StoryObj = {
	render: () => (
		<Dialog>
			<DialogTrigger asChild>
				<Button>打开弹窗</Button>
			</DialogTrigger>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>标题</DialogTitle>
				</DialogHeader>
			</DialogContent>
		</Dialog>
	),
};
