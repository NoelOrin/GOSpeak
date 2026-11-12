import {
	Button,
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@gospeak/components";
import { Search } from "lucide-react";
import type { BackendUser } from "@/api/auth";

// Radix Select 不允许空字符串作为 value，用哨兵值表示「未选择」
const NONE_VALUE = "__none__";

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

export default function UserSearchSelect({
	value,
	onChange,
	users,
	searchValue,
	onSearchInput,
	onSearch,
	loading,
	disabled,
	id = "user-select",
}: UserSearchSelectProps) {
	return (
		<div className="min-w-60 flex-1">
			<label
				className="text-muted-foreground mb-1 block py-1 text-xs font-medium"
				htmlFor={id}
			>
				用户
			</label>
			<div className="flex flex-col gap-2">
				<form
					className="flex gap-2"
					onSubmit={(e) => {
						e.preventDefault();
						onSearch();
					}}
				>
					<label className="border-input bg-card flex h-9 flex-1 items-center gap-2 rounded-sm border px-3">
						<Search size={14} className="text-faint-foreground shrink-0" />
						<input
							type="search"
							className="min-w-0 grow bg-transparent text-sm outline-none"
							placeholder="搜索用户名、显示名、邮箱"
							value={searchValue}
							onChange={(e) => onSearchInput(e.currentTarget.value)}
						/>
					</label>
					<Button type="submit" variant="outline">
						搜索
					</Button>
				</form>
				<Select
					value={value === "" ? NONE_VALUE : String(value)}
					onValueChange={(next) =>
						onChange(next === NONE_VALUE ? "" : Number(next))
					}
					disabled={disabled}
				>
					<SelectTrigger id={id} className="h-9">
						<SelectValue placeholder="选择用户" />
					</SelectTrigger>
					<SelectContent>
						<SelectItem value={NONE_VALUE}>选择用户</SelectItem>
						{users.map((u) => (
							<SelectItem key={u.id} value={String(u.id)}>
								{u.display_name || u.name} ({u.name})
							</SelectItem>
						))}
					</SelectContent>
				</Select>
				{loading && (
					<span className="text-faint-foreground text-xs">搜索中...</span>
				)}
			</div>
		</div>
	);
}
