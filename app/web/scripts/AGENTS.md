# scripts/ — 构建辅助与组件工具脚本

本目录放与构建、组件基线相关的独立 `.mjs` 脚本，不参与运行时打包。

## Structure

```
scripts/
├── capture-storybook-baseline.mjs   # Storybook 组件级基线截图（历史对照工具，已归档）
├── extract-theme-tokens.mjs         # 主题 token 实测值提取（历史对照工具，已归档）
└── AGENTS.md
```

## capture-storybook-baseline.mjs

用 Playwright 为 `packages/components` 的 Storybook 生成组件级基线截图。前置 `cd packages/components && pnpm storybook --ci`（6006 端口），脚本拉 `http://localhost:6006/index.json`，对每个 story 在 `light` / `dark` 两个主题下以 400×240 @2x 截图，并写入 `.last-run` 时间戳。

```bash
# 在 app/web 下执行
node scripts/capture-storybook-baseline.mjs            # → packages/components/storybook-baseline
node scripts/capture-storybook-baseline.mjs --actual   # → packages/components/storybook-actual
```

用于主题/样式改动后的视觉回归对照（历史对照工具，已归档）：`baseline` 为基准，`--actual` 生成当前实拍再比对。

## extract-theme-tokens.mjs

从运行中的迁移前旧版前端（默认 `http://localhost:3000`，可传 `baseUrl`）读取一组主题 CSS 变量（`--color-base-*` / `--color-primary` / `--radius-*` / `--border` 等）的计算值，在 `acid` / `dark` 主题下打印 `theme.css` 片段，供迁移时人工对齐 `packages/components/src/theme.css` 的 token。

```bash
node scripts/extract-theme-tokens.mjs [baseUrl]
```

属 React 19 迁移遗留的一次性历史对照工具，已归档，需旧前端仍在运行；当前应用的主题 token 以 `packages/components/src/theme.css` 为准。

## 构建产物分析（BUILD_STATS）

`vite.config.ts` 里的 `rollup-plugin-visualizer` 默认关闭，仅在生产构建且 `BUILD_STATS=1` 时以 `apply: "build"` 注入，输出 `dist/stats.html`（treemap 模板，含 gzip / brotli 体积，构建后自动打开）。需要分析分包时显式开启：

```bash
BUILD_STATS=1 pnpm build
```

`stats.html` 内嵌 `const data = ...` 的 JSON，可直接解析各模块的 `renderedLength`。下例按关键字过滤（留空则打印全部），把 `key` 换成关心的包名即可：

```bash
python3 -c "
import json,json.decoder as jd
h=open('dist/stats.html',encoding='utf-8').read()
i=h.find('const data = ')+len('const data = ')
d,_=jd.JSONDecoder().raw_decode(h[i:])
nm=d['nodeMetas']; np=d['nodeParts']
inv={v['metaUid']:k for k,v in np.items() if 'metaUid' in v}
key='react'  # 换成关心的模块名，置空则打印全部
for mid,meta in nm.items():
    p=meta.get('id','')
    if key and key not in p: continue
    uid=inv.get(mid); part=np.get(uid,{}) if uid else {}
    print(p.split('/')[-1], 'rendered=', part.get('renderedLength',0))
"
```

`vite.config.ts` 的 `manualChunks` 已按 `vendor`（react / react-dom）、`router`（@tanstack/react-router）、`ui`（@radix-ui/ / clsx / sonner / lucide-react）分组，可据此核对各分包体积。

## 历史说明（已归档，非当前约束）

`cui-icons-plugin.mjs` 及配套的 `cui-solid-icons` tree-shake 插件已随 React 19 迁移移除：`cui-solid` / `cui-solid-icons` 依赖已删除，`vite.config.ts` 中不再注册该插件。此段仅为历史说明，**不应再视为当前构建约束**，当前不应实现 `cui-icons-plugin` 相关逻辑。当前图标统一使用 `lucide-react`，由打包器原生 tree-shake，无需专用图标插件；原文档中针对 `cui-solid-icons` 的 export 块重写、`buildStart` 扫描与树摇验证逻辑均不再适用。
