#!/usr/bin/env node
// Hook 依赖检查棘轮（ratchet）。
//
// biome.json 已启用 lint/correctness/useExhaustiveDependencies（warn）。
// warn 不会让 `biome ci` 失败，因此这里做一层「只许减少、不许增加」的门禁：
// 既有的历史告警作为技术债基线保留可见性，任何新增告警都会让 CI 失败。
// 每清理一处（补全依赖或写明 biome-ignore 理由），就把 BASELINE 同步下调。
//
// 依赖 Biome 的 --reporter=json（当前标记为 experimental）；若输出格式变化，
// 脚本会明确报错而不是静默放行。
import { execFileSync } from "node:child_process";

const BASELINE = 11;
const RULE = "lint/correctness/useExhaustiveDependencies";
const args = [
	"exec",
	"biome",
	"lint",
	"--files-ignore-unknown=true",
	"--reporter=json",
	"./app/web",
	"./packages",
];

let raw = "";
try {
	raw = execFileSync("pnpm", args, {
		encoding: "utf8",
		maxBuffer: 32 * 1024 * 1024,
		stdio: ["ignore", "pipe", "pipe"],
	});
} catch (err) {
	// Biome 在有 error 级诊断时返回非零，但 JSON 仍在 stdout，继续解析以给出具体位置。
	raw = err.stdout?.toString() ?? "";
	if (!raw.trim()) {
		console.error(`无法运行 biome lint: ${err.message}`);
		process.exit(1);
	}
}

let report;
try {
	report = JSON.parse(raw);
} catch {
	console.error(
		"无法解析 biome lint 的 JSON 输出（Biome reporter 格式可能已变化）",
	);
	process.exit(1);
}
if (!Array.isArray(report.diagnostics)) {
	console.error(
		"biome lint JSON 缺少 diagnostics 字段（Biome reporter 格式可能已变化）",
	);
	process.exit(1);
}

const found = report.diagnostics.filter((d) => d.category === RULE);
if (found.length > BASELINE) {
	console.error(
		`新增 hook 依赖告警：当前 ${found.length} 处，基线 ${BASELINE} 处。请补全依赖或用带理由的 biome-ignore 显式豁免。`,
	);
	for (const d of found) {
		const loc = d.location?.path ?? "?";
		console.error(`  - ${loc}: ${d.message ?? ""}`);
	}
	process.exit(1);
}

console.log(
	`hook 依赖告警 ${found.length} 处（基线 ${BASELINE} 处），无新增。`,
);
