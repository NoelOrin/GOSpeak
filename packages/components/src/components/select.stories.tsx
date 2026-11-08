import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "./select";

const meta: Meta<typeof Select> = {
	title: "Components/Select",
	component: Select,
};
export default meta;

const ROLES = ["user", "admin", "ban"];
const USER_IDS = Array.from({ length: 30 }, (_, i) => `user-${i + 1}`);

export const Default: StoryObj = {
	render: () => (
		<div className="w-64">
			<Select defaultValue="user">
				<SelectTrigger>
					<SelectValue placeholder="请选择" />
				</SelectTrigger>
				<SelectContent>
					{ROLES.map((r) => (
						<SelectItem key={r} value={r}>
							{r}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</div>
	),
};

/** 弹层展开态：用于核验位置、宽度是否跟 trigger 对齐。 */
export const Open: StoryObj = {
	render: () => (
		<div className="w-64">
			<Select defaultOpen defaultValue="user">
				<SelectTrigger>
					<SelectValue />
				</SelectTrigger>
				<SelectContent>
					{ROLES.map((r) => (
						<SelectItem key={r} value={r}>
							{r}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</div>
	),
};

/** 表格内窄 trigger（项目里角色切换的实际形态）。 */
export const InTableCompact: StoryObj = {
	render: function Render() {
		const [role, setRole] = useState("admin");
		return (
			<table className="w-[420px] text-sm">
				<thead>
					<tr className="text-left">
						<th className="px-3 py-2">成员</th>
						<th className="px-3 py-2">角色</th>
						<th className="px-3 py-2">操作</th>
					</tr>
				</thead>
				<tbody>
					<tr className="border-t border-border/60">
						<td className="px-3 py-2">张三</td>
						<td className="px-3 py-2">
							<Select value={role} onValueChange={setRole}>
								<SelectTrigger className="h-8 w-auto gap-1 px-2 text-xs">
									<SelectValue placeholder="未分配" />
								</SelectTrigger>
								<SelectContent>
									{ROLES.map((r) => (
										<SelectItem key={r} value={r}>
											{r}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</td>
						<td className="px-3 py-2 text-xs text-faint-foreground">踢出</td>
					</tr>
				</tbody>
			</table>
		);
	},
};

/** 长列表：核验滚动与限高。 */
export const LongList: StoryObj = {
	render: () => (
		<div className="w-72">
			<Select defaultOpen>
				<SelectTrigger>
					<SelectValue placeholder="选择用户" />
				</SelectTrigger>
				<SelectContent>
					{USER_IDS.map((id, i) => (
						<SelectItem key={id} value={id}>
							用户 {i + 1} (user_{i + 1}) 一个很长很长很长的显示名
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</div>
	),
};

/** 空值与禁用。 */
export const PlaceholderAndDisabled: StoryObj = {
	render: () => (
		<div className="flex w-64 flex-col gap-3">
			<Select>
				<SelectTrigger>
					<SelectValue placeholder="未选择（显示 placeholder）" />
				</SelectTrigger>
				<SelectContent>
					<SelectItem value="a">选项 A</SelectItem>
				</SelectContent>
			</Select>
			<Select disabled>
				<SelectTrigger>
					<SelectValue placeholder="禁用态" />
				</SelectTrigger>
				<SelectContent>
					<SelectItem value="a">选项 A</SelectItem>
				</SelectContent>
			</Select>
		</div>
	),
};
