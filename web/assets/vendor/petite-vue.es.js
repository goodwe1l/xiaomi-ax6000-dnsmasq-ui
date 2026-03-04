// 轻量 Petite-Vue 兼容层：保留 createApp 入口，便于在无构建链路环境下运行。
export function createApp(context) {
  const app = typeof context === "function" ? context() : context || {};
  return {
    mount(target) {
      const root =
        typeof target === "string" ? document.querySelector(target) : target;
      if (!root) {
        throw new Error("createApp.mount: 找不到挂载节点");
      }
      if (typeof app.mount === "function") {
        app.mount(root);
      }
      return app;
    },
  };
}
