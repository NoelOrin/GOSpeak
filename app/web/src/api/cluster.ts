import apiClient from "./apiClient";

export interface ClusterNodeView {
	node: Record<string, unknown>;
	labels: Record<string, string>;
}

export interface ClusterStats {
	total_nodes: number;
	ready_nodes: number;
	draining_nodes: number;
	offline_nodes: number;
	assignments: number;
}

export interface ServerAssignment {
	id: number;
	server_uuid: string;
	node_uuid: string;
	status: string;
}

export async function listClusterNodes(): Promise<ClusterNodeView[]> {
	const data = await apiClient.post<{ nodes: ClusterNodeView[] }>({
		url: "/api/v1/cluster/nodes/list",
	});
	return data.nodes;
}

export async function getClusterStats(): Promise<ClusterStats> {
	const data = await apiClient.post<ClusterStats>({
		url: "/api/v1/cluster/stats",
	});
	return data;
}

export async function scaleServer(
	serverUuid: string,
	replicas: number,
): Promise<ServerAssignment[]> {
	const data = await apiClient.post<{ assignments: ServerAssignment[] }>({
		url: "/api/v1/cluster/servers/scale",
		data: { server_uuid: serverUuid, replicas },
	});
	return data.assignments;
}

export async function listServerAssignments(
	serverUuid: string,
): Promise<ServerAssignment[]> {
	const data = await apiClient.post<{ assignments: ServerAssignment[] }>({
		url: "/api/v1/cluster/servers/list",
		data: { server_uuid: serverUuid },
	});
	return data.assignments;
}

export async function drainServerAssignments(
	serverUuid: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/cluster/servers/drain",
		data: { server_uuid: serverUuid },
	});
}

export async function autoScaleServer(
	serverUuid: string,
	replicas: number,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/cluster/servers/autoscale",
		data: { server_uuid: serverUuid, replicas },
	});
}

export async function drainClusterNode(nodeId: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/cluster/nodes/drain",
		data: { node_id: nodeId },
	});
}

export async function undrainClusterNode(nodeId: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/cluster/nodes/undrain",
		data: { node_id: nodeId },
	});
}
