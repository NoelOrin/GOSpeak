import { defineConfig } from 'vite'
import { devtools } from '@tanstack/devtools-vite'
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import solidPlugin from 'vite-plugin-solid'
import tailwindcss from '@tailwindcss/vite'
import path from "node:path";
import { execSync } from "node:child_process";
import { spawn } from "child_process";
import { visualizer } from "rollup-plugin-visualizer";

// @ts-ignore
export default defineConfig(({ mode }) => {

  const base = "/";
  const isProduction = mode === "production";

  return {
    base,
    plugins: [
      devtools(), 
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
        configureServer(server) {
          if (process.env.NODE_ENV === "development") {
            const tsr = spawn("npx", ["tsr", "watch"], {
              stdio: "inherit", // 让日志输出到控制台
              shell: true,
              cwd: process.cwd(),
            });
            server.httpServer?.on("close", () => {
              tsr.kill();
            });

            tsr.on("error", (err) => {
              console.error(err);
            });
          }
        },
      },
      // 打包结果可视化
      ...(isProduction ? [{
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
      open: true,
      host: true,
      proxy: {
        "/api/v1": {
          target: "http://localhost:8998",
          changeOrigin: true,
        },
        "/socket.io": {
          target: "http://localhost:8998",
          changeOrigin: true,
          ws: true,
        },
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
        "@tanstack/solid-router",
        "@tanstack/router-plugin/vite",
        "cui-solid",
        "clsx",
        "daisyui",
      ],
      exclude: ['fsevents'],
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
      // 生产环境优化
      ...(isProduction && {
        minify: true,
        drop: ['console', 'debugger'],
        pure: ['console.log', 'console.info', 'console.debug'],
      }),
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
          manualChunks: (id) => {
            // 数组映射分包
            const chunkGroups = {
              vendor: ['solid-js', 'solid-js/store', 'solid-js/web'],
              router: ['@tanstack/solid-router'],
              ui: ['cui-solid', 'clsx', 'daisyui','cui-solid-icons'],
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
          assetFileNames: (assetInfo) => {
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
      brotliSize: isProduction,
     },
  };
});
