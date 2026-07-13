export * from "./core/index";
export type { LoadedPlugin } from "./core/loader";
export { initPlugin, loadPlugin } from "./core/loader";
export { Command, On } from "./decorators/handlers";
export type { PluginMetaInput } from "./decorators/register";
export {
	getPluginModulePath,
	RegisterPlugin,
	setPluginModulePath,
} from "./decorators/register";
export * from "./filters/index";
export type {
	ApiClientOptions,
	AuthCredentials,
	AuthResult,
	BotConfig,
	BotStatus,
	SocketClientOptions,
} from "./runtime/index";
export {
	AuthClient,
	BotRunner,
	GOSpeakApiClient,
	GOSpeakSocketClient,
} from "./runtime/index";
