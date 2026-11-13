import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Skeleton } from "./skeleton";

describe("Skeleton", () => {
	it("渲染带脉冲动画的占位块并对读屏隐藏", () => {
		const html = renderToStaticMarkup(<Skeleton className="h-4 w-10" />);
		expect(html).toContain("animate-pulse");
		expect(html).toContain('aria-hidden="true"');
		expect(html).toContain('data-slot="skeleton"');
	});

	it("透传并合并 className（尺寸/圆角覆盖默认值）", () => {
		const html = renderToStaticMarkup(
			<Skeleton className="h-24 w-full rounded-lg" />,
		);
		expect(html).toContain("h-24");
		expect(html).toContain("rounded-lg");
		expect(html).not.toContain("rounded-md");
	});
});
