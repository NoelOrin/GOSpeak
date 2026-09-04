import { showToast } from "solid-notifications";

export function showLoginSuccessToast() {
	showToast("登录成功", { type: "success" });
}
