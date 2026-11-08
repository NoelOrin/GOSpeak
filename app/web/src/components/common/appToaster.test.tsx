import { render, waitFor } from "@testing-library/react";
import { toast } from "@gospeak/components";
import { act } from "react";
import { beforeEach, describe, expect, it } from "vitest";
import { useThemeStore } from "@/stores/themeStore";
import AppToaster from "./appToaster";

const toasterNode = () =>
	document.querySelector("[data-sonner-toaster]") as HTMLElement | null;

/** sonner 的列表节点在首条 toast 之后才挂载，这里用它作为「宿主已就绪」的信号。 */
async function mountAndToast() {
	render(<AppToaster />);
	await act(async () => {
		toast.success("保存成功");
		await new Promise((resolve) => setTimeout(resolve, 0));
	});
	await waitFor(() => expect(toasterNode()).not.toBeNull());
	return toasterNode();
}

describe("AppToaster", () => {
	beforeEach(() => {
		useThemeStore.setState({ isDark: false });
		toast.dismiss();
	});

	it("mounts a sonner host so toast calls actually render", async () => {
		const node = await mountAndToast();
		expect(node?.textContent).toContain("保存成功");
	});

	it("follows the app theme instead of prefers-color-scheme", async () => {
		const node = await mountAndToast();
		expect(node?.getAttribute("data-sonner-theme")).toBe("light");

		await act(async () => {
			useThemeStore.setState({ isDark: true });
		});
		expect(toasterNode()?.getAttribute("data-sonner-theme")).toBe("dark");
	});
});
