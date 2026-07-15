import Pencil from "lucide-solid/icons/pencil";
import Trash2 from "lucide-solid/icons/trash-2";
import { For, Show } from "solid-js";
import type { OAuthProvider } from "@/api/oauth";
import ProviderIcon from "@/components/oauth/ProviderIcon";
import { NATIVE_PROVIDERS } from "./presets";

export interface OAuthProviderTableProps {
	loading: boolean;
	providers: OAuthProvider[];
	deletingId: number | null;
	onEdit: (p: OAuthProvider) => void;
	onDelete: (p: OAuthProvider) => void;
}

export default function OAuthProviderTable(props: OAuthProviderTableProps) {
	const isNative = (providerName: string) => NATIVE_PROVIDERS.has(providerName);

	return (
		<div>
			<Show
				when={!props.loading}
				fallback={<div class="loading loading-spinner loading-sm" />}
			>
				<Show
					when={props.providers.length > 0}
					fallback={
						<div class="text-base-content/50 py-8 text-center text-sm">
							暂无 OAuth 提供商配置，点击「添加」开始配置
						</div>
					}
				>
					<div class="overflow-x-auto">
						<table class="table table-zebra table-sm">
							<thead>
								<tr>
									<th>名称</th>
									<th>显示名称</th>
									<th>类型</th>
									<th>回调 URL</th>
									<th>状态</th>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								<For each={props.providers}>
									{(p) => (
										<tr>
											<td class="font-medium">
												<div class="flex items-center gap-2">
													<ProviderIcon name={p.name} size={16} />
													<span class="font-mono">{p.name}</span>
												</div>
											</td>
											<td>{p.display_name || "-"}</td>
											<td>
												<Show
													when={isNative(p.name)}
													fallback={
														<span class="badge badge-info badge-sm">通用</span>
													}
												>
													<span class="badge badge-ghost badge-sm">内置</span>
												</Show>
											</td>
											<td class="max-w-48 truncate text-xs text-base-content/60">
												{p.redirect_url || "-"}
											</td>
											<td>
												<Show
													when={p.enabled}
													fallback={
														<span class="badge badge-ghost badge-sm">
															已禁用
														</span>
													}
												>
													<span class="badge badge-success badge-sm">启用</span>
												</Show>
											</td>
											<td>
												<div class="flex gap-1">
													<button
														type="button"
														class="btn btn-ghost btn-xs"
														onClick={() => props.onEdit(p)}
													>
														<Pencil size={14} />
													</button>
													<button
														type="button"
														class="btn btn-ghost btn-xs text-error"
														disabled={props.deletingId === p.id}
														onClick={() => props.onDelete(p)}
													>
														<Show
															when={props.deletingId === p.id}
															fallback={<Trash2 size={14} />}
														>
															<span class="loading loading-spinner loading-xs" />
														</Show>
													</button>
												</div>
											</td>
										</tr>
									)}
								</For>
							</tbody>
						</table>
					</div>
				</Show>
			</Show>
		</div>
	);
}
