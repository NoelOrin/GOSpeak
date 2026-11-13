import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import {
	LoadingRegion,
	SkeletonCardGrid,
	SkeletonForm,
	SkeletonListRows,
	SkeletonPage,
	SkeletonTableRows,
} from "./Skeletons";

afterEach(cleanup);

describe("Skeletons", () => {
	it("LoadingRegion 暴露 status 语义与读屏文案", () => {
		const { getByRole } = render(
			<LoadingRegion label="正在加载用户">
				<SkeletonTableRows rows={2} cols={3} />
			</LoadingRegion>,
		);
		const region = getByRole("status");
		expect(region.getAttribute("aria-busy")).toBe("true");
		expect(region.getAttribute("aria-label")).toBe("正在加载用户");
	});

	it("SkeletonTableRows 按 rows*cols 渲染占位块", () => {
		const { container } = render(
			<LoadingRegion label="加载">
				<SkeletonTableRows rows={3} cols={4} />
			</LoadingRegion>,
		);
		expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBe(
			12,
		);
	});

	it("SkeletonListRows / SkeletonCardGrid / SkeletonForm 至少渲染一个占位块", () => {
		for (const node of [
			<SkeletonListRows key="l" rows={4} />,
			<SkeletonCardGrid key="c" cards={4} />,
			<SkeletonForm key="f" fields={3} />,
		]) {
			const { container, unmount } = render(node);
			expect(
				container.querySelectorAll('[data-slot="skeleton"]').length,
			).toBeGreaterThan(0);
			unmount();
		}
	});

	it("SkeletonPage 保留页面加载骨架的可访问性契约", () => {
		const { getByRole } = render(<SkeletonPage />);
		const region = getByRole("status");
		expect(region.getAttribute("aria-label")).toBe("页面加载中");
		expect(
			region.querySelectorAll('[data-slot="skeleton"]').length,
		).toBeGreaterThan(0);
	});
});
