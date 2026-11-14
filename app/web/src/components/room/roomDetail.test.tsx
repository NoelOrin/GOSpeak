import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import RoomDetail from "./roomDetail";

class ResizeObserverStub {
	observe() {}
	unobserve() {}
	disconnect() {}
}

Object.defineProperty(globalThis, "ResizeObserver", {
	value: ResizeObserverStub,
	writable: true,
});

const mocks = vi.hoisted(() => ({
	setMemberOutputVolume: vi.fn(),
	setMemberMute: vi.fn(),
	socket: {
		rooms: [] as Array<{
			name: string;
			domain_uuid: string | null;
			members: Array<Record<string, unknown>>;
		}>,
		currentRoom: "测试" as string | null,
	},
	voice: {
		otherMemberState: {} as Record<
			string,
			{ outputVolume: number; isMute: boolean }
		>,
	},
}));

/**
 * 加入中 / 失败状态必须作为房间面板自身的内容渲染（0.4.0 的做法），
 * 不能是盖在面板上的浮层卡片 —— 之前那版有 bg-muted + border + shadow-xl，
 * 看起来像弹窗，实际面板里什么都没有。
 */
const session = {
	selectedRoom: { name: "测试", domain_uuid: null },
	phase: "joining_media" as string,
	phaseLabel: "连接媒体...",
	isJoined: false,
	isReconnecting: false,
	isLoading: true,
	error: null as string | null,
	retry: vi.fn(),
};

vi.mock("./VoiceSessionProvider", () => ({
	useVoiceSessionContext: () => session,
}));
// 密码弹窗会连带加载 chatStore/私聊处理器，与本用例无关。
vi.mock("@/components/room/components/passwordModal", () => ({
	default: () => null,
}));
vi.mock("@/stores/socketStore", () => ({
	useSocketStore: (selector: (s: unknown) => unknown) => selector(mocks.socket),
	socketStore: {
		currentDomainUUID: () => null,
		currentRoom: () => null,
		kickMember: vi.fn(),
	},
}));
vi.mock("@/stores/userStore", () => {
	const hook = (selector: (s: unknown) => unknown) =>
		selector({ user: { name: "me" } });
	Object.assign(hook, { getState: () => ({ user: { name: "me" } }) });
	return { useUserStore: hook, default: hook };
});
vi.mock("@/stores/voiceChatStore", () => ({
	useVoiceChatStore: (selector: (s: unknown) => unknown) =>
		selector(mocks.voice),
	useVoiceChatActions: () => ({
		setMemberOutputVolume: mocks.setMemberOutputVolume,
		setMemberMute: mocks.setMemberMute,
	}),
}));
vi.mock("@/handler_audio", () => ({
	setMutedByIdentity: vi.fn(),
	setVolumeByIdentity: vi.fn(),
}));
vi.mock("@/handler_audio/speakingStore", () => ({
	useSpeakingStore: (selector: (s: unknown) => unknown) =>
		selector({ speakingIdentities: [] }),
}));
vi.mock("@/stores/screenShareStore", () => ({
	useScreenShareStore: (selector: (s: unknown) => unknown) =>
		selector({ remote: [], localSharing: false, localTrack: null }),
}));

function reset(overrides: Partial<typeof session> = {}) {
	Object.assign(session, {
		phase: "joining_media",
		phaseLabel: "连接媒体...",
		isJoined: false,
		isLoading: true,
		error: null,
		...overrides,
	});
}

const bob = {
	id: "2",
	identity: "bob",
	name: "bob",
	display_name: "Bob",
	avatar: "",
	is_muted: false,
	is_mic_muted: false,
	joined_at: 0,
};

function joinWithMembers() {
	reset({
		phase: "ready",
		phaseLabel: "已连接",
		isJoined: true,
		isLoading: false,
	});
	mocks.socket.rooms = [{ name: "测试", domain_uuid: null, members: [bob] }];
}

beforeEach(() => {
	mocks.socket.rooms = [];
	mocks.socket.currentRoom = "测试";
	mocks.voice.otherMemberState = {};
});

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
});

describe("RoomDetail / 加入状态渲染在面板内", () => {
	it("加载态直接铺在面板里，没有浮层卡片", () => {
		reset();
		const { container } = render(<RoomDetail />);

		expect(screen.getByText("测试")).toBeTruthy();
		expect(screen.getByText("连接媒体...")).toBeTruthy();
		// 旧的浮层是 absolute inset-0 + 圆角卡片 + 阴影
		expect(container.querySelector(".shadow-xl")).toBeNull();
		expect(container.querySelector('[class*="inset-0"]')).toBeNull();
	});

	it("失败态给出原因和重试按钮", () => {
		reset({
			phase: "failed",
			phaseLabel: "加入失败",
			isLoading: false,
			error: "socket connect timeout",
		});
		render(<RoomDetail />);

		expect(screen.getByText("socket connect timeout")).toBeTruthy();
		expect(screen.getByRole("button", { name: "重试" })).toBeTruthy();
		// 不是密码错误，不应出现重新输入密码
		expect(screen.queryByRole("button", { name: "重新输入密码" })).toBeNull();
	});

	it("已加入但没有成员时提示等待，而不是继续显示加载态", () => {
		reset({
			phase: "ready",
			phaseLabel: "已连接",
			isJoined: true,
			isLoading: false,
		});
		render(<RoomDetail />);

		expect(screen.getByText("已连接，等待成员加入")).toBeTruthy();
		expect(screen.queryByText("连接媒体...")).toBeNull();
	});
});

describe("RoomDetail / 成员卡片音量控件", () => {
	it("静音时音量行仍然可见，滑块禁用", () => {
		joinWithMembers();
		mocks.voice.otherMemberState = {
			bob: { outputVolume: 60, isMute: true },
		};
		render(<RoomDetail />);

		expect(screen.getByText("60%")).toBeTruthy();
		expect(screen.getByRole("button", { name: "取消静音" })).toBeTruthy();
		const slider = screen.getByRole("slider");
		expect(slider.getAttribute("aria-valuenow")).toBe("60");
		expect(slider.getAttribute("data-disabled")).toBe("");
	});

	it("点击百分比把音量恢复到 100", () => {
		joinWithMembers();
		mocks.voice.otherMemberState = {
			bob: { outputVolume: 60, isMute: false },
		};
		render(<RoomDetail />);

		fireEvent.click(screen.getByRole("button", { name: /60%/ }));

		expect(mocks.setMemberOutputVolume).toHaveBeenCalledWith("bob", 100);
	});

	it("点击音量图标切换静音，与成员操作菜单双写一致", () => {
		joinWithMembers();
		mocks.voice.otherMemberState = {
			bob: { outputVolume: 60, isMute: false },
		};
		render(<RoomDetail />);

		fireEvent.click(screen.getByRole("button", { name: "静音" }));

		expect(mocks.setMemberMute).toHaveBeenCalledWith("bob", true);
	});
});
