// split-openapi-by-tag.js
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const DOC_DIR = path.join(__dirname, "doc");

/**
 * Recursively collect $ref references from an object
 * @param {any} obj - Object to scan
 * @param {Set} refSet - Set to store definition names
 */
function collectRefs(obj, refSet) {
	if (!obj || typeof obj !== "object") return;
	if (Array.isArray(obj)) {
		obj.forEach((item) => {
			collectRefs(item, refSet);
		});
		return;
	}
	Object.entries(obj).forEach(([key, value]) => {
		if (key === "$ref" && typeof value === "string") {
			const match = value.match(/^#\/definitions\/(.+)$/);
			if (match) refSet.add(match[1]);
		} else {
			collectRefs(value, refSet);
		}
	});
}

/**
 * Resolve all nested references
 * @param {Set} initialRefs - Initial set of references
 * @param {object} definitions - All definitions from the API doc
 * @returns {Set} Resolved references
 */
function resolveAllRefs(initialRefs, definitions) {
	const resolved = new Set(initialRefs);
	const queue = Array.from(initialRefs);
	while (queue.length > 0) {
		const defName = queue.shift();
		const defObj = definitions[defName];
		if (defObj) {
			const nestedRefs = new Set();
			collectRefs(defObj, nestedRefs);
			nestedRefs.forEach((name) => {
				if (!resolved.has(name)) {
					resolved.add(name);
					queue.push(name);
				}
			});
		}
	}
	return resolved;
}

/**
 * Process a single doc subdirectory (e.g., scada/, user/)
 * @param {string} subdir - Absolute path to the subdirectory
 */
function processDocDir(subdir) {
	const inputFile = path.join(subdir, "api.v1.json");
	if (!fs.existsSync(inputFile)) {
		console.warn(`⚠️  Skipping ${path.basename(subdir)}: api.v1.json not found`);
		return;
	}

	// Read and parse the OpenAPI doc
	const rawData = fs.readFileSync(inputFile, "utf8");
	const api = JSON.parse(rawData);

	const {
		swagger,
		schemes,
		info,
		host,
		basePath,
		securityDefinitions,
		paths,
		definitions,
	} = api;
	const tagGroups = new Map();

	// Group paths by tags
	Object.entries(paths).forEach(([pathUrl, pathItem]) => {
		Object.entries(pathItem).forEach(([method, operation]) => {
			if (typeof operation !== "object" || operation === null) return;
			const tags = operation.tags || ["Default"];
			tags.forEach((tag) => {
				if (!tagGroups.has(tag)) {
					tagGroups.set(tag, { paths: {}, refs: new Set() });
				}
				const group = tagGroups.get(tag);
				if (!group.paths[pathUrl]) group.paths[pathUrl] = {};
				group.paths[pathUrl][method] = operation;
			});
		});
	});

	// Process each tag group
	tagGroups.forEach((group, tag) => {
		// Collect direct references
		const directRefs = new Set();
		collectRefs(group.paths, directRefs);

		// Resolve all nested references
		const allRefNames = resolveAllRefs(directRefs, definitions);

		// Extract filtered definitions
		const filteredDefs = {};
		allRefNames.forEach((name) => {
			if (definitions[name]) filteredDefs[name] = definitions[name];
		});

		// Include common response definitions
		const commonDefs = [
			"response.Response",
			"response.PageData",
			"response.Pagination",
		];
		commonDefs.forEach((name) => {
			if (definitions[name] && !filteredDefs[name]) {
				filteredDefs[name] = definitions[name];
			}
		});

		// Build output document
		const outputApi = {
			swagger,
			schemes,
			info: { ...info, title: `${info.title} - ${tag}` },
			host,
			basePath,
			paths: group.paths,
			definitions: filteredDefs,
			securityDefinitions,
		};

		// Write output file to the subdirectory
		const fileName = `${tag.toLowerCase().replace(/\s+/g, "-")}.json`;
		const filePath = path.join(subdir, fileName);
		fs.writeFileSync(filePath, JSON.stringify(outputApi, null, 2), "utf8");
		console.log(
			`✅ [${path.basename(subdir)}] Generated: ${fileName} (${Object.keys(group.paths).length} paths, ${Object.keys(filteredDefs).length} defs)`,
		);
	});

	console.log(
		`\n✨ [${path.basename(subdir)}] Split completed! Output dir: ${subdir}`,
	);
}

// Main: Find all subdirectories with api.v1.json
const entries = fs.readdirSync(DOC_DIR, { withFileTypes: true });
const targetSubdirs = entries
	.filter((entry) => entry.isDirectory())
	.map((entry) => path.join(DOC_DIR, entry.name))
	.filter((subdir) => fs.existsSync(path.join(subdir, "api.v1.json")));

if (targetSubdirs.length === 0) {
	console.error(
		"❌ No subdirectories containing api.v1.json found",
		DOC_DIR,
	);
	process.exit(1);
}

// Process each target subdirectory
for (const subdir of targetSubdirs) {
	console.log(`\n📂 Processing ${path.basename(subdir)} OpenAPI doc...`);
	processDocDir(subdir);
}

console.log("\n✅ All OpenAPI docs split successfully!");
