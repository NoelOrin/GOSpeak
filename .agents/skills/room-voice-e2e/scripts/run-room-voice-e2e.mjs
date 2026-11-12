#!/usr/bin/env node
/**
 * GOSpeak room voice e2e runner (Playwright).
 *
 * Suites:
 *   join | switch | rapid-switch | media | multi-user | all
 *
 * Required env:
 *   BASE_URL (default http://localhost:3000)
 *   E2E_USER / E2E_PASS
 * Optional:
 *   E2E_USER_B / E2E_PASS_B  (multi-user)
 *   E2E_SUITE                (default all)
 *   E2E_HEADLESS             (default 1)
 *   E2E_BROWSER              (auto | system | msedge | chrome | chromium | firefox | webkit)
 *                            default auto = system default browser, then fallback chain
 *   E2E_RAPID_ROUNDS         (default 3)
 *   E2E_RAPID_DELAY_MS       (default 120)
 *   E2E_ARTIFACT_DIR         (default ./artifacts)
 *   E2E_FAKE_MEDIA           (default 1) use Chromium-family fake devices when supported
 */

import fs from "node:fs/promises";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { chromium, firefox, webkit } from "playwright";
import {
  createRoom,
  ensureTwoRooms,
  env,
  leaveRoom,
  login,
  memberCount,
  rapidSwitchRooms,
  selectRoomByName,
  switchRoom,
  uniqueName,
  waitForJoined,
  grantMediaPermissions,
} from "./ui-helpers.mjs";
import {
  getMediaSnapshot,
  installMediaProbe,
  waitForMediaReady,
  waitForRemoteAudio,
} from "./media-probe.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

function parseArgs(argv) {
  const args = {
    suite: env("E2E_SUITE", "all"),
    headless: env("E2E_HEADLESS", "1") !== "0",
    baseUrl: env("BASE_URL", "http://localhost:3000"),
    user: env("E2E_USER", ""),
    pass: env("E2E_PASS", ""),
    userB: env("E2E_USER_B", ""),
    passB: env("E2E_PASS_B", ""),
    browser: env("E2E_BROWSER", "auto").toLowerCase(),
    rapidRounds: Number(env("E2E_RAPID_ROUNDS", "3")),
    rapidDelayMs: Number(env("E2E_RAPID_DELAY_MS", "120")),
    artifactDir: env(
      "E2E_ARTIFACT_DIR",
      path.join(__dirname, "artifacts", new Date().toISOString().replace(/[:.]/g, "-")),
    ),
    fakeMedia: env("E2E_FAKE_MEDIA", "1") !== "0",
  };

  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    if (a === "--suite") args.suite = argv[++i];
    else if (a === "--headed") args.headless = false;
    else if (a === "--base-url") args.baseUrl = argv[++i];
    else if (a === "--user") args.user = argv[++i];
    else if (a === "--pass") args.pass = argv[++i];
    else if (a === "--user-b") args.userB = argv[++i];
    else if (a === "--pass-b") args.passB = argv[++i];
    else if (a === "--browser") args.browser = String(argv[++i] || "auto").toLowerCase();
    else if (a === "--artifact-dir") args.artifactDir = argv[++i];
  }
  return args;
}

function nowIso() {
  return new Date().toISOString();
}

async function ensureDir(dir) {
  await fs.mkdir(dir, { recursive: true });
}

async function writeJson(file, data) {
  await fs.writeFile(file, `${JSON.stringify(data, null, 2)}\n`, "utf8");
}

function launchArgs(fakeMedia) {
  const args = [
    "--autoplay-policy=no-user-gesture-required",
    "--use-fake-ui-for-media-stream",
  ];
  if (fakeMedia) {
    args.push("--use-fake-device-for-media-stream");
  }
  return args;
}

/** Normalize free-form browser labels into Playwright engine keys. */
function normalizeBrowserKey(raw) {
  const value = String(raw || "").trim().toLowerCase();
  if (!value) return "";
  if (["auto", "system", "default"].includes(value)) return "system";
  if (["msedge", "edge", "microsoft edge", "microsoft-edge", "com.microsoft.edgemac", "com.microsoft.edge"].includes(value)) {
    return "msedge";
  }
  if (["chrome", "google chrome", "google-chrome", "com.google.chrome"].includes(value)) {
    return "chrome";
  }
  if (["chrome-beta", "google chrome beta", "com.google.chrome.beta"].includes(value)) {
    return "chrome-beta";
  }
  if (["chromium", "playwright-chromium"].includes(value)) return "chromium";
  if (["firefox", "mozilla firefox", "org.mozilla.firefox"].includes(value)) return "firefox";
  if (["webkit", "safari", "com.apple.safari"].includes(value)) return "webkit";
  if (["brave", "com.brave.browser"].includes(value)) return "chrome"; // closest chromium channel
  return value;
}

function readMacDefaultBrowserId() {
  try {
    const plist = execFileSync(
      "defaults",
      ["read", "com.apple.LaunchServices/com.apple.launchservices.secure", "LSHandlers"],
      { encoding: "utf8" },
    );
    // Prefer https handler, then http.
    const blocks = plist.split("};");
    let httpsHandler = "";
    let httpHandler = "";
    for (const block of blocks) {
      if (!block.includes("LSHandlerRoleAll") || !block.includes("LSHandlerURLScheme")) continue;
      const scheme = (block.match(/LSHandlerURLScheme\s*=\s*([a-zA-Z0-9+-]+)/) || [])[1] || "";
      const role = (block.match(/LSHandlerRoleAll\s*=\s*"?([^";\n]+)"?/) || [])[1] || "";
      if (!role) continue;
      if (scheme === "https") httpsHandler = role;
      if (scheme === "http") httpHandler = role;
    }
    return httpsHandler || httpHandler || "";
  } catch {
    return "";
  }
}

function readLinuxDefaultBrowserId() {
  try {
    const out = execFileSync("xdg-settings", ["get", "default-web-browser"], {
      encoding: "utf8",
    }).trim();
    return out || "";
  } catch {
    try {
      const out = execFileSync(
        "xdg-mime",
        ["query", "default", "x-scheme-handler/https"],
        { encoding: "utf8" },
      ).trim();
      return out || "";
    } catch {
      return "";
    }
  }
}

function readWindowsDefaultBrowserId() {
  // Best-effort: ProgId under UserChoice. Failure is fine; fallback chain continues.
  try {
    const out = execFileSync(
      "reg",
      [
        "query",
        "HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\Shell\\Associations\\UrlAssociations\\https\\UserChoice",
        "/v",
        "ProgId",
      ],
      { encoding: "utf8" },
    );
    const match = out.match(/ProgId\s+REG_SZ\s+(\S+)/i);
    return match?.[1] || "";
  } catch {
    return "";
  }
}

function detectSystemDefaultBrowserKey() {
  const platform = process.platform;
  let raw = "";
  if (platform === "darwin") raw = readMacDefaultBrowserId();
  else if (platform === "linux") raw = readLinuxDefaultBrowserId();
  else if (platform === "win32") raw = readWindowsDefaultBrowserId();

  // Desktop file / bundle id / progid → engine key
  const lower = String(raw).toLowerCase();
  if (!lower) return "";
  if (lower.includes("edge")) return "msedge";
  if (lower.includes("chrome") && lower.includes("beta")) return "chrome-beta";
  if (lower.includes("chrome") || lower.includes("chromium")) {
    return lower.includes("chromium") ? "chromium" : "chrome";
  }
  if (lower.includes("firefox") || lower.includes("mozilla")) return "firefox";
  if (lower.includes("safari")) return "webkit";
  if (lower.includes("brave")) return "chrome";
  return normalizeBrowserKey(raw);
}

/**
 * Build ordered candidate list.
 * Priority: explicit browser (if not auto) → system default → generic fallbacks.
 * Never hardcode Edge as first choice.
 */
function browserCandidates(requested) {
  const key = normalizeBrowserKey(requested || "auto");
  const systemKey = detectSystemDefaultBrowserKey();
  // Generic fallbacks are neutral (not Edge-first). System default is pushed earlier when detected.
  const generic = ["chrome", "chromium", "msedge", "firefox", "webkit"];
  const ordered = [];

  const push = (item) => {
    const n = normalizeBrowserKey(item);
    if (!n || n === "system") return;
    if (!ordered.includes(n)) ordered.push(n);
  };

  if (key && key !== "system") {
    push(key);
  } else if (systemKey) {
    push(systemKey);
  }

  // If user asked for system/auto, keep system first then fallbacks.
  // If user asked for a specific browser, still allow fallbacks after it fails.
  for (const item of generic) push(item);
  // Ensure system default appears early even when explicit request differs and fails.
  if (systemKey) {
    // already pushed first when auto; for explicit, keep as secondary preference
    if (key && key !== "system" && ordered[0] !== systemKey) {
      // insert system right after explicit
      const rest = ordered.filter((x) => x !== systemKey);
      ordered.splice(0, ordered.length, rest[0], systemKey, ...rest.slice(1));
      // de-dup
      const dedup = [];
      for (const x of ordered) if (!dedup.includes(x)) dedup.push(x);
      return { requested: key || "system", systemKey, candidates: dedup };
    }
  }
  return { requested: key || "system", systemKey, candidates: ordered };
}

function engineFor(browserKey) {
  switch (browserKey) {
    case "msedge":
    case "chrome":
    case "chrome-beta":
    case "chromium":
      return {
        type: "chromium",
        launcher: chromium,
        channel: browserKey === "chromium" ? undefined : browserKey,
        supportsFakeMediaArgs: true,
      };
    case "firefox":
      return {
        type: "firefox",
        launcher: firefox,
        channel: undefined,
        supportsFakeMediaArgs: false,
      };
    case "webkit":
      return {
        type: "webkit",
        launcher: webkit,
        channel: undefined,
        supportsFakeMediaArgs: false,
      };
    default:
      return null;
  }
}

async function launchBrowserWithFallback(args) {
  const plan = browserCandidates(args.browser);
  const errors = [];

  for (const candidate of plan.candidates) {
    const engine = engineFor(candidate);
    if (!engine) {
      errors.push({ browser: candidate, error: "unsupported browser key" });
      continue;
    }

    const launchOptions = {
      headless: args.headless,
    };
    if (engine.channel) launchOptions.channel = engine.channel;
    if (engine.supportsFakeMediaArgs) {
      launchOptions.args = launchArgs(args.fakeMedia);
    }

    try {
      const browser = await engine.launcher.launch(launchOptions);
      const info = {
        requested: plan.requested,
        systemDefault: plan.systemKey || null,
        selected: candidate,
        engine: engine.type,
        channel: engine.channel || null,
        headless: args.headless,
        fallbackUsed: candidate !== plan.candidates[0],
        attempts: errors.slice(),
      };
      console.log(
        `[browser] selected=${info.selected} engine=${info.engine}` +
          (info.channel ? ` channel=${info.channel}` : "") +
          (info.systemDefault ? ` systemDefault=${info.systemDefault}` : "") +
          (info.fallbackUsed ? " (fallback)" : ""),
      );
      return { browser, info };
    } catch (error) {
      errors.push({
        browser: candidate,
        error: String(error?.message || error),
      });
      console.warn(`[browser] launch failed for ${candidate}: ${error?.message || error}`);
    }
  }

  const detail = errors
    .map((e) => `- ${e.browser}: ${e.error}`)
    .join("\n");
  throw new Error(
    `Unable to launch any browser.\nrequested=${plan.requested}\nsystemDefault=${plan.systemKey || "unknown"}\nattempts:\n${detail}`,
  );
}

async function newContext(browser, args, label) {
  const context = await browser.newContext({
    baseURL: args.baseUrl,
    ignoreHTTPSErrors: true,
    permissions: ["microphone", "camera"],
  });
  await grantMediaPermissions(context);
  const page = await context.newPage();
  page.setDefaultTimeout(20000);
  await installMediaProbe(page);
  page._e2eLabel = label;
  return { context, page };
}

async function screenshot(page, artifactDir, name) {
  const file = path.join(artifactDir, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true }).catch(() => {});
  return file;
}

function result(name, ok, details = {}) {
  return {
    name,
    ok,
    at: nowIso(),
    ...details,
  };
}

async function suiteJoin(page, args, artifactDir) {
  const room = uniqueName("e2e-join");
  await createRoom(page, { name: room });
  await selectRoomByName(page, room);
  await waitForJoined(page, room);
  const media = await waitForMediaReady(page, 20000);
  await screenshot(page, artifactDir, "join-success");
  if (!media.ok) {
    return result("join", false, {
      room,
      reason: media.reason,
      snapshot: media.snapshot,
    });
  }
  return result("join", true, {
    room,
    memberCount: await memberCount(page),
    snapshot: media.snapshot,
  });
}

async function suiteSwitch(page, args, artifactDir) {
  const { roomA, roomB } = await ensureTwoRooms(page, "e2e-switch");
  await switchRoom(page, roomA);
  const snapA = await getMediaSnapshot(page);
  await switchRoom(page, roomB);
  const snapB = await getMediaSnapshot(page);
  await screenshot(page, artifactDir, "switch-room-b");
  const ok =
    (snapB?.currentRoomText === roomB || snapB?.hasLeaveButton) &&
    (await memberCount(page)) != null;
  return result("switch", ok, {
    roomA,
    roomB,
    snapA,
    snapB,
  });
}

async function suiteRapidSwitch(page, args, artifactDir) {
  const { roomA, roomB } = await ensureTwoRooms(page, "e2e-rapid");
  await switchRoom(page, roomA);
  const timeline = await rapidSwitchRooms(
    page,
    [roomA, roomB],
    args.rapidRounds,
    args.rapidDelayMs,
  );
  const finalSnap = await waitForMediaReady(page, 15000);
  await screenshot(page, artifactDir, "rapid-switch-final");
  const ok = timeline.every((t) => t.ok) && finalSnap.ok;
  return result("rapid-switch", ok, {
    roomA,
    roomB,
    rounds: args.rapidRounds,
    delayMs: args.rapidDelayMs,
    timeline,
    finalSnapshot: finalSnap.snapshot,
  });
}

async function suiteMedia(page, args, artifactDir) {
  const room = uniqueName("e2e-media");
  await createRoom(page, { name: room });
  await selectRoomByName(page, room);
  await waitForJoined(page, room);
  const ready = await waitForMediaReady(page, 25000);
  const snap = ready.snapshot || (await getMediaSnapshot(page));
  await screenshot(page, artifactDir, "media-publish");

  const hasLocalAudio =
    (snap?.localTracks || []).some((t) => t.kind === "audio") ||
    (snap?.getUserMediaCalls || 0) > 0;
  const hasPc = (snap?.peerConnections || []).length > 0;
  const publishOk = ready.ok && hasLocalAudio && hasPc;

  // single-user cannot fully prove remote pull; mark pull as self-check best-effort
  const pullSelf = {
    remoteAudioCount: (snap?.remoteAudio || []).length,
    note: "single-user media suite only proves publish/session; use multi-user for pull",
  };

  return result("media", publishOk, {
    room,
    publishOk,
    pullSelf,
    snapshot: snap,
  });
}

async function suiteMultiUser(browser, args, artifactDir) {
  if (!args.userB || !args.passB) {
    return result("multi-user", false, {
      skipped: false,
      reason: "E2E_USER_B / E2E_PASS_B required",
    });
  }

  const a = await newContext(browser, args, "user-a");
  const b = await newContext(browser, args, "user-b");
  try {
    await login(a.page, { username: args.user, password: args.pass });
    await login(b.page, { username: args.userB, password: args.passB });

    const room = uniqueName("e2e-multi");
    await createRoom(a.page, { name: room });
    await selectRoomByName(a.page, room);
    await waitForJoined(a.page, room);
    const aReady = await waitForMediaReady(a.page, 25000);
    if (!aReady.ok) {
      await screenshot(a.page, artifactDir, "multi-a-failed");
      return result("multi-user", false, {
        room,
        phase: "user-a-join",
        snapshot: aReady.snapshot,
        reason: aReady.reason,
      });
    }

    await selectRoomByName(b.page, room);
    await waitForJoined(b.page, room);
    const bReady = await waitForMediaReady(b.page, 25000);
    if (!bReady.ok) {
      await screenshot(b.page, artifactDir, "multi-b-failed");
      return result("multi-user", false, {
        room,
        phase: "user-b-join",
        snapshot: bReady.snapshot,
        reason: bReady.reason,
      });
    }

    // each should eventually receive the other side's remote audio element/track
    const aRemote = await waitForRemoteAudio(a.page, 1, 25000);
    const bRemote = await waitForRemoteAudio(b.page, 1, 25000);
    await screenshot(a.page, artifactDir, "multi-a-remote");
    await screenshot(b.page, artifactDir, "multi-b-remote");

    const countA = await memberCount(a.page);
    const countB = await memberCount(b.page);
    const ok =
      aRemote.ok &&
      bRemote.ok &&
      (countA == null || countA >= 2) &&
      (countB == null || countB >= 2);

    return result("multi-user", ok, {
      room,
      memberCountA: countA,
      memberCountB: countB,
      aRemote,
      bRemote,
      aSnapshot: await getMediaSnapshot(a.page),
      bSnapshot: await getMediaSnapshot(b.page),
    });
  } finally {
    await a.context.close().catch(() => {});
    await b.context.close().catch(() => {});
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (!args.user || !args.pass) {
    console.error("E2E_USER and E2E_PASS are required");
    process.exit(2);
  }

  await ensureDir(args.artifactDir);
  const report = {
    startedAt: nowIso(),
    baseUrl: args.baseUrl,
    suite: args.suite,
    results: [],
    artifactDir: args.artifactDir,
  };

  let browser;
  let browserInfo;
  try {
    ({ browser, info: browserInfo } = await launchBrowserWithFallback(args));
  } catch (error) {
    report.finishedAt = nowIso();
    report.ok = false;
    report.failed = 1;
    report.passed = 0;
    report.browser = { error: String(error?.message || error) };
    report.results.push(
      result("browser-launch", false, { error: String(error?.message || error) }),
    );
    const jsonPath = path.join(args.artifactDir, "report.json");
    const mdPath = path.join(args.artifactDir, "report.md");
    await writeJson(jsonPath, report);
    await fs.writeFile(mdPath, renderMarkdown(report), "utf8");
    console.error(error?.message || error);
    process.exit(1);
  }
  report.browser = browserInfo;

  try {
    const suites =
      args.suite === "all"
        ? ["join", "switch", "rapid-switch", "media", "multi-user"]
        : [args.suite];

    for (const suite of suites) {
      process.stdout.write(`\n==> running suite: ${suite}\n`);
      try {
        if (suite === "multi-user") {
          const r = await suiteMultiUser(browser, args, args.artifactDir);
          report.results.push(r);
          console.log(r.ok ? `PASS ${suite}` : `FAIL ${suite}`, r.reason || "");
          continue;
        }

        const { context, page } = await newContext(browser, args, suite);
        try {
          await login(page, { username: args.user, password: args.pass });
          let r;
          if (suite === "join") r = await suiteJoin(page, args, args.artifactDir);
          else if (suite === "switch") r = await suiteSwitch(page, args, args.artifactDir);
          else if (suite === "rapid-switch")
            r = await suiteRapidSwitch(page, args, args.artifactDir);
          else if (suite === "media") r = await suiteMedia(page, args, args.artifactDir);
          else throw new Error(`Unknown suite: ${suite}`);
          report.results.push(r);
          console.log(r.ok ? `PASS ${suite}` : `FAIL ${suite}`);
          if (!r.ok) {
            await screenshot(page, args.artifactDir, `${suite}-failure`);
          }
        } finally {
          await context.close().catch(() => {});
        }
      } catch (error) {
        const r = result(suite, false, {
          error: String(error?.message || error),
          stack: error?.stack,
          timeline: error?.timeline,
        });
        report.results.push(r);
        console.error(`FAIL ${suite}:`, error?.message || error);
      }
    }
  } finally {
    await browser.close().catch(() => {});
  }

  report.finishedAt = nowIso();
  report.passed = report.results.filter((r) => r.ok).length;
  report.failed = report.results.filter((r) => !r.ok).length;
  report.ok = report.failed === 0;

  const jsonPath = path.join(args.artifactDir, "report.json");
  const mdPath = path.join(args.artifactDir, "report.md");
  await writeJson(jsonPath, report);
  await fs.writeFile(mdPath, renderMarkdown(report), "utf8");

  console.log("\n==== summary ====");
  console.log(`passed=${report.passed} failed=${report.failed}`);
  console.log(`report: ${mdPath}`);

  process.exit(report.ok ? 0 : 1);
}

function renderMarkdown(report) {
  const browser = report.browser || {};
  const browserLine = browser.selected
    ? `**Browser**: ${browser.selected}` +
      (browser.engine ? ` (${browser.engine})` : "") +
      (browser.systemDefault ? ` · systemDefault=${browser.systemDefault}` : "") +
      (browser.fallbackUsed ? " · fallback" : "")
    : browser.error
      ? `**Browser**: launch failed — ${browser.error}`
      : "**Browser**: n/a";
  const lines = [
    "# Room Voice E2E Report",
    "",
    `**Started**: ${report.startedAt}`,
    `**Finished**: ${report.finishedAt}`,
    `**Base URL**: ${report.baseUrl}`,
    `**Suite**: ${report.suite}`,
    browserLine,
    `**Result**: ${report.ok ? "PASS" : "FAIL"} (${report.passed} passed / ${report.failed} failed)`,
    "",
    "## Results",
    "",
    "| Suite | Status | Notes |",
    "|---|---|---|",
  ];
  for (const r of report.results) {
    const note =
      r.reason ||
      r.error ||
      r.room ||
      (r.roomA && r.roomB ? `${r.roomA} -> ${r.roomB}` : "") ||
      "";
    lines.push(`| ${r.name} | ${r.ok ? "PASS" : "FAIL"} | ${String(note).replace(/\|/g, "/")} |`);
  }
  lines.push("", "## Artifacts", "", `- JSON: \`report.json\``, `- Dir: \`${report.artifactDir}\``);
  lines.push("");
  return lines.join("\n");
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
