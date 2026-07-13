export interface APIClientAuthBindings {
	getAccessToken: () => string;
	getRefreshToken: () => string;
	updateAccessToken: (token: string) => Promise<void> | void;
	clearAuth: () => Promise<void> | void;
}

let bindings: APIClientAuthBindings = {
	getAccessToken: () => {
		throw new Error("APIClientAuth not initialized");
	},
	getRefreshToken: () => "",
	updateAccessToken: () => undefined,
	clearAuth: () => undefined,
};

export function bindAPIClientAuth(next: APIClientAuthBindings) {
	bindings = next;
}

export function getAPIClientAuth() {
	return bindings;
}
