export * from "./core/index";
export * from "./filters/index";
export {
	RegisterPlugin,
	setPluginModulePath,
	getPluginModulePath,
} from "./decorators/register";
export type { PluginMetaInput } from "./decorators/register";
export { On, Command } from "./decorators/handlers";
export { loadPlugin, initPlugin } from "./core/loader";
export type { LoadedPlugin } from "./core/loader";
export { GOSpeakApiClient, GOSpeakSocketClient, BotRunner, AuthClient } from "./runtime/index";
export type {
	BotConfig,
	BotStatus,
	ApiClientOptions,
	SocketClientOptions,
	AuthCredentials,
	AuthResult,
} from "./runtime/index";
