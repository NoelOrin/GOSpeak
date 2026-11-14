import { describe, expect, it } from "vitest";
import {
	api,
	connectWS,
	createDomain,
	getAdminToken,
	registerUser,
} from "../helpers";

interface AnnouncementDTO {
	uuid: string;
	title: string;
	scope_type: string;
	scope_id?: string;
	is_read: boolean;
	level: string;
	content: string;
}

async function publish(
	token: string,
	body: Record<string, unknown>,
): Promise<AnnouncementDTO> {
	const res = await api<AnnouncementDTO>("/api/v1/announcement/admin/create", {
		token,
		body: {
			scope_type: "platform",
			content: "# 标题\n正文 **加粗**",
			level: "info",
			publish: true,
			...body,
		},
	});
	expect(res.code).toBe(0);
	if (!res.data) throw new Error("announcement response missing data");
	return res.data;
}

describe("announcement module", () => {
	it("publishes, lists, counts and marks read", async () => {
		const adminToken = await getAdminToken();
		const user = await registerUser("ann");
		const title = `集成公告_${Date.now()}`;
		const created = await publish(adminToken, { title });
		expect(created.uuid).toBeTruthy();

		const list = await api<{ items: AnnouncementDTO[]; total: number }>(
			"/api/v1/announcement/list",
			{ token: user.access_token, body: {} },
		);
		expect(list.code).toBe(0);
		const found = list.data?.items.find((item) => item.uuid === created.uuid);
		expect(found?.title).toBe(title);
		expect(found?.is_read).toBe(false);

		const before = await api<{ count: number }>(
			"/api/v1/announcement/unread-count",
			{ token: user.access_token, body: {} },
		);
		expect(before.data?.count).toBeGreaterThanOrEqual(1);

		const marked = await api("/api/v1/announcement/mark-read", {
			token: user.access_token,
			body: { uuid: created.uuid },
		});
		expect(marked.code).toBe(0);

		const after = await api<{ count: number }>(
			"/api/v1/announcement/unread-count",
			{ token: user.access_token, body: {} },
		);
		expect(after.data?.count).toBe(before.data?.count - 1);

		const again = await api("/api/v1/announcement/mark-read", {
			token: user.access_token,
			body: { uuid: created.uuid },
		});
		expect(again.code).toBe(0);
		const afterAgain = await api<{ count: number }>(
			"/api/v1/announcement/unread-count",
			{ token: user.access_token, body: {} },
		);
		expect(afterAgain.data?.count).toBe(after.data?.count);
	});

	it("rejects publishing from a regular user", async () => {
		const user = await registerUser("ann");
		const res = await api("/api/v1/announcement/admin/create", {
			token: user.access_token,
			body: {
				scope_type: "platform",
				title: "hack",
				content: "x",
				publish: true,
			},
		});
		expect(res.code).toBe(1013);
	});

	it("keeps drafts invisible and domain announcements scoped", async () => {
		const adminToken = await getAdminToken();
		const owner = await registerUser("ann");
		const outsider = await registerUser("ann");
		const domain = await createDomain(
			owner.access_token,
			`ann_dom_${Date.now()}`,
		);

		const draftTitle = `草稿_${Date.now()}`;
		await publish(adminToken, { title: draftTitle, publish: false });
		const ownerList = await api<{ items: AnnouncementDTO[] }>(
			"/api/v1/announcement/list",
			{ token: owner.access_token, body: {} },
		);
		expect(ownerList.data?.items.some((i) => i.title === draftTitle)).toBe(
			false,
		);

		const domainTitle = `域公告_${Date.now()}`;
		const created = await publish(owner.access_token, {
			scope_type: "domain",
			scope_id: domain.uuid,
			title: domainTitle,
		});

		const inDomain = await api<{ items: AnnouncementDTO[] }>(
			"/api/v1/announcement/list",
			{ token: owner.access_token, body: {} },
		);
		expect(inDomain.data?.items.some((i) => i.uuid === created.uuid)).toBe(
			true,
		);

		const outDomain = await api<{ items: AnnouncementDTO[] }>(
			"/api/v1/announcement/list",
			{ token: outsider.access_token, body: {} },
		);
		expect(outDomain.data?.items.some((i) => i.uuid === created.uuid)).toBe(
			false,
		);

		const deniedMark = await api("/api/v1/announcement/mark-read", {
			token: outsider.access_token,
			body: { uuid: created.uuid },
		});
		expect(deniedMark.code).toBe(3001);
	});

	it("pushes announcement:new over websocket with an object payload", async () => {
		const adminToken = await getAdminToken();
		const user = await registerUser("ann");
		const ws = await connectWS(user.access_token);
		try {
			const title = `推送_${Date.now()}`;
			const created = await publish(adminToken, { title });
			const payload = (await ws.waitForEvent(
				"announcement:new",
			)) as AnnouncementDTO;
			expect(payload.uuid).toBe(created.uuid);
			expect(payload.title).toBe(title);
			expect(typeof payload).toBe("object");
		} finally {
			ws.close();
		}
	});
});
