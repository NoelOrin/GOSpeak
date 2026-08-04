import { createFileRoute, redirect } from "@tanstack/solid-router";
import Network from "lucide-solid/icons/network";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Save from "lucide-solid/icons/save";
import { createResource, createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	drainClusterNode,
	getClusterStats,
	listClusterNodes,
	scaleServer,
	undrainClusterNode,
	type ClusterNodeView,
} from "@/api/cluster";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import { hasPermission } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/manage/cluster/")({
	beforeLoad: () => {
		if (!hasPermission("cluster:read") && !hasPermission("cluster:manage")) {
			throw redirect({ to: "/" });
		}
	},
	component: ClusterPage,
	staticData: {
		title: "集群",
		icon: "icon-manage",
	},
});

function nodeValue(view: ClusterNodeView, key: string): string {
	const value = view.node[key];
	return value === null || value === undefined ? "-" : String(value);
}

function StatusBadge(props: { status: string }) {
	const tone = () => {
		switch (props.status) {
			case "ready":
			case "busy":
				return "badge-success";
			case "draining":
				return "badge-warning";
			case "offline":
			case "unhealthy":
				return "badge-error";
			default:
				return "badge-ghost";
		}
	};
	return (
		<span class={`badge badge-sm ${tone()}`}>{props.status || "pending"}</span>
	);
}

function ClusterPage() {
	const [nodes, { refetch: refetchNodes }] = createResource(listClusterNodes);
	const [stats, { refetch: refetchStats }] = createResource(getClusterStats);
	const [serverUUID, setServerUUID] = createSignal("");
	const [replicas, setReplicas] = createSignal(1);
	const [busy, setBusy] = createSignal(false);
	const canManage = () => hasPermission("cluster:manage");

	const refresh = () => {
		void refetchNodes();
		void refetchStats();
	};

	const handleDrain = async (nodeId: string) => {
		if (busy()) return;
		setBusy(true);
		try {
			await drainClusterNode(nodeId);
			showToast("节点已进入排空", { type: "success" });
			refresh();
		} catch {
		} finally {
			setBusy(false);
		}
	};

	const handleUndrain = async (nodeId: string) => {
		if (busy()) return;
		setBusy(true);
		try {
			await undrainClusterNode(nodeId);
			showToast("节点已恢复调度", { type: "success" });
			refresh();
		} catch {
		} finally {
			setBusy(false);
		}
	};

	const handleScale = async () => {
		const uuid = serverUUID().trim();
		if (!uuid) {
			showToast("请输入 Server UUID", { type: "warning" });
			return;
		}
		if (busy()) return;
		setBusy(true);
		try {
			await scaleServer(uuid, replicas());
			showToast("副本数已调整", { type: "success" });
			refresh();
		} catch {
		} finally {
			setBusy(false);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Network size={18} />}
				title="集群控制面"
				description="Agent / Worker 节点状态与 Server 副本调度"
				actions={
					<button class="btn btn-ghost btn-sm" onClick={refresh}>
						<RefreshCcw size={16} />
						刷新
					</button>
				}
			/>

			<Show when={!stats.loading && stats()} fallback={<ManageLoading />}>
				<ManageSection title="节点概览" padded={false}>
					<div class="grid grid-cols-2 gap-px bg-base-300/70 sm:grid-cols-5">
						{[
							["总节点", stats()?.total_nodes ?? 0],
							["可调度", stats()?.ready_nodes ?? 0],
							["排空", stats()?.draining_nodes ?? 0],
							["离线", stats()?.offline_nodes ?? 0],
							["实例分配", stats()?.assignments ?? 0],
						].map(([label, value]) => (
							<div class="bg-base-100 px-4 py-3">
								<div class="text-xs text-base-content/50">{label}</div>
								<div class="mt-1 text-xl font-bold">{value}</div>
							</div>
						))}
					</div>
				</ManageSection>
			</Show>

			<Show
				when={!nodes.loading}
				fallback={<ManageLoading label="加载节点..." />}
			>
				<ManageSection
					title="节点列表"
					description="注册、心跳、负载与调度状态"
					padded={false}
				>
					<div class="overflow-x-auto">
						<table class="table table-sm">
							<thead>
								<tr>
									<th>节点</th>
									<th>状态</th>
									<th>地址</th>
									<th>SFU</th>
									<th>负载</th>
									<th>实例</th>
									<th class="text-right">操作</th>
								</tr>
							</thead>
							<tbody>
								<For each={nodes() ?? []}>
									{(view) => (
										<tr>
											<td>
												<div class="font-semibold">
													{nodeValue(view, "name")}
												</div>
												<div class="text-xs text-base-content/50">
													{nodeValue(view, "uuid")}
												</div>
											</td>
											<td>
												<StatusBadge status={nodeValue(view, "status")} />
											</td>
											<td class="max-w-52 truncate text-xs">
												{nodeValue(view, "advertise_url")}
											</td>
											<td class="text-xs">{nodeValue(view, "sfu_provider")}</td>
											<td class="text-xs">
												{nodeValue(view, "rooms")} 房 /{" "}
												{nodeValue(view, "connections")} 连接 /{" "}
												{nodeValue(view, "load_percent")}%
											</td>
											<td class="text-xs">
												{nodeValue(view, "serving_servers")}
											</td>
											<td class="text-right">
												<Show when={canManage()}>
													<div class="flex justify-end gap-1">
														<Show
															when={nodeValue(view, "status") === "draining"}
															fallback={
																<button
																	class="btn btn-ghost btn-xs"
																	onClick={() =>
																		void handleDrain(String(view.node.uuid))
																	}
																>
																	排空
																</button>
															}
														>
															<button
																class="btn btn-ghost btn-xs"
																onClick={() =>
																	void handleUndrain(String(view.node.uuid))
																}
															>
																恢复
															</button>
														</Show>
													</div>
												</Show>
											</td>
										</tr>
									)}
								</For>
							</tbody>
						</table>
					</div>
				</ManageSection>
			</Show>

			<Show when={canManage()}>
				<ManageSection title="Server 扩缩容">
					<div class="flex flex-wrap items-end gap-3">
						<label class="form-control w-full max-w-xs">
							<span class="label-text text-xs">Server UUID</span>
							<input
								class="input input-bordered input-sm mt-1"
								value={serverUUID()}
								onInput={(e) => setServerUUID(e.currentTarget.value)}
								placeholder="server-uuid"
							/>
						</label>
						<label class="form-control w-28">
							<span class="label-text text-xs">副本数</span>
							<input
								class="input input-bordered input-sm mt-1"
								type="number"
								min="0"
								value={replicas()}
								onInput={(e) =>
									setReplicas(Math.max(0, Number(e.currentTarget.value) || 0))
								}
							/>
						</label>
						<button
							class="btn btn-primary btn-sm"
							onClick={() => void handleScale()}
						>
							<Save size={16} />
							应用
						</button>
					</div>
				</ManageSection>
			</Show>
		</ManagePage>
	);
}
