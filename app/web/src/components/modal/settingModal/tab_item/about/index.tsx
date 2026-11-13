import { DesktopUpdateSection } from "./desktopUpdate";
import { ProjectSection } from "./project";
import { ServerUpdateSection } from "./serverUpdate";
import { VersionSection } from "./version";
import { Page } from "../shared";
import type { SettingTabConfig } from "../types";

/**
 * 「关于」页共享骨架，具体更新实现按构建目标二选一。
 *
 * 条件编译（构建期）：`__GOSPEAK_DESKTOP__` 是 vite define 注入的字面量，
 * 两个分支都在调用处，死分支（含其中未使用的 import 绑定）被整体摇树剔除——
 * Web 构建里 desktopUpdate.tsx 与 @gospeak/desktop 的更新能力不进包，
 * 桌面构建里 serverUpdate.tsx（依赖 admin API）不进包。
 * 手法与 tab_item/index.ts 里 desktop tab 的注册保持一致。
 */
const UpdateSection = __GOSPEAK_DESKTOP__
	? DesktopUpdateSection
	: ServerUpdateSection;

export function AboutPage() {
	return (
		<Page title="关于" desc="版本信息、软件更新与项目地址。">
			<VersionSection />
			<UpdateSection />
			<ProjectSection />
		</Page>
	);
}

const aboutTab: SettingTabConfig = {
	id: "about",
	label: "关于",
	icon: "info",
	component: AboutPage,
};

export default aboutTab;
