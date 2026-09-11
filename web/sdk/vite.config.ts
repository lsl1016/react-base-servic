import path from "path";
import type { IncomingMessage } from "http";
import { codeInspectorPlugin } from "code-inspector-plugin";
import Icons from "unplugin-icons/vite";
import type { PluginOption, UserConfig } from "vite";
import { defineConfig } from "vitest/config";
import dts from "vite-plugin-dts";
import solidPlugin from "vite-plugin-solid";
import { SDK_CSS_FILE, SDK_JS_FILE, SDK_VERSION } from "./build/constants";
const sharedPlugins = [
  Icons({ compiler: "solid" }) as PluginOption,
  solidPlugin(),
];

const devPlugins = [
  Icons({ compiler: "solid" }) as PluginOption,
  codeInspectorPlugin({
    bundler: "vite",
    launchType: "exec",
    needEnvInspector: false,
    hideConsole: false,
    hotKeys: ["ctrlKey"],
  }) as PluginOption,
  solidPlugin(),
];

const sharedCSS = {
  preprocessorOptions: {
    scss: {
      api: "modern-compiler" as const,
    },
  },
};

const reactServiceTarget = process.env.REACT_SERVICE_TARGET ?? "http://127.0.0.1:8080";
const browserDevAssetBase = "/react-base-service/react/sdk";

const getRequestOrigin = (req: IncomingMessage) => {
  const protocol = req.headers["x-forwarded-proto"] ?? "http";
  const host = req.headers.host ?? "127.0.0.1:5173";
  return `${Array.isArray(protocol) ? protocol[0] : protocol}://${host}`;
};

const browserDevAssetPlugin = (): PluginOption => ({
  name: "agent-browser-dev-assets",
  configureServer(server) {
    server.middlewares.use((req, res, next) => {
      const pathname = req.url?.split("?")[0];

      if (pathname === `${browserDevAssetBase}/${SDK_CSS_FILE}`) {
        res.statusCode = 200;
        res.setHeader("Access-Control-Allow-Origin", "*");
        res.setHeader("Content-Type", "text/css; charset=utf-8");
        res.end("");
        return;
      }

      if (pathname === `${browserDevAssetBase}/${SDK_JS_FILE}`) {
        const origin = getRequestOrigin(req);
        res.statusCode = 200;
        res.setHeader("Access-Control-Allow-Origin", "*");
        res.setHeader("Content-Type", "application/javascript; charset=utf-8");
        res.end([
          `import * as sdk from "${origin}/browser.ts";`,
          "const api = { ...sdk, createClient: sdk.createAgentClient };",
          "window.AgentWebSDK = { ...(window.AgentWebSDK ?? {}), ...api };",
          `export * from "${origin}/browser.ts";`,
          "export const createClient = sdk.createAgentClient;",
          "",
        ].join("\n"));
        return;
      }

      next();
    });
  },
});

export default defineConfig(({ mode }): UserConfig => {
  const baseConfig: UserConfig = {
    plugins: [...sharedPlugins],
    css: sharedCSS,
    test: {
      environment: "jsdom",
    },
  };

  if (mode === "browser") {
    return {
      ...baseConfig,
      plugins: [...devPlugins, browserDevAssetPlugin()],
      server: {
        cors: true,
        headers: {
          "Access-Control-Allow-Origin": "*",
        },
        proxy: {
          "/react-base-service": {
            target: reactServiceTarget,
            changeOrigin: true,
            ws: true,
          },
        },
      },
      build: {
        lib: {
          entry: path.resolve(__dirname, "browser.ts"),
          name: "AgentWebSDK",
          fileName: () => "web-agent",
          formats: ["iife"],
        },
        outDir: "dist",
        emptyOutDir: false,
        rollupOptions: {
          output: {
            inlineDynamicImports: true,
            entryFileNames: SDK_JS_FILE,
            assetFileNames: (assetInfo) => {
              if (assetInfo.name?.endsWith(".css")) {
                return SDK_CSS_FILE;
              }
              return `${SDK_VERSION}/[name][extname]`;
            },
          },
        },
      },
    };
  }

  return {
    ...baseConfig,
    plugins: [
      ...sharedPlugins,
      dts({
        entryRoot: ".",
        outDir: "dist",
        include: [
          "index.ts",
          "browser.ts",
          "build/constants.ts",
          "analytics/**/*.ts",
          "protocol/**/*.ts",
          "client/**/*.ts",
          "session/**/*.ts",
          "storage/**/*.ts",
          "runtime/**/*.ts",
          "tools/**/*.ts",
          "ui/**/*.ts",
          "ui/**/*.tsx",
          "types/**/*.d.ts",
        ],
        exclude: ["__tests__", "dist", "node_modules"],
      }),
    ],
    build: {
      lib: {
        entry: path.resolve(__dirname, "index.ts"),
        name: "AgentWebSDK",
        fileName: () => "index",
        formats: ["es"],
      },
      outDir: "dist",
      emptyOutDir: true,
      rollupOptions: {
        output: {
          entryFileNames: "index.js",
          assetFileNames: (assetInfo) => {
            if (assetInfo.name?.endsWith(".css")) {
              return "style.css";
            }
            return "[name][extname]";
          },
        },
      },
    },
  };
});
