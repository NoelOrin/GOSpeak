/**
 * UI helpers for GOSpeak room voice e2e.
 * Prefer role/text selectors because the app currently has few data-testid hooks.
 */

export function env(name, fallback = "") {
  const value = process.env[name];
  return value == null || value === "" ? fallback : value;
}

export function requiredEnv(name) {
  const value = env(name);
  if (!value) {
    throw new Error(`Missing required env: ${name}`);
  }
  return value;
}

export function uniqueName(prefix) {
  const stamp = new Date().toISOString().replace(/[-:.TZ]/g, "").slice(0, 14);
  return `${prefix}-${stamp}-${Math.floor(Math.random() * 1000)}`;
}

export async function grantMediaPermissions(context) {
  await context.grantPermissions(["microphone", "camera"], {
    origin: env("BASE_URL", "http://localhost:3000"),
  });
}

export async function firstDomainPath(page) {
  return page.evaluate(async () => {
    const token = localStorage.getItem("accessToken");
    const headers = token ? { Authorization: `Bearer ${token}` } : {};
    const res = await fetch("/api/v1/domain/my-domains", {
      method: "POST",
      headers: { ...headers, "Content-Type": "application/json" },
      body: "{}",
    });
    if (!res.ok) return "";
    const body = await res.json();
    const domains = body?.data;
    const uuid = Array.isArray(domains)
      ? domains[0]?.uuid
      : body?.data?.domain_uuids?.[0];
    return uuid ? `/domain/${uuid}` : "";
  });
}

export async function login(page, { username, password, goDomain = true }) {
  await page.goto("/login", { waitUntil: "domcontentloaded" });
  await page.getByPlaceholder("请输入用户名").fill(username);
  await page.getByPlaceholder("请输入密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();

  // admin first-login force password change may appear; fail fast with clear message.
  const forceChange = page.getByText("修改密码").or(page.getByText("首次登录"));
  if (await forceChange.first().isVisible({ timeout: 1500 }).catch(() => false)) {
    throw new Error(
      `Account ${username} requires first password change. Use a non-default account for e2e.`,
    );
  }

  await page.waitForURL((url) => !url.pathname.startsWith("/login"), {
    timeout: 20000,
  });
  await page.waitForLoadState("networkidle").catch(() => {});

  if (goDomain) {
    const domainPath = await firstDomainPath(page);
    if (domainPath) {
      await page.goto(domainPath, { waitUntil: "domcontentloaded" });
      // room list header
      await page
        .getByText(/语音域|刷新/)
        .first()
        .waitFor({ state: "visible", timeout: 15000 });
    } else {
      await page.goto("/discover", { waitUntil: "domcontentloaded" });
      await page
        .getByText(/发现域|快捷进入已加入的语音域/)
        .first()
        .waitFor({ state: "visible", timeout: 15000 })
        .catch(() => {});
    }
    // socket connected eventually (room list or empty state)
    await page
      .getByText(/暂无房间|刷新/)
      .first()
      .waitFor({ state: "visible", timeout: 15000 })
      .catch(() => {});
  }
}

export async function openCreateRoomModal(page) {
  // Domain sidebar uses icon button title="新建房间"; dashboard uses "快速创建房间".
  const createBtn = page
    .locator('button[title="新建房间"]')
    .or(page.getByRole("button", { name: /快速创建房间|创建房间|新建房间/ }))
    .or(page.locator('button:has-text("快速创建房间")'))
    .or(page.locator('button[title*="创建"]'));
  await createBtn.first().click({ timeout: 10000 });
	await page.getByRole("heading", { name: "新建房间" }).waitFor({
		state: "visible",
		timeout: 10000,
	});
	await page.locator("#field-name").first().waitFor({
		state: "visible",
		timeout: 10000,
	});
}

export async function createRoom(page, { name, password = "", limit = "12", joinAfterCreate = false }) {
  await openCreateRoomModal(page);

  // Actual labels from createRoomModal.tsx
  const nameInput = page.locator("#field-name").first();
  await nameInput.fill(name);

  if (password) {
    const passwordInput = page.locator("#field-password").first();
    if (await passwordInput.isVisible().catch(() => false)) {
      await passwordInput.fill(password);
    }
  }

  const limitInput = page.locator("#field-limit").first();
  if (await limitInput.isVisible().catch(() => false)) {
    await limitInput.fill(String(limit));
  }

  // Default modal joins after create; for suite isolation prefer staying in list mode.
  if (!joinAfterCreate) {
    const joinToggle = page.locator("#field-joinAfterCreate").first();
    if (await joinToggle.isVisible().catch(() => false)) {
      const checked = await joinToggle.isChecked().catch(() => true);
      if (checked) await joinToggle.click();
    }
  }

	await page.getByRole("button", { name: "创建房间" }).click();

	// 创建完成以 toast/列表文本出现为准；按钮可见性由进房步骤处理。
	await page
		.getByText(new RegExp(`已创建房间:\\s*${name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`))
		.first()
		.waitFor({ state: "attached", timeout: 15000 });

	if (!page.url().includes("/domain")) {
		const domainPath = await firstDomainPath(page);
		if (domainPath) {
			await page.goto(domainPath, { waitUntil: "domcontentloaded" });
		}
	}
	await page.waitForTimeout(1200);
	return name;
}

export async function selectRoomByName(page, roomName) {
	// Room list item title is a span with room name; double-click the row container.
	// Room list rows are buttons; use a visible button to avoid matching hidden duplicate text.
	const roomRow = page.locator("button").filter({ hasText: roomName, visible: true }).first();
	await roomRow.waitFor({ state: "visible", timeout: 15000 });
	await roomRow.dblclick();
}

export async function waitForJoined(page, roomName, timeoutMs = 25000) {
  const leaveBtn = page.getByRole("button", { name: "离开" });
  await leaveBtn.waitFor({ state: "visible", timeout: timeoutMs });

  if (roomName) {
    await page
      .locator(".font-bold.truncate")
      .filter({ hasText: roomName })
      .first()
      .waitFor({ state: "visible", timeout: timeoutMs })
      .catch(async () => {
        // fallback: any text match in header area
        await page.getByText(roomName, { exact: true }).first().waitFor({
          state: "visible",
          timeout: 3000,
        });
      });
  }

  // Ensure not stuck in failed state.
  if (await page.getByRole("button", { name: "重试" }).isVisible().catch(() => false)) {
    const errText = await page.locator(".text-error\\/70, .text-error").first().textContent().catch(() => "");
    throw new Error(`Join failed for room ${roomName || "?"}: ${errText || "retry visible"}`);
  }
}

export async function leaveRoom(page) {
  const leaveBtn = page.getByRole("button", { name: "离开" });
  if (await leaveBtn.isVisible().catch(() => false)) {
    await leaveBtn.click();
    await leaveBtn.waitFor({ state: "hidden", timeout: 15000 }).catch(() => {});
  }
}

export async function switchRoom(page, roomName) {
  await selectRoomByName(page, roomName);
  await waitForJoined(page, roomName);
}

export async function rapidSwitchRooms(page, roomNames, rounds = 3, delayMs = 150) {
  const timeline = [];
  for (let i = 0; i < rounds; i += 1) {
    for (const roomName of roomNames) {
      const started = Date.now();
      await selectRoomByName(page, roomName);
      try {
        await waitForJoined(page, roomName, 20000);
        timeline.push({
          room: roomName,
          round: i + 1,
          ok: true,
          ms: Date.now() - started,
        });
      } catch (error) {
        timeline.push({
          room: roomName,
          round: i + 1,
          ok: false,
          ms: Date.now() - started,
          error: String(error?.message || error),
        });
        throw Object.assign(error, { timeline });
      }
      if (delayMs > 0) {
        await page.waitForTimeout(delayMs);
      }
    }
  }
  return timeline;
}

export async function ensureTwoRooms(page, prefix = "e2e-room") {
  const roomA = uniqueName(`${prefix}-a`);
  const roomB = uniqueName(`${prefix}-b`);
  await createRoom(page, { name: roomA });
  await createRoom(page, { name: roomB });
  return { roomA, roomB };
}

export async function memberCount(page) {
  const text = await page
    .locator("span")
    .filter({ hasText: /\d+\s*人在线/ })
    .first()
    .textContent()
    .catch(() => "");
  const match = String(text || "").match(/(\d+)\s*人在线/);
  return match ? Number(match[1]) : null;
}
