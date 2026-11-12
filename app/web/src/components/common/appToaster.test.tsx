import { cleanup, render, waitFor } from "@testing-library/react";
import { toast } from "@gospeak/components";
import { act } from "react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
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

	afterEach(() => {
		// sonner 的 toast 自带自动关闭计时器（默认数秒）。不显式清理的话，
		// 计时器会在用例结束、jsdom 环境销毁之后才触发 React 调度，
		// 抛出 "ReferenceError: window is not defined" 的 unhandled error，
		// 让整个 vitest 进程以非零码退出——测试全绿但 CI 红。
		// 必须先 dismiss 再卸载，确保计时器在环境还活着时被取消。
		toast.dismiss();
		cleanup();
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
