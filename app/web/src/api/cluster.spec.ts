import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/apiClient", () => {
	const mockPost = vi.fn();
	return {
		default: { post: mockPost },
		mockPost,
	};
});

import apiClient from "@/api/apiClient";
import {
	drainClusterNode,
	getClusterStats,
	listClusterNodes,
	scaleServer,
	undrainClusterNode,
} from "@/api/cluster";

describe("clusterApi", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("listClusterNodes returns node views", async () => {
		const nodes = [{ node: { name: "node-a", status: "ready" }, labels: {} }];
		(apiClient.post as any).mockResolvedValue({ nodes });
		const result = await listClusterNodes();
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/cluster/nodes/list",
		});
		expect(result).toEqual(nodes);
	});

	it("getClusterStats returns stats", async () => {
		const stats = {
			total_nodes: 2,
			ready_nodes: 1,
			draining_nodes: 0,
			offline_nodes: 1,
			assignments: 1,
		};
		(apiClient.post as any).mockResolvedValue(stats);
		const result = await getClusterStats();
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/cluster/stats",
		});
		expect(result).toEqual(stats);
	});

	it("scaleServer sends replicas", async () => {
		(apiClient.post as any).mockResolvedValue({ assignments: [] });
		await scaleServer("srv-1", 3);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/cluster/servers/scale",
			data: { server_uuid: "srv-1", replicas: 3 },
		});
	});

	it("drain and undrain call node endpoints", async () => {
		(apiClient.post as any).mockResolvedValue(null);
		await drainClusterNode("node-a");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/cluster/nodes/drain",
			data: { node_id: "node-a" },
		});
		await undrainClusterNode("node-a");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/cluster/nodes/undrain",
			data: { node_id: "node-a" },
		});
	});
});
