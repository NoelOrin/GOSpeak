import Search from "lucide-solid/icons/search";
import { For, Show } from "solid-js";
import type { BackendUser } from "@/api/auth";

export interface UserSearchSelectProps {
	value: number | "";
	onChange: (value: number | "") => void;
	users: BackendUser[];
	searchValue: string;
	onSearchInput: (value: string) => void;
	onSearch: () => void;
	loading?: boolean;
	disabled?: boolean;
	id?: string;
}

export default function UserSearchSelect(props: UserSearchSelectProps) {
	const id = () => props.id || "user-select";

	return (
		<div class="form-control min-w-60 flex-1">
			<label class="label py-1" for={id()}>
				<span class="label-text text-xs font-medium text-base-content/70">
					用户
				</span>
			</label>
			<div class="flex flex-col gap-2">
				<form
					class="flex gap-2"
					onSubmit={(e) => {
						e.preventDefault();
						props.onSearch();
					}}
				>
					<label class="input input-bordered input-sm flex flex-1 items-center gap-2 bg-base-100">
						<Search size={14} class="shrink-0 text-base-content/40" />
						<input
							type="search"
							class="min-w-0 grow bg-transparent outline-none"
							placeholder="搜索用户名、显示名、邮箱"
							value={props.searchValue}
							onInput={(e) => props.onSearchInput(e.currentTarget.value)}
						/>
					</label>
					<button type="submit" class="btn btn-sm">
						搜索
					</button>
				</form>
				<select
					id={id()}
					class="select select-bordered select-sm bg-base-100"
					value={props.value}
					onChange={(e) =>
						props.onChange(
							e.currentTarget.value ? Number(e.currentTarget.value) : "",
						)
					}
					disabled={props.disabled}
				>
					<option value="">选择用户</option>
					<For each={props.users}>
						{(u) => (
							<option value={u.id}>
								{u.display_name || u.name} ({u.name})
							</option>
						)}
					</For>
				</select>
				<Show when={props.loading}>
					<span class="text-xs text-base-content/50">搜索中...</span>
				</Show>
			</div>
		</div>
	);
}
