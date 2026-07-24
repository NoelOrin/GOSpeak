export type MessageDTO = {
	uuid: string;
	room_uuid: string;
	author_id: string;
	content: string;
	reply_to?: string;
	mentions?: string[];
	edited_at?: string | null;
	deleted: boolean;
	created_at: string;
	client_nonce?: string;
};

export type MessageListResult = {
	items: MessageDTO[];
	has_more: boolean;
	next_before: string;
};
