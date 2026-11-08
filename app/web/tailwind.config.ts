// tailwind.config.js
/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class', // 关键配置
  // @gospeak/components 的源码同样是类名来源：漏扫会让共享组件里独有的
  // 变体类（如 data-[orientation=vertical]:flex-col）永远不生成，表现为样式莫名失效。
  content: [
    "./src/**/*.{html,tsx}",
    "../../packages/components/src/**/*.{ts,tsx}",
  ],
  plugins: [],
}
