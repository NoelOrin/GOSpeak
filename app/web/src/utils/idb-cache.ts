// IndexedDB KV cache for chat messages and conversation list.
// Two object stores:
//   conversations — cached conversation list (full snapshot)
//   messages — per-conversation message buffers (max 200 each)
//
// Strategy: write-through. On first load, show cached data instantly,
// then refresh from API in background. WS events append/update in-place.

const DB_NAME = "gospeak_chat";
const DB_VERSION = 1;
const MAX_MSGS_PER_CONV = 200;

export interface CachedConversation {
	conversation_id: string;
	other_identity: string;
	other_display_name: string;
	other_avatar: string;
	last_content: string;
	last_sender_identity: string;
	last_message_at: number;
	unread_count: number;
	cached_at: number;
}

export interface CachedMessage {
	id: string;
	conversation_id: string;
	content: string;
	sender_identity: string;
	sender_display: string;
	sender_role: string;
	timestamp: number;
	reply_to: string;
	target_identity: string;
}

function openDB(): Promise<IDBDatabase> {
	return new Promise((resolve, reject) => {
		const req = indexedDB.open(DB_NAME, DB_VERSION);
		req.onupgradeneeded = () => {
			const db = req.result;
			if (!db.objectStoreNames.contains("conversations")) {
				db.createObjectStore("conversations", { keyPath: "conversation_id" });
			}
			if (!db.objectStoreNames.contains("messages")) {
				const store = db.createObjectStore("messages", {
					keyPath: "id",
				});
				store.createIndex("by_conv", "conversation_id", { unique: false });
			}
		};
		req.onsuccess = () => resolve(req.result);
		req.onerror = () => reject(req.error);
	});
}

export const chatCache = {
	async getConversations(): Promise<CachedConversation[]> {
		const db = await openDB();
		return new Promise((resolve, reject) => {
			const tx = db.transaction("conversations", "readonly");
			const store = tx.objectStore("conversations");
			const req = store.getAll();
			req.onsuccess = () => resolve(req.result as CachedConversation[]);
			req.onerror = () => reject(req.error);
		});
	},

	async setConversations(list: CachedConversation[]): Promise<void> {
		const db = await openDB();
		return new Promise((resolve, reject) => {
			const tx = db.transaction("conversations", "readwrite");
			const store = tx.objectStore("conversations");
			store.clear();
			for (const item of list) {
				store.put(item);
			}
			tx.oncomplete = () => resolve();
			tx.onerror = () => reject(tx.error);
		});
	},

	async getMessages(convID: string): Promise<CachedMessage[]> {
		const db = await openDB();
		return new Promise((resolve, reject) => {
			const tx = db.transaction("messages", "readonly");
			const store = tx.objectStore("messages");
			const index = store.index("by_conv");
			const req = index.getAll(convID);
			req.onsuccess = () => resolve(req.result as CachedMessage[]);
			req.onerror = () => reject(req.error);
		});
	},

	async appendMessages(convID: string, msgs: CachedMessage[]): Promise<void> {
		const db = await openDB();
		return new Promise((resolve, reject) => {
			const tx = db.transaction("messages", "readwrite");
			const store = tx.objectStore("messages");
			for (const msg of msgs) {
				store.put(msg);
			}
			// Shift old messages out if over limit
			const index = store.index("by_conv");
			const countReq = index.count(convID);
			countReq.onsuccess = () => {
				if (countReq.result > MAX_MSGS_PER_CONV) {
					const cursorReq = index.openCursor(convID, "next");
					const excess = countReq.result - MAX_MSGS_PER_CONV;
					let deleted = 0;
					cursorReq.onsuccess = () => {
						const cursor = cursorReq.result;
						if (cursor && deleted < excess) {
							cursor.delete();
							deleted++;
							cursor.continue();
						} else {
							// Deletion complete (or cursor exhausted)
							resolve();
						}
					};
					cursorReq.onerror = () => reject(cursorReq.error);
				} else {
					resolve();
				}
			};
			countReq.onerror = () => reject(countReq.error);
			tx.onerror = () => reject(tx.error);
		});
	},

	async prependMessages(convID: string, msgs: CachedMessage[]): Promise<void> {
		const db = await openDB();
		return new Promise((resolve, reject) => {
			const tx = db.transaction("messages", "readwrite");
			const store = tx.objectStore("messages");
			for (const msg of msgs) {
				store.put(msg);
			}
			tx.oncomplete = () => resolve();
			tx.onerror = () => reject(tx.error);
		});
	},

	async getCursor(convID: string): Promise<string | null> {
		const db = await openDB();
		return new Promise((resolve, reject) => {
			const tx = db.transaction("messages", "readonly");
			const index = tx.objectStore("messages").index("by_conv");
			const req = index.openCursor(convID, "prev");
			req.onsuccess = () => {
				if (req.result) {
					resolve((req.result.value as CachedMessage).id);
				} else {
					resolve(null);
				}
			};
			req.onerror = () => reject(req.error);
		});
	},

	async clear(): Promise<void> {
		const db = await openDB();
		return new Promise((resolve, reject) => {
			const tx = db.transaction(["conversations", "messages"], "readwrite");
			tx.objectStore("conversations").clear();
			tx.objectStore("messages").clear();
			tx.oncomplete = () => resolve();
			tx.onerror = () => reject(tx.error);
		});
	},
};
