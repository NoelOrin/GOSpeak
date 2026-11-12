import { render, screen } from "@testing-library/react";
import { Badge } from "@gospeak/components";
import { describe, expect, it } from "vitest";

describe("Badge", () => {
	it("renders nothing when count is 0", () => {
		const { container } = render(<Badge count={0} />);
		expect(container.firstChild).toBeNull();
	});

	it("renders the count", () => {
		render(<Badge count={7} />);
		expect(screen.getByText("7")).toBeTruthy();
	});

	it("caps at 99+", () => {
		render(<Badge count={150} />);
		expect(screen.getByText("99+")).toBeTruthy();
	});

	it("renders a dot when dot is set", () => {
		const { container } = render(<Badge dot />);
		expect(container.querySelector("[data-slot='badge-dot']")).toBeTruthy();
	});
});
