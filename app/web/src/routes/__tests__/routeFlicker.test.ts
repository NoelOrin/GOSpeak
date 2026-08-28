import { describe, expect, it } from "vitest";
import fs from "node:fs";
import path from "node:path";

function readText(candidates: string[]): string {
	for (const p of candidates) {
		try {
			return fs.readFileSync(p, "utf8");
		} catch {}
	}
	throw new Error(`none of candidates exists: ${candidates.join(", ")}`);
}

function resolveWebFile(rel: string): string {
	// vitest cwd may be app/web or repo root; try both
	const candidates = [
		path.join(process.cwd(), rel),
		path.join(process.cwd(), "app/web", rel),
		path.resolve(__dirname, "..", "..", rel.replace(/^src\//, "")),
		path.resolve(__dirname, "../../..", rel),
	];
	// normalize: try direct absolute from repo root
	const repoRootCandidates = [
		path.join(
			"/Users/noelorin/GOSpeak",
			rel.startsWith("app/") ? rel : `app/web/${rel}`,
		),
	];
	return readText([...candidates, ...repoRootCandidates]);
}

describe("route flicker regression", () => {
	it("manage page should not start from transparent frame (no opacity:0 flash)", () => {
		const css = resolveWebFile("src/assets/styles/global.css");
		// .manage-page 140ms 淡入从 opacity:0 起帧会在每次切页时制造可感知的空窗
		// 修后应移除该动画或至少不以透明起帧
		const hasManagePageAnimation = /\.manage-page\s*\{[^}]*animation\s*:/s.test(
			css,
		);
		const hasOpacityZeroKeyframes =
			/@keyframes\s+manage-page-enter[\s\S]*?opacity:\s*0/s.test(css);
		expect(
			hasManagePageAnimation && hasOpacityZeroKeyframes,
			"manage-page 不应再使用从 opacity:0 开始的入场动画（会在路由切换时闪白/闪烁）",
		).toBe(false);
	});

	it("route files should not export extra default that breaks auto code splitting", () => {
		const files = [
			"src/pages/(app)/discover/index.tsx",
			"src/pages/(app)/link/index.tsx",
			"src/pages/invite/d/$code/index.tsx",
		];
		const offenders: string[] = [];
		for (const rel of files) {
			const text = resolveWebFile(rel);
			if (/export\s+default\s+RouteComponent/.test(text)) {
				offenders.push(rel);
			}
		}
		expect(
			offenders,
			`以下路由文件仍在额外导出 default，会阻止 tanstack autoCodeSplitting 并在切换时放大空窗/闪烁风险: ${offenders.join(", ")}`,
		).toEqual([]);
	});
});
