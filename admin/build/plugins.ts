import vue from "@vitejs/plugin-vue";
import vueJsx from "@vitejs/plugin-vue-jsx";
import AutoImport from "unplugin-auto-import/vite";
import { ElementPlusResolver } from "unplugin-vue-components/resolvers";
import Components from "unplugin-vue-components/vite";
import { codeInspectorPlugin } from "code-inspector-plugin";
import { visualizer } from "rollup-plugin-visualizer";
import vueSetupExtend from "unplugin-vue-setup-extend-plus/vite";
import { PluginOption } from "vite";
import viteCompression from "vite-plugin-compression";
import { createHtmlPlugin } from "vite-plugin-html";
import { VitePWA } from "vite-plugin-pwa";
import NextDevTools from "vite-plugin-vue-devtools";

/**
 * 创建 vite 插件
 * @param viteEnv
 */
export const createVitePlugins = (viteEnv: ViteEnv): (PluginOption | PluginOption[])[] => {
  const { VITE_GLOB_APP_TITLE, VITE_REPORT, VITE_DEVTOOLS, VITE_PWA, VITE_CODEINSPECTOR } = viteEnv;
  return [
    vue(),
    // element-plus 按需引入（模板组件 Components resolver 自动注册+样式自动引入；
    // 命令式 API（ElMessage/ElMessageBox/ElNotification）由 AutoImport resolver 补样式——
    // main.ts 全量 import ElementPlus 与全量 CSS 移除）
    AutoImport({ resolvers: [ElementPlusResolver()] }),
    Components({ resolvers: [ElementPlusResolver()] }),
    // vue 可以使用 jsx/tsx 语法
    vueJsx(),
    // devTools
    VITE_DEVTOOLS && NextDevTools({ launchEditor: "code" }),
    // name 可以写在 script 标签上
    vueSetupExtend({}),
    // 创建打包压缩配置
    createCompression(viteEnv),
    // 注入变量到 html 文件
    createHtmlPlugin({
      minify: true,
      inject: {
        data: { title: VITE_GLOB_APP_TITLE }
      }
    }),
    // vitePWA
    VITE_PWA && createVitePwa(viteEnv),
    // 是否生成包预览，分析依赖包大小做优化处理
    VITE_REPORT && (visualizer({ filename: "stats.html", gzipSize: true, brotliSize: true }) as PluginOption),
    // 自动 IDE 并将光标定位到 DOM 对应的源代码位置。see: https://inspector.fe-dev.cn/guide/start.html
    VITE_CODEINSPECTOR &&
      codeInspectorPlugin({
        bundler: "vite"
      })
  ];
};

/**
 * @description 根据 compress 配置，生成不同的压缩规则
 * @param viteEnv
 */
const createCompression = (viteEnv: ViteEnv): PluginOption | PluginOption[] => {
  const { VITE_BUILD_COMPRESS = "none", VITE_BUILD_COMPRESS_DELETE_ORIGIN_FILE } = viteEnv;
  const compressList = VITE_BUILD_COMPRESS.split(",");
  const plugins: PluginOption[] = [];
  if (compressList.includes("gzip")) {
    plugins.push(
      viteCompression({
        ext: ".gz",
        algorithm: "gzip",
        deleteOriginFile: VITE_BUILD_COMPRESS_DELETE_ORIGIN_FILE
      })
    );
  }
  if (compressList.includes("brotli")) {
    plugins.push(
      viteCompression({
        ext: ".br",
        algorithm: "brotliCompress",
        deleteOriginFile: VITE_BUILD_COMPRESS_DELETE_ORIGIN_FILE
      })
    );
  }
  return plugins;
};

/**
 * @description VitePwa
 * @param viteEnv
 */
const createVitePwa = (viteEnv: ViteEnv): PluginOption | PluginOption[] => {
  const { VITE_GLOB_APP_TITLE } = viteEnv;
  return VitePWA({
    registerType: "autoUpdate",
    manifest: {
      name: VITE_GLOB_APP_TITLE,
      short_name: VITE_GLOB_APP_TITLE,
      theme_color: "#ffffff",
      icons: [
        {
          src: "/logo.png",
          sizes: "192x192",
          type: "image/png"
        },
        {
          src: "/logo.png",
          sizes: "512x512",
          type: "image/png"
        },
        {
          src: "/logo.png",
          sizes: "512x512",
          type: "image/png",
          purpose: "any maskable"
        }
      ]
    }
  });
};
