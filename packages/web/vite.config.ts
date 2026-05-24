import { defineConfig } from 'vite'
import { devtools } from '@tanstack/devtools-vite'
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import solidPlugin from 'vite-plugin-solid'
import tailwindcss from '@tailwindcss/vite'
import path from "node:path";
import { execSync } from "node:child_process";
import { spawn } from "child_process";

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
      solidPlugin(), 
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
    ],

    server: {
      open: true,
      host: true,
      proxy: {
        "/xxx": {
          target: "https://xxx",
          changeOrigin: true,
          rewrite: (path) => {
            return path.replace(/^\/xxx/, "");
          },
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
      ],
    },
    esbuild: {
      drop: isProduction ? ["console", "debugger"] : [],
    },
    build: {
      minify: "esbuild",
      sourcemap: true,
      cssCodeSplit: true,
      reportCompressedSize: false,
      chunkSizeWarningLimit: 1000, // 提高警告阈值到 1000 KB
      rollupOptions: {
        output: {
          manualChunks: {
           
          },
        },
        input: {
          index: "index.html",
        },
      },
    },
  };
});
