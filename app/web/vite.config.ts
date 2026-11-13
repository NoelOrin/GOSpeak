import { defineConfig } from "vite";
import { devtools } from "@tanstack/devtools-vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { readFileSync } from "node:fs";
import path from "node:path";
import net from "node:net";
import { fileURLToPath } from "node:url";
import { visualizer } from "rollup-plugin-visualizer";

/**
 * 前端版本号：取仓库根 package.json 的 version。
 *
 * 与 scripts/check-version.mjs 的口径一致（根 package.json / 根 VERSION /
 * .github/versions/server.yaml 三者由 CI 强制相等），因此前端展示的版本
 * 与后端 buildinfo.Version 同源。读根 package.json 而不是 VERSION 文件，
 * 是因为 Dockerfile 的 web-builder 阶段只复制了 package.json，没有复制 VERSION。
 */
function readAppVersion(): string {
	try {
		const rootPkg = path.resolve(
			path.dirname(fileURLToPath(import.meta.url)),
			"../..",
			"package.json",
		);
		const { version } = JSON.parse(readFileSync(rootPkg, "utf8")) as {
			version?: string;
		};
		return version?.trim() || "dev";
	} catch {
		return "dev";
	}
}

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

  // 条件编译目标：GOSPEAK_TARGET=desktop 构建桌面端（桌面独享模块进包），
  // 默认 web（桌面独享模块在打包期整体剔除）。见 @gospeak/desktop 的 conditional.ts。
  const gospeakTarget: "web" | "desktop" =
    process.env.GOSPEAK_TARGET === "desktop" ? "desktop" : "web";

  const appVersion = readAppVersion();

  return {
    // 条件编译注入常量（app/web/types/gospeak-build.d.ts 提供类型）
    define: {
      __GOSPEAK_TARGET__: JSON.stringify(gospeakTarget),
      __GOSPEAK_DESKTOP__: String(gospeakTarget === "desktop"),
      __GOSPEAK_APP_VERSION__: JSON.stringify(appVersion),
    },
    base,
    plugins: [
      devtools({
        eventBusConfig: { port: devtoolsPort },
      }),
      tanstackRouter({
        target: "react",
        autoCodeSplitting: true,
        routesDirectory: "./src/pages", // 路由目录
        generatedRouteTree: "./src/routeTree.gen.ts",
        quoteStyle: "single",
        semicolons: false,
        routeFileIgnorePrefix: "-", // -开头的文件会被忽略
        routeFileIgnorePattern: "components", // 忽略遍历文件夹下的components 文件夹
      }),
      react(),
      tailwindcss(),
      // 打包结果可视化
      // 默认关闭，需要时 BUILD_STATS=1 pnpm build
      ...(isProduction && process.env.BUILD_STATS === "1"
        ? [
            {
              ...visualizer({
                filename: "dist/stats.html",
                title: "Bundle Analyzer",
                template: "treemap", // treemap, sunburst, network
                gzipSize: true,
                brotliSize: true,
                open: true,
              }),
              apply: "build",
            },
          ]
        : []),
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
        "/uploads": {
          target: "http://localhost:8998",
          changeOrigin: true,
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
          "!**/packages/components/src/**",
          "!**/packages/desktop/src/**",
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
      exclude: [
        "fsevents",
        "@gospeak/sfu-client",
        "@gospeak/components",
        // workspace 源码直引（不预打包），保证 define 注入的条件编译常量生效且支持 HMR
        "@gospeak/desktop",
      ],
      rolldownOptions: {
        define: {
          global: "globalThis",
        },
        treeshake: true,
      },
    },
    esbuild: {
      drop: isProduction ? ["console", "debugger"] : [],
      legalComments: "inline",
    },
    html: {
      minify: isProduction
        ? {
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
          }
        : false,
    },
    build: {
      target: "es2022",
      cssMinify: isProduction ? "lightningcss" : false,
      minify: isProduction ? "esbuild" : false,
      sourcemap: isProduction ? false : "inline",
      cssCodeSplit: true,
      reportCompressedSize: false,
      chunkSizeWarningLimit: 500, // 提高警告阈值 500 KB
      rollupOptions: {
        output: {
          manualChunks: (id: string) => {
            // 数组映射分包
            const chunkGroups = {
              vendor: ["/react/", "/react-dom/"],
              router: ["@tanstack/react-router"],
              ui: ["@radix-ui/", "clsx", "sonner", "lucide-react"],
            };

            // 包
            if (id.includes("node_modules")) {
              for (const [chunkName, dependencies] of Object.entries(chunkGroups)) {
                if (dependencies.some((dep) => id.includes(dep))) {
                  return chunkName;
                }
              }
            }

            // 对于不匹配任何条件的模块，返回 undefined 让 Rolldown 自动处理
            return undefined;
          },
          // 优化文件名
          assetFileNames: (assetInfo: { names?: string[] }) => {
            const name = assetInfo.names?.[0] ?? "";
            if (name.endsWith(".css")) {
              return "assets/[name].[hash].css";
            }
            return "assets/[name].[hash].[ext]";
          },
          chunkFileNames: "chunks/[name].[hash].js",
          entryFileNames: "entries/[name].[hash].js",
        },
        input: {
          index: "index.html",
        },
      },
    },
  };
});
