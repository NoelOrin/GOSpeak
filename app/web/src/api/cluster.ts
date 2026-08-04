import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
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
	const res = (await apiClient.post({
		url: "/api/v1/cluster/nodes/list",
	})) as AxiosResponse<Result<{ nodes: ClusterNodeView[] }>>;
	return (res as any).data.data.nodes;
}

export async function getClusterStats(): Promise<ClusterStats> {
	const res = (await apiClient.post({
		url: "/api/v1/cluster/stats",
	})) as AxiosResponse<Result<ClusterStats>>;
	return (res as any).data.data;
}

export async function scaleServer(
	serverUuid: string,
	replicas: number,
): Promise<ServerAssignment[]> {
	const res = (await apiClient.post({
		url: "/api/v1/cluster/servers/scale",
		data: { server_uuid: serverUuid, replicas },
	})) as AxiosResponse<Result<{ assignments: ServerAssignment[] }>>;
	return (res as any).data.data.assignments;
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
