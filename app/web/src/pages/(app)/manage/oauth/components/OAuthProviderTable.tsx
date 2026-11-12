import { Pencil, Trash2 } from "lucide-react";
import type { OAuthProvider } from "@/api/oauth";
import {
	ManageSection,
	ManageTag,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
import {
	LoadingRegion,
	SkeletonTableRows,
} from "@/components/common/Skeletons";
import ProviderIcon from "@/components/oauth/ProviderIcon";
import { NATIVE_PROVIDERS } from "./presets";

export interface OAuthProviderTableProps {
	loading: boolean;
	providers: OAuthProvider[];
	deletingId: number | null;
	onEdit: (p: OAuthProvider) => void;
	onDelete: (p: OAuthProvider) => void;
}

export default function OAuthProviderTable({
	loading,
	providers,
	deletingId,
	onEdit,
	onDelete,
}: OAuthProviderTableProps) {
	const isNative = (providerName: string) => NATIVE_PROVIDERS.has(providerName);

	const iconBtn =
		"hover:bg-muted inline-flex size-11 min-h-11 min-w-11 cursor-pointer items-center justify-center rounded-sm transition-colors disabled:pointer-events-none disabled:opacity-50";

	return (
		<ManageSection title="已配置提供商" padded={false}>
			{loading ? (
				<LoadingRegion label="正在加载 OAuth 提供商">
					<SkeletonTableRows rows={4} cols={5} />
				</LoadingRegion>
			) : providers.length === 0 ? (
				<div className="border-border/60 bg-muted/20 text-muted-foreground m-4 rounded-lg border border-dashed py-10 text-center text-sm">
					暂无 OAuth 提供商配置，点击「添加」开始配置
				</div>
			) : (
				<div className="overflow-x-auto">
					<table className="w-full text-sm">
						<thead>
							<tr className={manageTableHeadClass}>
								<th className="px-3 py-2 text-left">名称</th>
								<th className="px-3 py-2 text-left">显示名称</th>
								<th className="px-3 py-2 text-left">类型</th>
								<th className="px-3 py-2 text-left">回调 URL</th>
								<th className="px-3 py-2 text-left">状态</th>
								<th className="px-3 py-2 text-left">操作</th>
							</tr>
						</thead>
						<tbody>
							{providers.map((p) => (
								<tr key={p.id} className={manageTableRowClass}>
									<td className="px-3 py-2 font-medium">
										<div className="flex items-center gap-2">
											<ProviderIcon name={p.name} size={16} />
											<span className="font-mono">{p.name}</span>
										</div>
									</td>
									<td className="px-3 py-2">{p.display_name || "-"}</td>
									<td className="px-3 py-2">
										{isNative(p.name) ? (
											<ManageTag>内置</ManageTag>
										) : (
											<ManageTag>通用</ManageTag>
										)}
									</td>
									<td className="text-muted-foreground max-w-48 truncate px-3 py-2 text-xs">
										{p.redirect_url || "-"}
									</td>
									<td className="px-3 py-2">
										{p.enabled ? (
											<ManageTag>启用</ManageTag>
										) : (
											<ManageTag>已禁用</ManageTag>
										)}
									</td>
									<td className="px-3 py-2">
										<div className="flex gap-1">
											<button
												type="button"
												className={iconBtn}
												aria-label={`编辑 ${p.display_name || p.name}`}
												onClick={() => onEdit(p)}
											>
												<Pencil size={14} />
											</button>
											<button
												type="button"
												className={`${iconBtn} text-muted-foreground`}
												aria-label={`删除 ${p.display_name || p.name}`}
												disabled={deletingId === p.id}
												onClick={() => onDelete(p)}
											>
												{deletingId === p.id ? (
													<span className="border-primary border-t-primary size-3.5 animate-spin rounded-full border-2" />
												) : (
													<Trash2 size={14} />
												)}
											</button>
										</div>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</ManageSection>
	);
}
