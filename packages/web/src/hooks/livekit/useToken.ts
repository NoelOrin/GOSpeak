import apiClient from "@/api/apiClient";
import { useQuery } from "@tanstack/solid-query";

export interface TokenData {
	token: string;
	serverUrl: string;
}

const useToken = () =>
	useQuery(() => ({
		queryKey: ["token"],
		queryFn: async () => {
			const response = await apiClient.post({
				url: "/api/v1/signal/token",
				data: {
					roomName: "test-room",
					identity: `user${Date.now()}`,
					canPublish: true,
					canSubscribe: true,
				},
			});
			return (response as any).data.data as TokenData;
		},
	}));

export default useToken;
