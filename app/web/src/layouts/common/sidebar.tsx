import { useNavigate } from "@tanstack/solid-router";
import ShieldCheck from "lucide-solid/icons/shield-check";
import { Show } from "solid-js";
import Divider from "@/components/common/divider";
import OptionSquare from "@/components/common/optionSquare";
import SvgIcon from "@/components/svgIcon";
import { hasManageAccess } from "@/utils/permissions";

interface SidebarProps {
	onOpenSettings?: () => void;
}

const Sidebar = (props: SidebarProps) => {
	const navigate = useNavigate();

	return (
		<div class="flex flex-col justify-start items-center space-y-2 px-2 w-16 select-none">
			{/* @ts-ignore */}
			<OptionSquare label="首页" onClick={() => navigate({ to: "/" })}>
				<SvgIcon width={24} height={24} name="home" />
			</OptionSquare>

			<OptionSquare
				label="频道"
				onClick={() => navigate({ to: "/channel", search: { id: 12413 } })}
			>
				<SvgIcon width={24} height={24} name="list-two" fill="none" />
			</OptionSquare>

			<OptionSquare label="设置" onClick={() => props.onOpenSettings?.()}>
				<SvgIcon width={24} height={24} name="setting-two" />
			</OptionSquare>
			<Show when={hasManageAccess()}>
				<OptionSquare label="管理" onClick={() => navigate({ to: "/manage" })}>
					<ShieldCheck size={24} />
				</OptionSquare>
			</Show>
			<Divider />

			<div class="flex flex-col flex-1 items-end space-y-2 h-full">
				<OptionSquare label="新会话">+</OptionSquare>
				{/* <OptionSquare>123</OptionSquare> */}
			</div>
		</div>
	);
};
export default Sidebar;
