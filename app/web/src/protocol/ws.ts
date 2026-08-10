export interface WSEndpointInfo {
	/** Worker 节点地址；后端未返回时由 wsClient 沿用当前连接地址。 */
	url?: string;
}
