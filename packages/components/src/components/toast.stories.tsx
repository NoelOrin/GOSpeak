import type { Meta, StoryObj } from "@storybook/react";
import { Toaster, toast } from "./toast";

const meta: Meta<typeof Toaster> = {
	title: "UI/Toast",
	component: Toaster,
};
export default meta;

export const Demo: StoryObj = {
	render: () => (
		<div className="flex flex-col gap-2">
			<Toaster richColors position="top-center" />
			<button type="button" onClick={() => toast.error("操作失败")}>
				触发 error
			</button>
			<button type="button" onClick={() => toast.success("操作成功")}>
				触发 success
			</button>
			<button type="button" onClick={() => toast.warning("你已被禁言")}>
				触发 warning
			</button>
		</div>
	),
};
