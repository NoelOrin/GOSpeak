# scripts/ — vite 插件

## cui-icons-plugin.mjs

`cuiIconsTreeShakePlugin` —— cui-solid-icons icon-set tree-shake 插件。

### 背景

`cui-solid-icons@1.0.9` 各 icon-set(f7/feather 等)是单文件 minified ESM,千余图标组件以 `function XX(i){...}` 声明 + 末尾 `export { XX as F7Yyy, ... }` 全量导出。rolldown 对这种模式保守,整包打入 bundle(f7 实测 2.0 MB rendered,但 cui-solid 实际只用 6 个 f7 图标)。

### 机制

插件 `transform` 钩子拦截原库文件(`cui-solid-icons/dist/<set>/<set>.min.esm.js`),把 export 块重写为只含"实际被引用"的图标别名。rolldown 据此删未引用的 `function` 声明(无副作用),实现 tree-shake。

- **不改 function 体**:无 shim 中转、无循环引用,runtime 正常(修复了之前 shim 方案 `ot is not defined` 的循环剪枝问题)。
- **引用集来源**:`buildStart`/`configureServer` 扫描 `node_modules/cui-solid/dist/cui.min.esm.js` + 业务 `src/**/*.{ts,tsx}` 的 `import { ... } from "cui-solid-icons/<set>"`,存模块级 Set,transform 时查。
- **别名匹配**:minified 别名可含 `$`(如 `x$ as F7Person`),正则用 `[\w$]+` 而非 `\w+`。
- **export 块定位**:平衡括号扫描(CRLF 兼容),找 `export { ... }` 块。

### 接入

`vite.config.ts` plugins 数组加 `cuiIconsTreeShakePlugin()`(已加在 tailwindcss 后)。

### 追加图标(零维护)

业务代码正常 `import { F7Xxx } from "cui-solid-icons/f7"`,无需改插件。dev/build 启动时自动重扫,export 块自动含新图标。

cui-solid 升级新增图标 import 同理自动收录。

### 新增 icon-set

插件对任意 set 通用(transform 正则匹配 `cui-solid-icons/dist/<set>/<set>.min.esm.js`)。业务或 cui-solid 引用新 set 时,扫描自动收录,无需改插件或 vite 配置。

### 验证

默认构建不输出 `stats.html`，验证树摇结果时显式开启：

```bash
BUILD_STATS=1 pnpm build
python3 -c "
import json,json.decoder as jd
h=open('dist/stats.html',encoding='utf-8').read()
i=h.find('const data = ')+len('const data = ')
d,_=jd.JSONDecoder().raw_decode(h[i:])
nm=d['nodeMetas']; np=d['nodeParts']
inv={v['metaUid']:k for k,v in np.items() if 'metaUid' in v}
for mid,meta in nm.items():
    p=meta.get('id','')
    if 'cui-solid-icons' in p:
        uid=inv.get(mid); part=np.get(uid,{}) if uid else {}
        print(p.split('/')[-1], 'rendered=', part.get('renderedLength',0))
"
```

预期:`f7.min.esm.js` ~8 KB(6 图标)、`feather.min.esm.js` ~13 KB(29 图标),原 2.0 MB / 116 KB。

### 局限

- 依赖库文件结构(`function XX` 声明 + 末尾 export 块)。库大版本升级若改结构需适配插件正则。
- 仅 tree-shake 命名导出的图标组件;若库内图标以其他形式(如对象属性)导出,需扩展 `rewriteExports`。
