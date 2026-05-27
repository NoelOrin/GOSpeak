---
name: openapi-to-service
description: Use when generating TypeScript API service files from remote OpenAPI/Swagger endpoints (SCADA: http://119.91.128.215:29429/swagger/doc.json, USER: http://119.91.128.215:27147/swagger/doc.json), or from JSON docs in .agents/skills/openapi-to-service/doc/scada/ or .agents/skills/openapi-to-service/doc/user/ for the scada-web project.
---

# OpenAPI to Service

## Overview
Convert an OpenAPI/Swagger 2.0 JSON file into a typed TypeScript API service file using the project's `apiClient` and existing conventions. SCADA docs are from `.agents/skills/openapi-to-service/doc/scada/`, USER docs from `.agents/skills/openapi-to-service/doc/user/`, with generated services in `src/api/services/scada/` and `src/api/services/user/` respectively.

The full pipeline: **fetch remote spec → split by tag → generate per-tag service**. The split script `index.js` (co-located with this skill) processes `api.v1.json` in `doc/scada/` (SCADA) and `doc/user/` (USER), producing one JSON per API tag in each subdirectory. Each per-tag JSON is then converted to a service file in the corresponding services subdirectory.

> **Note**: All doc files live under `.agents/skills/openapi-to-service/doc/`. Service output files remain at `src/api/services/scada/` and `src/api/services/user/`.

## Project Conventions (scada-web)

Each service file follows this exact structure:

**SCADA modules** (`src/api/services/scada/`):
```
import apiClient from "../../apiClient";
import type { listParamsType, PageData } from "#/api";

// 1. Domain types (from definitions)
export interface Entity { ... }
export interface CreateEntityRequest { ... }

// 2. API URL enum
export enum EntityApi {
    List = "/api/v1/entities",
    GetById = "/api/v1/entities/{id}",
    ...
}

// 3. API functions
const listEntities = (params: listParamsType) =>
    apiClient.get<PageData<Entity>>({ url: EntityApi.List, params });

const getEntityById = (id: string) =>
    apiClient.get<Entity>({ url: EntityApi.GetById.replace("{id}", id) });

// 4. Default export
export default { listEntities, getEntityById, ... };
```

**USER modules** (`src/api/services/user/`):
```
import { userApiClient as apiClient } from "../../apiClient";
import type { listParamsType, PageData } from "#/api";

// 1. Domain types (from definitions)
export interface Entity { ... }
export interface CreateEntityRequest { ... }

// 2. API URL enum
export enum EntityApi {
    List = "/api/v1/entities",
    GetById = "/api/v1/entities/{id}",
    ...
}

// 3. API functions
const listEntities = (params: listParamsType) =>
    apiClient.get<PageData<Entity>>({ url: EntityApi.List, params });

const getEntityById = (id: string) =>
    apiClient.get<Entity>({ url: EntityApi.GetById.replace("{id}", id) });

// 4. Default export
export default { listEntities, getEntityById, ... };
```

## Conversion Rules

### TypeScript Interface Generation

| JSON Swagger Type | TypeScript Type |
|---|---|
| `"type": "string"` | `string` |
| `"type": "integer"` / `"number"` | `number` |
| `"type": "boolean"` | `boolean` |
| `"type": "array"` + `items` | `T[]` |
| `"type": "object"` + `properties` | interface with properties |
| `"$ref"` | type name extracted from `#/definitions/prefix.Name` |
| `"allOf"` with `$ref` | inline the referenced type |
| no explicit type + `properties` | object/interface |
| `"type": "string"` + `enum` | union: `"val1" \| "val2"` |
| `"description"` field | JSDoc `/** desc */` |

**Naming conventions for types:**
- Strip prefix: `dto.CreateDeviceTypeRequest` → `CreateDeviceTypeRequest`
- Strip prefix: `model.DeviceType` → `DeviceType`
- Strip prefix: `response.PageData` → use `PageData` from `#/api` (do NOT redefine)
- Strip prefix: `response.Response` → use `Response` (do NOT redefine unless needed)

**Skip redefining these common types** (import from `#/api` instead):
- `response.PageData` → `PageData<T>` from `#/api`
- `response.Pagination` → from `#/api`
- `response.Response` → from `#/api`
- `response.BizCode` → skip entirely (enum only)

### API URL Enum Generation

Enum name: `{Tag}Api` (e.g., `DeviceTypeApi`, `ChannelApi`)

For each path in `paths`:
- Derive member name from HTTP method + summary (e.g., `GET /api/v1/device-types` with summary "设备类型列表" → `GetDeviceTypes`)
- Value = the URL path
- Replace `{param}` placeholders with `{id}` for standard CRUD on `/{id}` paths
- Deduplicate identical URLs with the same method → single enum member
- For paths containing `{id}` with specific actions (`/enable`, `/disable`), create separate members: `EnableDeviceType`, `DisableDeviceType`

### API Function Generation

Map each path+method to a function:

| Pattern | Signature | Example |
|---------|-----------|---------|
| GET list (query params) | `(params: listParamsType)` | `const list = (params) => apiClient.get<PageData<T>>({ url, params })` |
| GET by id (path param) | `(id: string)` | `const getById = (id) => apiClient.get<T>({ url: ...replace("{id}", id) })` |
| POST create (body) | `(data: CreateReq)` | `const create = (data) => apiClient.post<T>({ url, data })` |
| PUT update (id + body) | `(id: string, data: UpdateReq)` | `const update = (id, data) => apiClient.put<T>({ url: ...replace("{id}", id), data })` |
| DELETE by id | `(id: string)` | `const remove = (id) => apiClient.delete<Response>({ url: ...replace("{id}", id) })` |
| POST action (id only) | `(id: string)` | `const action = (id) => apiClient.post<Response>({ url: ...replace("{id}", id) })` |
| POST batch | `(data: BatchReq)` | `const batch = (data) => apiClient.post<Response>({ url, data })` |

**Response types for `apiClient.get/post/put/delete<T>()`:**
- List endpoints: `T = PageData<Entity>`
- Single entity endpoints: `T = Entity`
- Action endpoints (enable/disable/delete): `T = Response`
- The `apiClient` interceptor unwraps `Result<T>`, so the generic is the unwrapped type

**Function naming:**
- camelCase derived from `summary` field: `"设备类型列表"` → `getDeviceTypes`, `"创建设备类型"` → `createDeviceType`
- For CRUD operations: use `get`/`create`/`update`/`delete` prefix
- For actions: use action verb: `enable`/`disable`/`batchDelete`

**Path parameter replacement:**
- Replace standard `{id}` with the actual `id` argument
- For other path params (e.g., `{actionId}`), add as separate function arguments

### Response Type Selection

Use the response type from the operation's response schema:
- If response schema is `$ref: #/definitions/response.Response` and description mentions data, determine the actual data type from context
- For list GET: always use `PageData<Entity>`
- For single GET: use `Entity`
- For POST (create): use `Entity`
- For PUT (update): use `Entity`
- For DELETE: use `Response`
- For action POST (enable/disable): use `Response`

## Procedure

### ⚠️ GATE: Select target module(s) (MANDATORY)

> The user MUST specify which module(s) to update: **one**, **multiple** (comma-separated), or **all**.
>
> Always prompt the user to specify their selection if they haven't already.

**Available modules (tags):**

#### SCADA Modules (from `doc/scada/`)
| Tag | JSON File | Output File |
|-----|-----------|-------------|
| `Callback` | `doc/scada/callback.json` | `scada/callbackService.ts` |
| `Channel` | `doc/scada/channel.json` | `scada/channelService.ts` |
| `DeviceType` | `doc/scada/devicetype.json` | `scada/deviceTypeService.ts` |
| `DeviceInstance` | `doc/scada/deviceinstance.json` | `scada/deviceInstanceService.ts` |
| `FlowDefinition` | `doc/scada/flowdefinition.json` | `scada/flowDefinitionService.ts` |
| `FlowInstance` | `doc/scada/flowinstance.json` | `scada/flowInstanceService.ts` |
| `Health` | `doc/scada/health.json` | `scada/healthService.ts` |
| `History` | `doc/scada/history.json` | `scada/historyService.ts` |
| `ParamDictionary` | `doc/scada/paramdictionary.json` | `scada/paramDictionaryService.ts` |
| `PhaseTemplate` | `doc/scada/phasetemplate.json` | `scada/phaseTemplateService.ts` |
| `Recipe` | `doc/scada/recipe.json` | `scada/recipeService.ts` |
| `Step` | `doc/scada/step.json` | `scada/stepService.ts` |

#### USER Modules (from `doc/user/`)
> Fetch the USER doc first using Step 0 to list available tags. Example entries:
| Tag | JSON File | Output File |
|-----|-----------|-------------|
| `User` | `doc/user/user.json` | `user/userService.ts` |
| `Role` | `doc/user/role.json` | `user/roleService.ts` |

**How to ask:**
```
I need to generate service(s) from the OpenAPI spec.
Which module(s) would you like to update? You can choose:
- Single module: `DeviceType`
- Multiple modules (comma-separated): `DeviceType, Channel, FlowDefinition`
- All modules: `all` / `全部` / `everything`

Available SCADA modules: Callback | Channel | DeviceType | DeviceInstance | FlowDefinition | FlowInstance | Health | History | ParamDictionary | PhaseTemplate | Recipe | Step
Available USER modules: (fetched from USER doc, e.g., User | Role | ...)
```

> When user selects "all", process every available tag from both SCADA and USER docs.
> When user selects multiple modules, process only the specified tags (e.g., `DeviceType, Channel` → process those two tags only).

**Processing rules for module selection:**
- **Single module**: Follow Steps 1-7 directly for the selected tag.
- **Multiple modules**: Collect all selected tags, then loop over each tag and run Steps 1-7 sequentially or in parallel (use parallel subagents if 2+ independent tags).
- **All modules**: Collect every available tag from `doc/scada/*.json` and `doc/user/*.json` under the skill directory (run Step 0 split if these files are missing), then loop over every tag and run Steps 1-7 for each.

### Step 0: Fetch & Split (from remote)

Skip this step if you are working from existing files in `doc/scada/` or `doc/user/` (relative to the skill directory).

Run these commands from the **project root** (`scada-web/`):

```bash
# 1. Fetch SCADA OpenAPI spec (replace existing if updating)
mkdir -p .agents/skills/openapi-to-service/doc/scada
curl -o .agents/skills/openapi-to-service/doc/scada/api.v1.json http://119.91.128.215:29429/swagger/doc.json

# 2. Fetch USER OpenAPI spec (replace existing if updating)
mkdir -p .agents/skills/openapi-to-service/doc/user
curl -o .agents/skills/openapi-to-service/doc/user/api.v1.json http://119.91.128.215:27147/swagger/doc.json

# 3. Split by tag — produces one JSON per tag in doc/scada/ and doc/user/ (relative to skill dir)
node .agents/skills/openapi-to-service/index.js
```

The script reads `doc/scada/api.v1.json` and `doc/user/api.v1.json`, groups paths by `tags`, resolves `$ref` chains and writes per-tag files into the respective subdirectory (e.g., `doc/scada/devicetype.json`, `doc/user/auth.json`).

> If the user selected "all" modules, you MUST run the split script (step 3) to ensure all per-tag JSON files are generated, then collect all `.json` files in `doc/scada/` and `doc/user/` to get the full list of tags to process.

After splitting, proceed to convert the selected per-tag JSON:

### Step 1: Read & Parse

Read the input JSON file from `doc/scada/{tag}.json` (SCADA modules) or `doc/user/{tag}.json` (USER modules), relative to the skill directory `.agents/skills/openapi-to-service/`.

**Note**: The file path determines the module type:
- `doc/scada/*.json` → SCADA module → import: `import apiClient from "../../apiClient";`
- `doc/user/*.json` → USER module → import: `import { userApiClient as apiClient } from "../../apiClient";`

### Step 2: Extract Tag

Extract `tag` from `info.title` (e.g., `"SCADA Service API - DeviceType"` → tag = `"DeviceType"`, `"User Service API - User"` → tag = `"User"`)

### Step 3: Generate Interfaces

Parse `definitions` and generate TypeScript interfaces for domain types using the Conversion Rules below.

### Step 4: Generate URL Enum

Generate the `{Tag}Api` enum from `paths`.

### Step 5: Generate Functions

Generate typed API functions from `paths` using the conversion rules.

### Step 6: Generate Export

Generate default export object.

### Step 7: Write File

- **SCADA modules**: Write to `src/api/services/scada/{camelName}Service.ts`
  - Import: `import apiClient from "../../apiClient";`
- **USER modules**: Write to `src/api/services/user/{camelName}Service.ts`
  - Import: `import { userApiClient as apiClient } from "../../apiClient";`

Both use: `import type { listParamsType, PageData } from "#/api";`

Then follow the structure:
1. Import statements (use correct import above based on module type)
2. Domain types (interfaces from definitions)
3. `{Tag}Api` enum from paths
4. API functions (camelCase, use `apiClient.get/post/put/delete<T>()`)
5. Default export object

### Output File Name

`src/api/services/{module}/{camelName}Service.ts` where `{module}` is `scada` or `user` based on the doc source, and `{camelName}` is the tag in camelCase:
- SCADA modules (from `doc/scada/` under the skill directory):
  - `DeviceType` → `scada/deviceTypeService.ts`
  - `Channel` → `scada/channelService.ts`
  - `DeviceInstance` → `scada/deviceInstanceService.ts`
  - `FlowDefinition` → `scada/flowDefinitionService.ts`
  - `FlowInstance` → `scada/flowInstanceService.ts`
  - `Health` → `scada/healthService.ts`
  - `History` → `scada/historyService.ts`
  - `ParamDictionary` → `scada/paramDictionaryService.ts`
  - `PhaseTemplate` → `scada/phaseTemplateService.ts`
  - `Recipe` → `scada/recipeService.ts`
  - `Step` → `scada/stepService.ts`
- USER modules (from `doc/user/` under the skill directory):
  - (List tags from USER doc after fetching, e.g., `User` → `user/userService.ts`)

## Common Mistakes

- **Bulk generation**: Allowed only when user explicitly requests "all", "全部", multiple comma-separated modules, or "everything". Do not assume bulk generation. When processing multiple/all modules, iterate over each selected tag individually and generate corresponding service files.
- **Redefining common types**: `PageData`, `Pagination`, `Response` are available from `#/api` — only define locally if they don't exist
- **Missing `PageData` wrapper**: List endpoints should return `PageData<T>`, not bare `T[]`
- **Wrong response type for `apiClient`**: Remember apiClient unwraps `Result<T>`, so the generic is the business data type, not the response wrapper
- **Forgetting `.replace("{id}", id)`**: Path parameters must be substituted
- **Incorrect function naming**: Use the `summary` field to derive the function name, not the URL path
- **Missing enum for batch/action endpoints**: Even if they share URL paths, give them distinct enum member names
- **Skipping the fetch step**: When upstream docs are outdated, always re-fetch before generating
- **Not running index.js after fetch**: The split script must be re-run every time `api.v1.json` in `doc/scada/` or `doc/user/` is updated
- **Editing api.v1.json manually**: Always fetch from the upstream servers (SCADA/USER); don't edit the JSON by hand
