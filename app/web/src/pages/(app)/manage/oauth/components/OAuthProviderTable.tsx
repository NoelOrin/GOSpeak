import Pencil from "lucide-solid/icons/pencil";
import Trash2 from "lucide-solid/icons/trash-2";
import { For, Show } from "solid-js";
import type { OAuthProvider } from "@/api/oauth";
import {
	ManageSection,
	ManageTag,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
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
		<ManageSection title="已配置提供商" padded={false}>
			<Show
				when={!props.loading}
				fallback={<div class="loading loading-spinner loading-sm m-4" />}
			>
				<Show
					when={props.providers.length > 0}
					fallback={
						<div class="m-4 rounded-xl border border-dashed border-base-300 bg-base-200/20 py-10 text-center text-sm text-base-content/55">
							暂无 OAuth 提供商配置，点击「添加」开始配置
						</div>
					}
				>
					<div class="overflow-x-auto">
						<table class="table table-sm">
							<thead>
								<tr class={manageTableHeadClass}>
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
										<tr class={manageTableRowClass}>
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
													fallback={<ManageTag>通用</ManageTag>}
												>
													<ManageTag>内置</ManageTag>
												</Show>
											</td>
											<td class="max-w-48 truncate text-xs text-base-content/60">
												{p.redirect_url || "-"}
											</td>
											<td>
												<Show
													when={p.enabled}
													fallback={<ManageTag>已禁用</ManageTag>}
												>
													<ManageTag>启用</ManageTag>
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
														class="btn btn-ghost btn-xs text-base-content/60"
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
		</ManageSection>
	);
}
