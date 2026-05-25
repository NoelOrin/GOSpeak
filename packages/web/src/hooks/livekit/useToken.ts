import apiClient from "@/api/apiClient";
import { useQuery } from "@tanstack/solid-query";

const useToken = () =>
	useQuery(() => ({
		queryKey: ["token"],
		queryFn: async () => {
			const response = await apiClient.post({
				url: "/api/v1/token",
				data: {
					roomName: "test-room",
					identity: "user1",
					canPublish: true,
					canSubscribe: true,
				},
			});
			return response.data;
		},
	}));
export default useToken;
