import type { PermissionItem } from "@/api/permission";

export default function PermissionCard(props: {
	permission: PermissionItem;
	checked: boolean;
	onToggle: (code: string, checked: boolean) => void;
}) {
	return (
		<label class="flex min-h-20 items-start gap-3 rounded-md border border-base-300 p-3 hover:bg-base-200 cursor-pointer">
			<input
				type="checkbox"
				class="checkbox checkbox-sm mt-1"
				checked={props.checked}
				onChange={(e) =>
					props.onToggle(props.permission.code, e.currentTarget.checked)
				}
			/>
			<span class="min-w-0 flex-1">
				<span class="block truncate font-medium text-sm">
					{props.permission.name}
				</span>
				<span class="mt-1 block break-all font-mono text-base-content/50 text-xs">
					{props.permission.code}
				</span>
				<span class="mt-1 line-clamp-2 block text-base-content/60 text-xs">
					{props.permission.description}
				</span>
			</span>
		</label>
	);
}
