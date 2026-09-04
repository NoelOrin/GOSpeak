export enum EventType {
	OnBotLoaded = "OnBotLoaded",
	AdapterMessage = "AdapterMessage",
	OnMessageReceived = "OnMessageReceived",
	OnMessageSent = "OnMessageSent",
	OnRoomCreated = "OnRoomCreated",
	OnRoomJoined = "OnRoomJoined",
	OnRoomUpdated = "OnRoomUpdated",
	OnRoomLeft = "OnRoomLeft",
	OnMemberStateChanged = "OnMemberStateChanged",
	OnMemberJoined = "OnMemberJoined",
	OnMemberLeft = "OnMemberLeft",
	OnMemberKicked = "OnMemberKicked",
	OnUserMuted = "OnUserMuted",
	OnUserUnmuted = "OnUserUnmuted",
	OnSpeechPartial = "OnSpeechPartial",
	OnSpeechFinal = "OnSpeechFinal",
	OnPluginLoaded = "OnPluginLoaded",
	OnPluginUnloaded = "OnPluginUnloaded",
	OnPluginError = "OnPluginError",
	OnActiveSpeakers = "OnActiveSpeakers",
}

export type PermissionLevel =
	| "owner"
	| "admin"
	| "moderator"
	| "member"
	| "guest";

export interface RoomRef {
	id: string;
	name: string;
}

export interface MemberRef {
	identity: string;
	name: string;
	role: PermissionLevel;
}

export interface MessageEvent {
	eventType: EventType;
	messageId: string;
	room: RoomRef;
	sender: MemberRef;
	content: string;
	rawCommand?: ParsedCommand;
	isCommand: boolean;
	timestamp: number;
}

export interface ParsedCommand {
	name: string;
	args: string[];
	raw: string;
	alias?: string;
}

export interface RoomEvent {
	eventType: EventType;
	room: RoomRef;
	actor?: MemberRef;
	timestamp: number;
}

export interface MemberStateEvent {
	eventType: EventType;
	room: RoomRef;
	member: MemberRef;
	muted: boolean;
	volume?: number;
	timestamp: number;
}

export interface UserMuteEvent {
	eventType: EventType.OnUserMuted | EventType.OnUserUnmuted;
	userId: number;
	duration?: number;
	permanent?: boolean;
	reason?: string;
	expiresAt?: string;
	timestamp: number;
}

export interface ActiveSpeakersEvent {
	eventType: EventType.OnActiveSpeakers;
	room: RoomRef;
	identities: string[];
	timestamp: number;
}

export interface SpeechEvent {
	eventType: EventType.OnSpeechPartial | EventType.OnSpeechFinal;
	room: string;
	speaker: string;
	text: string;
	isFinal: boolean;
	confidence?: number;
	timestamp: number;
}

export interface PluginErrorEvent {
	eventType: EventType.OnPluginError;
	pluginName: string;
	handler: string;
	error: Error;
	timestamp: number;
}

export interface LifecycleEvent {
	eventType:
		| EventType.OnBotLoaded
		| EventType.OnPluginLoaded
		| EventType.OnPluginUnloaded
		| EventType.OnPluginError;
	pluginName?: string;
	timestamp: number;
}

export type BotEvent =
	| MessageEvent
	| RoomEvent
	| MemberStateEvent
	| UserMuteEvent
	| SpeechEvent
	| ActiveSpeakersEvent
	| PluginErrorEvent
	| LifecycleEvent;

export function createBotEvent(
	eventType: LifecycleEvent["eventType"],
	pluginName?: string,
): LifecycleEvent {
	return {
		eventType,
		pluginName,
		timestamp: Date.now(),
	};
}
