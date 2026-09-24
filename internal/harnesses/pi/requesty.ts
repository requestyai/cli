// Requesty provider for Pi. Written by `requesty pi` before every launch and
// loaded with `--extension`, so nothing in Pi's own configuration changes.
//
// The launch passes everything through the environment:
//   REQUESTY_API_KEY     the key for this profile
//   REQUESTY_BASE_URL    the router, without a /v1 suffix
//   REQUESTY_PI_CATALOG  a JSON file listing the models the profile can use
import { readFileSync } from "node:fs";
import type { ExtensionAPI, ProviderModelConfig } from "@earendil-works/pi-coding-agent";

const PROVIDER = "requesty";
const DEFAULT_BASE_URL = "https://router.requesty.ai";

// The native Anthropic Messages format lets Requesty apply automatic prompt
// caching; it takes the router's base URL without /v1.
const API = "anthropic-messages";

const trimmed = (value: string | undefined): string | undefined => {
	const text = value?.trim();
	return text === undefined || text === "" ? undefined : text;
};

const loadCatalog = (path: string | undefined): ProviderModelConfig[] => {
	if (path === undefined) {
		return [];
	}
	const parsed: unknown = JSON.parse(readFileSync(path, "utf8"));
	const models = (parsed as { models?: unknown }).models;
	return Array.isArray(models) ? (models as ProviderModelConfig[]) : [];
};

export default function (pi: ExtensionAPI): void {
	if (typeof pi.registerProvider !== "function") {
		throw new Error("Requesty needs pi 0.84.0 or newer; upgrade pi and try again.");
	}

	const apiKey = trimmed(process.env.REQUESTY_API_KEY);
	if (apiKey === undefined) {
		throw new Error("REQUESTY_API_KEY is not set; start Pi with `requesty pi`.");
	}

	pi.registerProvider(PROVIDER, {
		name: "Requesty",
		baseUrl: trimmed(process.env.REQUESTY_BASE_URL) ?? DEFAULT_BASE_URL,
		api: API,
		apiKey,
		headers: {
			"HTTP-Referer": "https://pi.dev",
			"X-Title": "Pi",
		},
		models: loadCatalog(trimmed(process.env.REQUESTY_PI_CATALOG)),
	});
}
