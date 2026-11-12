import type { Meta, StoryObj } from "@storybook/react";
import { Card, CardContent, CardHeader, CardTitle } from "./card";

const meta: Meta<typeof Card> = {
	title: "UI/Card",
	component: Card,
};
export default meta;

export const Default: StoryObj = {
	render: () => (
		<Card className="w-80">
			<CardHeader>
				<CardTitle>公开域</CardTitle>
			</CardHeader>
			<CardContent>0 个公开域</CardContent>
		</Card>
	),
};
