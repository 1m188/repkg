import { defineConfig } from "vite";

// base 使用相对路径，使 dist 可部署到任意子路径（GitHub Pages 项目页 /repkg/）或本地直接打开
export default defineConfig({
  base: "./",
  build: {
    target: "es2022",
    // 独立 worker 由 Vite 自动打包为独立 chunk
  },
});
