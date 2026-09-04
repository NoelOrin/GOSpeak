import { createResource, Show } from "solid-js";
import { Portal } from "solid-js/web";
import { fetchUserInfo } from "@/api/user";
import Avatar from "@/components/common/avatar";

interface UserInfoPopoverProps {
	identity: string;
	pos: { x: number; y: number };
	onClose: () => void;
}

const POPOVER_WIDTH = 224;
const POPOVER_HEIGHT = 210;
const OFFSET = 8;

const UserInfoPopover = (props: UserInfoPopoverProps) => {
	const [userData] = createResource(() => props.identity, fetchUserInfo);

	const cardStyle = () => {
		const vw = window.innerWidth;
		const vh = window.innerHeight;
		let left = props.pos.x + OFFSET;
		let top = props.pos.y + OFFSET;
		if (props.pos.x + OFFSET + POPOVER_WIDTH > vw) {
			left = props.pos.x - POPOVER_WIDTH - OFFSET;
		}
		if (props.pos.y + OFFSET + POPOVER_HEIGHT > vh) {
			top = props.pos.y - POPOVER_HEIGHT - OFFSET;
		}
		return { left: `${left}px`, top: `${top}px` };
	};

	return (
		<Portal>
			{/* biome-ignore lint/a11y/noStaticElementInteractions: backdrop click to close */}
			<div class="fixed inset-0 z-40" onClick={props.onClose} />
			<div
				class="fixed z-50 w-56 bg-base-100 rounded-xl shadow-xl border border-base-300 overflow-hidden"
				style={cardStyle()}
			>
				<Show
					when={!userData.loading}
					fallback={
						<div class="flex justify-center items-center h-24">
							<span class="loading loading-spinner loading-sm" />
						</div>
					}
				>
					<Show
						when={!userData.error && userData()}
						fallback={
							<div class="p-4 text-sm text-base-content/50">
								{userData.error ? "加载失败" : "用户不存在"}
							</div>
						}
					>
						{(user) => (
							<div class="p-4 flex flex-col items-center gap-3">
								<Avatar
									src={user().avatar}
									name={user().display_name || user().name}
									alt={user().name}
									class="size-16"
									textClass="text-xl"
								/>
								<div class="text-center">
									<div class="font-bold text-sm">
										{user().display_name || user().name}
									</div>
									<div class="text-xs text-base-content/50">@{user().name}</div>
									<div class="mt-1 badge badge-ghost badge-xs">
										{user().role}
									</div>
								</div>
							</div>
						)}
					</Show>
				</Show>
			</div>
		</Portal>
	);
};

export default UserInfoPopover;
