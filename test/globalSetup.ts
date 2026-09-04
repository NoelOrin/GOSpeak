import { execFileSync, spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { appendFileSync, existsSync, mkdirSync, mkdtempSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import net from "node:net";

const setupDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(setupDir, "..");
const serverDir = join(repoRoot, "app/server");
const runtimeDir = join(setupDir, ".runtime");
const serverInfoPath = join(runtimeDir, "server.json");

interface ServerState {
  child: ChildProcessWithoutNullStreams;
  dir: string;
  port: number;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolvePromise) => setTimeout(resolvePromise, ms));
}

function newestSourceMtime(dir: string, ignored = new Set<string>()): number {
  let latest = 0;
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (ignored.has(entry.name)) {
      continue;
    }
    const full = join(dir, entry.name);
    if (entry.isDirectory()) {
      latest = Math.max(latest, newestSourceMtime(full, ignored));
    } else if (entry.name.endsWith(".go") || entry.name === "go.mod" || entry.name === "go.sum") {
      latest = Math.max(latest, statSync(full).mtimeMs);
    }
  }
  return latest;
}

function getFreePort(): Promise<number> {
  return new Promise((resolvePromise, rejectPromise) => {
    const server = net.createServer();
    server.once("error", rejectPromise);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (address && typeof address === "object") {
        const port = address.port;
        server.close(() => resolvePromise(port));
      } else {
        server.close(() => rejectPromise(new Error("failed to allocate free port")));
      }
    });
  });
}

async function waitForServer(state: ServerState): Promise<void> {
  const url = `http://127.0.0.1:${state.port}`;
  const deadline = Date.now() + 90_000;
  while (Date.now() < deadline) {
    if (state.child.exitCode !== null) {
      throw new Error(`integration server exited early: ${state.child.exitCode}`);
    }
    try {
      const response = await fetch(`${url}/readyz`);
      if (response.ok) {
        const body = (await response.json()) as { status?: string };
        if (body.status === "ok") {
          writeFileSync(serverInfoPath, JSON.stringify({ url }, null, 2));
          return;
        }
      }
    } catch {
      // server is still starting
    }
    await sleep(500);
  }
  throw new Error(`integration server did not become ready within 90s; log: ${state.dir}/server.log`);
}

export default async function setup(): Promise<() => Promise<void>> {
  mkdirSync(runtimeDir, { recursive: true });

  const externalUrl = process.env.GOSPEAK_TEST_URL;
  if (externalUrl) {
    writeFileSync(serverInfoPath, JSON.stringify({ url: externalUrl.replace(/\/$/, "") }, null, 2));
    return async () => rmSync(runtimeDir, { recursive: true, force: true });
  }

  const bin = process.env.GOSPEAK_SERVER_BIN || join(runtimeDir, `gospeak-${process.platform}-${process.arch}`);
  const binaryMtime = existsSync(bin) ? statSync(bin).mtimeMs : 0;
  const sourceMtime = newestSourceMtime(serverDir, new Set(["tmp", "vendor", "docs", "internal/webui/dist"]));
  if (binaryMtime === 0 || sourceMtime > binaryMtime) {
    console.log("[integration] building isolated test server...");
    execFileSync("go", ["build", "-tags", "noembedweb", "-o", bin, "./main.go"], {
      cwd: serverDir,
      stdio: "inherit",
    });
  }

  const port = await getFreePort();
  const dir = mkdtempSync(join(runtimeDir, "server-"));
  const logFile = join(dir, "server.log");
  const child = spawn(bin, ["server", "-e", "test"], {
    cwd: serverDir,
    env: {
      ...process.env,
      APP_ENV: "test",
      GOSPEAK_ROLE: "all",
      SERVER_PORT: String(port),
      DB_TYPE: "SQLite",
      DB_PATH: join(dir, "app.db"),
      DB_WAL: "false",
      // auto = embedded NATS (default when NATS_URL empty); none 降级路径已在 v5138 移除
      STATE_STORE: "auto",
      NATS_EMBEDDED_PORT: "0",
      NATS_CONNECT_TIMEOUT: "1s",
      SFU_PROVIDER: "srs",
      SRS_HOST: "localhost",
      SRS_API_PORT: "1985",
      SRS_SECRET: "integration-test-secret",
      SRS_WHIP_URL: "/rtc/v1/whip/",
      SRS_PUBLIC_HOST: `http://127.0.0.1:${port}`,
      GIN_MODE: "release",
      LOG_LEVEL: "warn",
      STORAGE_TYPE: "local",
      STORAGE_PATH_PREFIX: join(dir, "uploads") + "/",
      EMAIL_ENABLED: "false",
      JWT_KEY: "integration-test-jwt-key",
    },
    stdio: ["ignore", "pipe", "pipe"],
  });

  child.stdout.on("data", (chunk: Buffer) => appendFileSync(logFile, chunk));
  child.stderr.on("data", (chunk: Buffer) => appendFileSync(logFile, chunk));

  const state: ServerState = { child, dir, port };
  await waitForServer(state);
  console.log(`[integration] server ready at http://127.0.0.1:${port}`);

  return async () => {
    if (child.exitCode === null) {
      child.kill("SIGTERM");
      await Promise.race([
        new Promise<void>((resolvePromise) => child.once("exit", () => resolvePromise())),
        sleep(8_000),
      ]);
    }
    if (child.exitCode === null) {
      child.kill("SIGKILL");
      await Promise.race([
        new Promise<void>((resolvePromise) => child.once("exit", () => resolvePromise())),
        sleep(2_000),
      ]);
    }
    rmSync(dir, { recursive: true, force: true });
    rmSync(serverInfoPath, { force: true });
  };
}
