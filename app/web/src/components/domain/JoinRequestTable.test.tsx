import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { JoinRequestTable } from "./JoinRequestTable";

const baseProps = {
	items: [],
	total: 0,
	counts: { pending: 0, approved: 0, rejected: 0 },
	status: "pending" as const,
	keyword: "",
	page: 1,
	pageSize: 10,
	loading: false,
	error: "",
	reviewing: 0,
	onStatusChange: () => {},
	onKeywordChange: () => {},
	onSearch: () => {},
	onPageChange: () => {},
	onReview: () => {},
};

afterEach(cleanup);

describe("JoinRequestTable", () => {
	it("shows the empty state", () => {
		render(<JoinRequestTable {...baseProps} />);
		expect(screen.getByText("暂无加入申请")).toBeTruthy();
	});

	it("renders rows with applicant names and review actions", () => {
		const onReview = vi.fn();
		render(
			<JoinRequestTable
				{...baseProps}
				total={1}
				counts={{ pending: 1, approved: 0, rejected: 0 }}
				items={[
					{
						id: 7,
						domain_uuid: "d1",
						user_uuid: "u1",
						invite_id: 1,
						status: "pending",
						reviewed_by: "",
						reviewed_at: null,
						created_at: "2026-11-10T04:00:00Z",
						updated_at: "2026-11-10T04:00:00Z",
						name: "alice",
						display_name: "爱丽丝",
					},
				]}
				onReview={onReview}
			/>,
		);
		expect(screen.getByText("爱丽丝")).toBeTruthy();
		fireEvent.click(screen.getByRole("button", { name: "通过" }));
		expect(onReview).toHaveBeenCalledWith(7, true);
		fireEvent.click(screen.getByRole("button", { name: "拒绝" }));
		expect(onReview).toHaveBeenCalledWith(7, false);
	});

	it("hides review actions for already reviewed rows", () => {
		render(
			<JoinRequestTable
				{...baseProps}
				status="approved"
				total={1}
				items={[
					{
						id: 8,
						domain_uuid: "d1",
						user_uuid: "u2",
						invite_id: 1,
						status: "approved",
						reviewed_by: "admin-1",
						reviewed_at: "2026-11-10T05:00:00Z",
						created_at: "2026-11-10T04:00:00Z",
						updated_at: "2026-11-10T05:00:00Z",
						name: "bob",
						display_name: "鲍勃",
					},
				]}
			/>,
		);
		expect(screen.queryByRole("button", { name: "通过" })).toBeNull();
		expect(screen.getByText("已通过")).toBeTruthy();
	});

	it("shows the error banner", () => {
		render(<JoinRequestTable {...baseProps} error="加载失败" />);
		expect(screen.getByRole("alert").textContent).toContain("加载失败");
	});
});
