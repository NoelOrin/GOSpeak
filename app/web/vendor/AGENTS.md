# vendor/ — 第三方库 shim

本目录存放为优化打包而创建的第三方库"垫片"文件。垫片仅 re-export 实际用到的子集,避免全量包打入 bundle。

## cui-solid-icons shim

### 背景

`cui-solid-icons@1.0.9` 的子路径包(`./f7`、`./feather` 等)是单文件 minified ESM,内含千余个图标组件:

| 子路径 | 文件大小 | 图标数 |
|--------|----------|--------|
| `f7` | 2.1 MB | 1252 |
| `feather` | 160 KB | 287 |
| `tabler` | 3.7 MB | 4215 |
| `phosphor` | 5.7 MB | 6482 |

文件内组件共享 helper、minified 后导出名被混淆,且未标 `/* @__PURE__ */`。导致 rolldown/vite tree-shake 失败,整包打入 bundle。实测 `f7` 整包 1.91 MB(渲染后)进生产 chunk,但 `cui-solid` 实际只用 6 个图标。

### 现状

- `cui-solid-icons-feather.ts` — re-export 28 个 feather 图标(`cui-solid` 实际依赖)
- `cui-solid-icons-f7.ts` — re-export 6 个 f7 图标(`cui-solid` 实际依赖)

### 工作机制

`vite.config.ts` 的 `resolve.alias` 用正则 `^cui-solid-icons/f7$` 与 `^cui-solid-icons/feather$` 精确匹配子路径,重定向到本目录 shim。正则锚定 `$` 确保只 hijack 这两个子路径,不误伤 `cui-solid-icons` 主入口或其他 icon-set(bi/geo/ionicons/phosphor/tabler 仍走原包,全量但未引入所以不入 bundle)。

shim 是命名导出 `export { Xxx } from "cui-solid-icons/feather";`,tree-shake 友好 — 只用到的图标组件进 bundle,其余被删。

### 追加图标

当业务需要新图标(且来自 feather 或 f7 集)时:

1. 确认图标在原库导出名。查 `node_modules/.pnpm/cui-solid-icons@1.0.9/node_modules/cui-solid-icons/dist/<set>/<set>.min.esm.js` 末尾 `export { ... as Xxx }` 块,或用:
   ```bash
   grep -oE "as (Feather|F7)[A-Z][A-Za-z]+" node_modules/.pnpm/cui-solid-icons@1.0.9/node_modules/cui-solid-icons/dist/<set>/<set>.min.esm.js | sort -u
   ```
2. 在对应 shim 文件的 `export { ... } from "cui-solid-icons/<set>";` 列表里加一行 `Xxx,`。
3. 业务代码 import 路径不变,继续用 `import { Xxx } from "cui-solid-icons/<set>"`(alias 自动重定向)。

切勿直接 import 原 `cui-solid-icons/<set>` 路径绕过 alias 标注,否则回退到全量包。

### 其他 icon-set

若未来引入 `bi`/`geo`/`ionicons`/`phosphor`/`tabler`:

- 仅在确实引用时才需 shim(未引用时原包不入 bundle,无问题)。
- 复制 f7 shim 模式:新建 `vendor/cui-solid-icons-<set>.ts`,列实际依赖的图标。
- 在 `vite.config.ts` 的 `resolve.alias` 加对应正则 entry:`{ find: /^cui-solid-icons\/<set>$/, replacement: path.resolve(__dirname, "./vendor/cui-solid-icons-<set>.ts") }`。

### 验证

构建后查 `dist/stats.html`(visualizer 生成),确认:
- `cui-solid-icons/.../f7.min.esm.js` 的 `renderedLength` 应从 ~2 MB 降到几 KB。
- 或 `du -sh dist/chunks/ui.*.js` 对比改动前后。
