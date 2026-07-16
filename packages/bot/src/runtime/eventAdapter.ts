import type { BotEvent, MessageEvent, ParsedCommand } from "../core/types";
import { EventType } from "../core/types";

/**
 * BroadcastBotMessage is the server→client payload for bot:command and bot:message.
 * Matches the Go server's broadcastBotMessage struct.
 */
export interface BroadcastBotMessage {
	room: string;
	content: string;
	replyTo?: string;
	messageId: string;
	from: {
		identity: string;
		displayName: string;
		role: string;
	};
	timestamp: number;
}

/**
 * EventAdapter translates raw socket event payloads into BotEvent objects
 * that can be dispatched through the EventBus to plugins.
 *
 * Key mappings:
 *   bot:command → AdapterMessage + OnMessageReceived (two events)
 *   bot:message → AdapterMessage + OnMessageReceived
 *
 * This separation lets plugins listen for raw commands (AdapterMessage)
 * or processed messages (OnMessageReceived).
 */
export class EventAdapter {
	/**
	 * Parse a string-encoded broadcastBotMessage payload into one or more BotEvents.
	 * Used when the socket client receives a bot:command or bot:message event.
	 */
	adaptBotMessage(rawPayload: string, eventType: EventType): BotEvent[] {
		let msg: BroadcastBotMessage;
		try {
			msg = JSON.parse(rawPayload) as BroadcastBotMessage;
		} catch {
			return [];
		}
		if (!msg.content || !msg.room) return [];

		const results: BotEvent[] = [];

		const command = this.parseCommand(msg.content);

		const messageEvent: MessageEvent = {
			eventType: EventType.OnMessageReceived,
			messageId: msg.messageId,
			room: { id: msg.room, name: msg.room },
			sender: {
				identity: msg.from.identity,
				name: msg.from.displayName || msg.from.identity,
				role: (msg.from.role as MessageEvent["sender"]["role"]) ?? "member",
			},
			content: msg.content,
			rawCommand: command ?? undefined,
			isCommand: command !== null,
			timestamp: msg.timestamp || Date.now(),
		};

		results.push(messageEvent);

		// Also emit as AdapterMessage (raw form)
		const adapterEvent: MessageEvent = {
			...messageEvent,
			eventType: EventType.AdapterMessage,
			messageId: msg.messageId,
		};
		results.push(adapterEvent);

		return results;
	}

	/**
	 * Parse a bot command string like "/kick alice" or "!help".
	 * Returns null if the text is not a command.
	 */
	private parseCommand(text: string): ParsedCommand | null {
		const trimmed = text.trim();
		if (!trimmed.startsWith("/") && !trimmed.startsWith("!")) return null;

		const parts = trimmed.slice(1).split(/\s+/);
		if (parts.length === 0 || parts[0] === "") return null;

		return {
			name: parts[0].toLowerCase(),
			args: parts.slice(1),
			raw: trimmed,
		};
	}
}
