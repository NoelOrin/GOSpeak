import { defineConfig } from 'vite'
import { devtools } from '@tanstack/devtools-vite'
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import solidPlugin from 'vite-plugin-solid'
import tailwindcss from '@tailwindcss/vite'
import path from "node:path";
import net from "node:net";
import { execSync } from "node:child_process";
import { spawn } from "child_process";
import { visualizer } from "rollup-plugin-visualizer";
import { cuiIconsTreeShakePlugin } from "./scripts/cui-icons-plugin.mjs";

function findAvailablePort(startPort: number): Promise<number> {
  if (startPort >= 65536) {
    return Promise.resolve(42069);
  }

  return new Promise((resolve) => {
    const server = net.createServer();
    server.listen(startPort, () => {
      server.close(() => resolve(startPort));
    });
    server.on("error", () => resolve(findAvailablePort(startPort + 1)));
  });
}

// @ts-ignore
export default defineConfig(async ({ mode }) => {

  const base = "/";
  const isProduction = mode === "production";
  const devtoolsPort = isProduction ? 42069 : await findAvailablePort(42069);

  return {
    base,
    plugins: [
      devtools({
        eventBusConfig: { port: devtoolsPort },
      }),
     tanstackRouter({
        target: "solid",
        autoCodeSplitting: true,
        routesDirectory: "./src/pages", // 路由目录
        generatedRouteTree: "./src/routeTree.gen.ts",
        quoteStyle: "single",
        semicolons: false,
        routeFileIgnorePrefix: "-", // -开头的文件会被忽略
        routeFileIgnorePattern: "components", // 忽略遍历文件夹下的components 文件夹
      }),
      solidPlugin({
        // HMR 优化选项
        hot: true,
        solid: {
          moduleName: "solid-js/web",
          generate: "dom",
          hydratable: false, // 开发环境下禁用hydrate以提高HMR速度
        }
      }), 
      tailwindcss(),
      // cui-solid-icons icon-set tree-shake: 重写 export 块只保留被引用图标,
      // 让 rolldown 删未引用 function 声明。详见 scripts/cui-icons-plugin.mjs。
      cuiIconsTreeShakePlugin(),
      // {
      //   name: "lefthook-plug",
      //   configEnvironment(_name, _config, env) {
      //     console.log(env.mode, "---");
      //     if (env.mode === "development") {
      //       execSync("npx lefthook install");
      //     }
      //   },
      // },
      {
        name: "gen-tanstack-route",
        configureServer(server: { httpServer: { on: (arg0: string, arg1: () => void) => void; }; }) {
          if (process.env.NODE_ENV === "development") {
            const tsr = spawn("npx", ["tsr", "watch"], {
              stdio: "inherit", // 让日志输出到控制台
              shell: true,
              cwd: process.cwd(),
            });
            if (server.httpServer) server.httpServer.on("close", () => tsr.kill());

            tsr.on("error", (err) => {
              console.error(err);
            });
          }
        },
      },
      // 打包结果可视化
      // 默认关闭，需要时 BUILD_STATS=1 pnpm build
      ...(isProduction && process.env.BUILD_STATS === "1" ? [{
        ...visualizer({
          filename: "dist/stats.html",
          title: "Bundle Analyzer",
          template: "treemap", // treemap, sunburst, network
          gzipSize: true,
          brotliSize: true,
          open: true,
        }),
        apply: "build",
      }] : []),
    ],
    // 代理
    server: {
      port: 3000,
      strictPort: false,
      open: true,
      host: "0.0.0.0",
      proxy: {
        "/api/v1": {
          target: "http://localhost:8998",
          changeOrigin: true,
          ws: true,
        },
        "/ws": {
          target: "http://localhost:8998",
          changeOrigin: true,
          ws: true,
        },
        "/rtc/v1": {
          target: "http://localhost:1985",
          changeOrigin: true,
          configure: (proxy) => {
            proxy.on("proxyReq", (proxyReq) => {
              proxyReq.setHeader("Connection", "close");
            });
          },
        },
      },
      // 监听 symlinked workspace 包源码以实现 HMR
      watch: {
        ignored: [
          "!**/packages/sfu-client/src/**",
          "!**/node_modules/@go-rtc/sfu-client/**",
        ],
      },
    },
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
        "#": path.resolve(__dirname, "./types"),
      },
    },
    optimizeDeps: {
      include: [
        "solid-js",
        "solid-js/store",
        "solid-js/web",
        "@tanstack/router-plugin/vite",
        "cui-solid",
        "clsx",
        "lucide-solid/icons/shield-check",
        "lucide-solid/icons/server-cog",
        "lucide-solid/icons/gavel",
        "lucide-solid/icons/hard-drive",
        "lucide-solid/icons/refresh-ccw",
        "lucide-solid/icons/save",
        "lucide-solid/icons/shield-x",
        "lucide-solid/icons/plus",
        "lucide-solid/icons/trash-2",
        "lucide-solid/icons/pencil",
        "lucide-solid/icons/check",
        "lucide-solid/icons/x",
        "lucide-solid/icons/user-x",
        "lucide-solid/icons/user-check",
        "lucide-solid/icons/clock",
        "lucide-solid/icons/infinity",
      ],
      exclude: ['fsevents', '@gospeak/sfu-client'],
      rolldownOptions: {
        define: {
          global: "globalThis",
        },
        treeshake: true,
      },
    },
    esbuild: {
      drop: isProduction ? ["console", "debugger"] : [],
      jsx: "preserve",
      legalComments: "inline",
    },
    html: {
      minify: isProduction ? {
        collapseBooleanAttributes: true,
        decodeEntities: true,
        minifyCSS: true,
        minifyJS: true,
        processConditionalComments: true,
        removeEmptyAttributes: true,
        removeRedundantAttributes: true,
        trimCustomFragments: true,
        useShortDoctype: true,
        removeComments: true,
        preserveLineBreaks: false,
        collapseWhitespace: true,
      } : false,
    },
    build: {
      target: 'es2022',
      cssMinify: isProduction ? 'lightningcss' : false,
      minify: isProduction ? "esbuild" : false,
      sourcemap: isProduction ? false : 'inline',
      cssCodeSplit: true,
      reportCompressedSize: false,
      chunkSizeWarningLimit: 500, // 提高警告阈值 500 KB
      rollupOptions: {
        output: {
          manualChunks: (id: string | string[]) => {
            // 数组映射分包
            const chunkGroups = {
              // path-anchored: pnpm peer-dep paths like lucide-solid@..._solid-js@1.9.13 contain bare "solid-js"
              vendor: ['/solid-js/'],
              router: ['@tanstack/solid-router'],
              ui: ['cui-solid', 'clsx', 'daisyui'],
              lucide: ['lucide-solid'],
            };

            // 包
            if (id.includes('node_modules')) {
              for (const [chunkName, dependencies] of Object.entries(chunkGroups)) {
                if (dependencies.some(dep => id.includes(dep))) {
                  return chunkName;
                }
              }
            }
            
            // 对于不匹配任何条件的模块，返回 undefined 让 Rolldown 自动处理
            return undefined;
          },
          // 优化文件名
          assetFileNames: (assetInfo: { name: string; }) => {
            if (assetInfo.name?.endsWith('.css')) {
              return 'assets/[name].[hash].css';
            }
            return 'assets/[name].[hash].[ext]';
          },
          chunkFileNames: 'chunks/[name].[hash].js',
          entryFileNames: 'entries/[name].[hash].js',
        },
        input: {
          index: "index.html",
        },
      },
      // 启用 brotli 压缩
    },
  };
});
