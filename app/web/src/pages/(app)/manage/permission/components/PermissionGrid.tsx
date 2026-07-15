import ShieldX from "lucide-solid/icons/shield-x";
import { For, Show } from "solid-js";
import type { PermissionItem } from "@/api/permission";
import PermissionCard from "./PermissionCard";

export default function PermissionGrid(props: {
	groupedPermissions: {
		domain: string;
		label: string;
		items: PermissionItem[];
	}[];
	selectedCodes: Set<string>;
	loading: boolean;
	onToggle: (code: string, checked: boolean) => void;
}) {
	return (
		<div class="relative min-h-0">
			<Show
				when={props.groupedPermissions.length > 0}
				fallback={<div class="loading loading-spinner loading-md" />}
			>
				<Show when={props.loading}>
					<div class="absolute inset-x-0 top-0 flex justify-center py-2 z-10">
						<div class="loading loading-spinner loading-sm" />
					</div>
				</Show>
				<div class="flex flex-col gap-5">
					<For each={props.groupedPermissions}>
						{(group) => (
							<section>
								<div class="mb-2 flex items-center gap-2 text-base-content/70 text-sm">
									<ShieldX size={15} />
									<span>{group.label}</span>
								</div>
								<div class="grid grid-cols-2 gap-2 xl:grid-cols-3 max-md:grid-cols-1">
									<For each={group.items}>
										{(permission) => (
											<PermissionCard
												permission={permission}
												checked={props.selectedCodes.has(permission.code)}
												onToggle={props.onToggle}
											/>
										)}
									</For>
								</div>
							</section>
						)}
					</For>
				</div>
			</Show>
		</div>
	);
}
