export type MessageDTO = {
	uuid: string;
	room_uuid: string;
	author_id: string;
	author_name?: string;
	author_avatar?: string;
	content: string;
	reply_to?: string;
	mentions?: string[];
	edited_at?: string | null;
	deleted: boolean;
	created_at: string;
	client_nonce?: string;
	/**
	 * 仅前端本地使用，不来自服务端：
	 * pending = 乐观插入、等 ack；failed = 发送失败，保留在列表里等用户重试。
	 */
	_status?: "pending" | "failed";
	_error?: string;
};

export type MessageListResult = {
	items: MessageDTO[];
	has_more: boolean;
	next_before: string;
};
