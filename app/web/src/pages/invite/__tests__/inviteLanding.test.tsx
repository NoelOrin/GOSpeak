import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// vi.mock 工厂会被提升到文件顶部，因此所有被工厂引用的状态必须用 vi.hoisted 声明。
const h = vi.hoisted(() => ({
	previewInvite: vi.fn(),
	getJoinRequest: vi.fn(),
	joinByCode: vi.fn(),
	registerViaInvite: vi.fn(),
	cancelJoinRequest: vi.fn(),
	navigate: vi.fn(),
	domainStoreState: {
		myDomainUUIDs: [] as string[],
		addDomain: vi.fn(),
		setCurrentDomain: vi.fn(),
		loadMembers: vi.fn(async () => {}),
		loadMyDomains: vi.fn(async () => {}),
	},
	userState: {
		user: null as null | { uuid: string },
		ensureSession: vi.fn(async () => true),
	},
}));

const { previewInvite, getJoinRequest, domainStoreState, userState } = h;

vi.mock("@/api/inviteJoin", () => ({
	previewInvite: (...a: unknown[]) => h.previewInvite(...a),
	getJoinRequest: (...a: unknown[]) => h.getJoinRequest(...a),
	joinByCode: (...a: unknown[]) => h.joinByCode(...a),
	registerViaInvite: (...a: unknown[]) => h.registerViaInvite(...a),
	cancelJoinRequest: (...a: unknown[]) => h.cancelJoinRequest(...a),
}));

vi.mock("@tanstack/react-router", () => ({
	createFileRoute: () => (opts: { component: () => JSX.Element }) => ({
		component: opts.component,
		useParams: () => ({ code: "CODE" }),
	}),
	Link: ({ children }: { children: React.ReactNode }) => (
		<span>{children}</span>
	),
	useNavigate: () => h.navigate,
}));

vi.mock("@/stores/domainStore", () => ({
	domainStore: h.domainStoreState,
	useDomainStore: (selector: (s: typeof h.domainStoreState) => unknown) =>
		selector(h.domainStoreState),
}));

vi.mock("@/stores/userStore", () => ({
	useUserStore: Object.assign(
		(selector: (s: typeof h.userState) => unknown) => selector(h.userState),
		{ getState: () => h.userState },
	),
}));

import { InviteLandingPage } from "../d/$code";

const previewPayload = {
	domain: { uuid: "d1", name: "Realm", icon_url: "", description: "desc" },
	requires_approval: false,
	remaining_uses: -1,
};

afterEach(cleanup);

describe("invite landing page", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		userState.user = null;
		domainStoreState.myDomainUUIDs = [];
		previewInvite.mockResolvedValue(previewPayload);
	});

	it("shows the inline register form for anonymous visitors", async () => {
		render(<InviteLandingPage />);
		await waitFor(() =>
			expect(screen.getByRole("button", { name: "注册并加入" })).toBeTruthy(),
		);
	});

	it("shows the join button for a logged-in non-member", async () => {
		userState.user = { uuid: "u1" };
		getJoinRequest.mockResolvedValue({
			request: null,
			status: "",
			is_member: false,
		});
		render(<InviteLandingPage />);
		await waitFor(() =>
			expect(screen.getByRole("button", { name: "加入" })).toBeTruthy(),
		);
	});

	it("shows the enter-domain button for an existing member", async () => {
		userState.user = { uuid: "u1" };
		domainStoreState.myDomainUUIDs = ["d1"];
		getJoinRequest.mockResolvedValue({
			request: null,
			status: "",
			is_member: true,
		});
		render(<InviteLandingPage />);
		await waitFor(() =>
			expect(screen.getByRole("button", { name: "进入域" })).toBeTruthy(),
		);
	});

	it("shows the pending status card when a request is awaiting review", async () => {
		userState.user = { uuid: "u1" };
		getJoinRequest.mockResolvedValue({
			request: { id: 1, status: "pending", created_at: "2026-11-10T04:00:00Z" },
			status: "pending",
			is_member: false,
		});
		render(<InviteLandingPage />);
		await waitFor(() => expect(screen.getByText("申请审核中")).toBeTruthy());
	});

	it("shows the rejected card with a reapply action", async () => {
		userState.user = { uuid: "u1" };
		getJoinRequest.mockResolvedValue({
			request: {
				id: 1,
				status: "rejected",
				created_at: "2026-11-10T04:00:00Z",
			},
			status: "rejected",
			is_member: false,
		});
		render(<InviteLandingPage />);
		await waitFor(() => expect(screen.getByText("申请未通过")).toBeTruthy());
	});

	it("shows a single error card without domain info when the code is invalid", async () => {
		// beforeEach 里已 mockResolvedValue，这里必须覆盖为 reject 才能进入错误分支。
		previewInvite.mockReset();
		previewInvite.mockRejectedValue(new Error("invalid"));
		render(<InviteLandingPage />);
		await waitFor(() =>
			expect(screen.getByText("邀请链接无效、已过期或已停用")).toBeTruthy(),
		);
		expect(screen.queryByText("Realm")).toBeNull();
	});
});
