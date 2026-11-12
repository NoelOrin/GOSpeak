import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * media.ts 的权限申请有两个必须成立的行为：
 *
 * 1. 授权失败后必须能重试 —— 之前把失败结果缓存在 audioPermissionPromise 里，
 *    用户点「重试」只会拿到同一个已失败的 promise，永远不会重新申请。
 * 2. 授权失败时 getAudioDevices 必须把"权限"作为原因报出来，而不是继续
 *    enumerateDevices() 拿到空列表后伪装成"读取设备失败"。
 */

const getUserMediaMock = vi.fn();
const enumerateDevicesMock = vi.fn();

function installMediaDevices() {
	Object.defineProperty(globalThis, "navigator", {
		value: {
			mediaDevices: {
				getUserMedia: getUserMediaMock,
				enumerateDevices: enumerateDevicesMock,
			},
		},
		configurable: true,
	});
}

function fakeStream(deviceId = "mic-real") {
	const track = {
		getSettings: () => ({ deviceId }),
		stop: vi.fn(),
	};
	return {
		getAudioTracks: () => [track],
		getTracks: () => [track],
	};
}

const denied = () => {
	const err = new Error("Permission denied");
	err.name = "NotAllowedError";
	return err;
};

describe("requestAudioPermission", () => {
	beforeEach(() => {
		vi.resetModules();
		installMediaDevices();
		getUserMediaMock.mockReset();
		enumerateDevicesMock.mockReset();
	});

	it("授权成功后缓存，重复调用不再弹权限框", async () => {
		getUserMediaMock.mockResolvedValue(fakeStream());
		const { requestAudioPermission } = await import("./media");

		await expect(requestAudioPermission()).resolves.toBe(true);
		await expect(requestAudioPermission()).resolves.toBe(true);
		expect(getUserMediaMock).toHaveBeenCalledTimes(1);
	});

	it("授权被拒后再次调用会重新申请（不缓存失败）", async () => {
		getUserMediaMock.mockRejectedValueOnce(denied());
		const { requestAudioPermission } = await import("./media");

		await expect(requestAudioPermission()).resolves.toBe(false);

		// 用户在系统设置里放行后点重试：必须重新走一次 getUserMedia。
		getUserMediaMock.mockResolvedValueOnce(fakeStream());
		await expect(requestAudioPermission()).resolves.toBe(true);
		expect(getUserMediaMock).toHaveBeenCalledTimes(2);
	});

	it("授权被拒时返回 false，不抛异常", async () => {
		getUserMediaMock.mockRejectedValue(denied());
		const { requestAudioPermission } = await import("./media");

		await expect(requestAudioPermission()).resolves.toBe(false);
	});
});

describe("getAudioDevices", () => {
	beforeEach(() => {
		vi.resetModules();
		installMediaDevices();
		getUserMediaMock.mockReset();
		enumerateDevicesMock.mockReset();
	});

	it("授权成功：返回带真实 label 的设备", async () => {
		getUserMediaMock.mockResolvedValue(fakeStream());
		enumerateDevicesMock.mockResolvedValue([
			{
				deviceId: "mic-real",
				groupId: "g",
				label: "MacBook 麦克风",
				kind: "audioinput",
			},
			{ deviceId: "spk", groupId: "g", label: "扬声器", kind: "audiooutput" },
		]);
		const { getAudioDevices } = await import("./media");

		const devices = await getAudioDevices();
		expect(devices.audioinputs).toHaveLength(1);
		expect(devices.audioinputs[0].label).toBe("MacBook 麦克风");
		expect(devices.audiooutputs).toHaveLength(1);
	});

	it("授权被拒：抛出权限错误，而不是返回空设备列表", async () => {
		getUserMediaMock.mockRejectedValue(denied());
		const { getAudioDevices } = await import("./media");

		await expect(getAudioDevices()).rejects.toMatchObject({
			name: "NotAllowedError",
		});
		// 权限没拿到就不该再去枚举（拿到的只会是空 label 的空壳）
		expect(enumerateDevicesMock).not.toHaveBeenCalled();
	});
});
