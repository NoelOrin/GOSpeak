import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { DomainInvite } from "@/api/invite";
import InviteCodeTable from "./InviteCodeTable";

afterEach(cleanup);

function invite(overrides: Partial<DomainInvite> = {}): DomainInvite {
	return {
		id: 1,
		domain_uuid: "d1",
		label: "默认邀请码",
		code: "ABCDEFGHJKLMNPQRSTUVWXYZ23456789AB",
		expires_at: null,
		max_uses: 0,
		used_count: 0,
		disabled_at: null,
		created_by: "u1",
		created_at: "2026-11-10T00:00:00Z",
		...overrides,
	};
}

describe("InviteCodeTable", () => {
	it("renders an empty hint when there are no invites", () => {
		render(
			<InviteCodeTable
				invites={[]}
				onDisable={() => {}}
				onShowRedemptions={() => {}}
			/>,
		);
		expect(screen.getByText("还没有邀请码")).toBeTruthy();
	});

	it("renders label, usage and status for an active invite", () => {
		render(
			<InviteCodeTable
				invites={[invite({ max_uses: 10, used_count: 3 })]}
				onDisable={() => {}}
				onShowRedemptions={() => {}}
			/>,
		);
		expect(screen.getByText("默认邀请码")).toBeTruthy();
		expect(screen.getByText("3 / 10")).toBeTruthy();
		expect(screen.getByText("生效中")).toBeTruthy();
	});

	it("marks unlimited usage", () => {
		render(
			<InviteCodeTable
				invites={[invite()]}
				onDisable={() => {}}
				onShowRedemptions={() => {}}
			/>,
		);
		expect(screen.getByText("0 / 不限")).toBeTruthy();
	});

	it("shows disabled status and hides the disable action", () => {
		render(
			<InviteCodeTable
				invites={[invite({ disabled_at: "2026-11-10T01:00:00Z" })]}
				onDisable={() => {}}
				onShowRedemptions={() => {}}
			/>,
		);
		expect(screen.getByText("已禁用")).toBeTruthy();
		expect(screen.queryByText("禁用")).toBeNull();
	});

	it("calls onDisable with the invite id", () => {
		const onDisable = vi.fn();
		render(
			<InviteCodeTable
				invites={[invite({ id: 9 })]}
				onDisable={onDisable}
				onShowRedemptions={() => {}}
			/>,
		);
		screen.getByText("禁用").click();
		expect(onDisable).toHaveBeenCalledWith(9);
	});
});
