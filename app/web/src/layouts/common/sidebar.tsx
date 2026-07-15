import { useNavigate } from "@tanstack/solid-router";
import Headphones from "lucide-solid/icons/headphones";
import Home from "lucide-solid/icons/home";
import Plus from "lucide-solid/icons/plus";
import Settings from "lucide-solid/icons/settings";
import ShieldCheck from "lucide-solid/icons/shield-check";
import { Show } from "solid-js";
import Divider from "@/components/common/divider";
import OptionSquare from "@/components/common/optionSquare";
import { hasManageAccess } from "@/utils/permissions";

interface SidebarProps {
	onOpenSettings?: () => void;
}

const iconProps = {
	size: 22,
	strokeWidth: 2.1,
} as const;

const Sidebar = (props: SidebarProps) => {
	const navigate = useNavigate();

	return (
		<div class="flex flex-col justify-start items-center space-y-2 px-2 w-16 select-none">
			{/* @ts-ignore */}
			<OptionSquare label="首页" onClick={() => navigate({ to: "/" })}>
				<Home {...iconProps} />
			</OptionSquare>

			<OptionSquare
				label="频道"
				onClick={() => navigate({ to: "/channel", search: { id: 12413 } })}
			>
				<Headphones {...iconProps} />
			</OptionSquare>

			<OptionSquare label="设置" onClick={() => props.onOpenSettings?.()}>
				<Settings {...iconProps} />
			</OptionSquare>
			<Show when={hasManageAccess()}>
				<OptionSquare label="管理" onClick={() => navigate({ to: "/manage" })}>
					<ShieldCheck {...iconProps} />
				</OptionSquare>
			</Show>
			<Divider />

			<div class="flex flex-col flex-1 items-end space-y-2 h-full">
				<OptionSquare label="新会话">
					<Plus {...iconProps} />
				</OptionSquare>
			</div>
		</div>
	);
};
export default Sidebar;
