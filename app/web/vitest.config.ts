import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

export default defineConfig(async (env) =>
	mergeConfig(await viteConfig(env), {
		test: {
			environment: "jsdom",
			setupFiles: ["./vitest.setup.ts"],
			exclude: ["**/node_modules/**", "**/dist/**"],
			// 接入 CI 前治理 flaky：domainStore.persistence 6.5s、loadSfuClient 4.2s、
			// socketEvents 3.4s 都会撞上 Vitest 默认 5s；而首个动态 import 用例还要
			// 承担 jsdom/Vite 冷启动（本地实测约 20s）。30s 覆盖冷启动余量，
			// 同时降低 worker 数，避免共享 runner 上资源竞争被误判为回归。
			testTimeout: 30_000,
			hookTimeout: 30_000,
			maxWorkers: 2,
		},
	}),
);
