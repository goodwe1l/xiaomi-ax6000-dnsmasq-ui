import { createApp } from "/assets/vendor/petite-vue.es.js";
import {
  authStatus,
  deleteLease,
  deleteTemplate,
  getState,
  login,
  logout,
  saveDefault,
  toggleDhcp,
  upsertLease,
  upsertTemplate,
} from "/assets/js/api.js";

const EMPTY_DATA = {
  dhcp: {
    enabled: false,
    state: "未知",
    toggleTo: "1",
    toggleLabel: "开启 LAN DHCP",
  },
  defaults: {
    gateway: "",
    dns: "",
  },
  staticLeases: [],
  templates: [],
  dynamic: {
    lanPrefix: "",
    lan: [],
    other: [],
  },
};

const DRAWER_ANIMATION_MS = 320;

function cloneEmptyData() {
  return {
    dhcp: { ...EMPTY_DATA.dhcp },
    defaults: { ...EMPTY_DATA.defaults },
    staticLeases: [],
    templates: [],
    dynamic: {
      lanPrefix: "",
      lan: [],
      other: [],
    },
  };
}

const state = {
  root: null,
  loading: true,
  pendingKey: "",
  data: cloneEmptyData(),
  defaultDnsList: [""],
  auth: {
    checked: false,
    authenticated: false,
    defaultPassword: true,
  },
  loginForm: {
    password: "",
  },
  editingLeaseMac: "",
  editingTemplateSec: "",
  leaseDraft: null,
  templateDraft: null,
  drawer: {
    phase: "closed",
    mode: "lease",
    title: "新增静态租约",
    payload: {},
    timer: null,
  },
  confirmDialog: {
    open: false,
    title: "",
    message: "",
    confirmText: "确认",
    onConfirm: null,
    pendingKey: "",
  },
  snackbar: {
    show: false,
    kind: "info",
    text: "",
  },
  snackbarTimer: null,
};

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function icon(name) {
  return `<svg class="icon" aria-hidden="true"><use href="/assets/icons/sprite.svg#${name}"></use></svg>`;
}

function isBusy(key) {
  return state.pendingKey === key;
}

function anyBusy() {
  return Boolean(state.pendingKey);
}

function splitDnsList(dnsText) {
  if (!dnsText) {
    return [""];
  }
  const list = String(dnsText)
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
  return list.length ? list : [""];
}

function syncDefaultDnsFromData() {
  state.defaultDnsList = splitDnsList(state.data.defaults.dns || "");
}

function clearDrawerTimer() {
  if (state.drawer.timer) {
    clearTimeout(state.drawer.timer);
    state.drawer.timer = null;
  }
}

function forceCloseDrawer(shouldRender = true) {
  clearDrawerTimer();
  state.drawer.phase = "closed";
  state.drawer.payload = {};
  if (shouldRender) {
    render();
  }
}

function openDrawer(mode, title, payload) {
  clearDrawerTimer();
  state.drawer.mode = mode;
  state.drawer.title = title;
  state.drawer.payload = payload || {};
  state.drawer.phase = "entering";
  render();
  state.drawer.timer = setTimeout(() => {
    if (state.drawer.phase !== "entering") {
      return;
    }
    state.drawer.phase = "open";
    state.drawer.timer = null;
    render();
  }, DRAWER_ANIMATION_MS);
}

function normalizeData(raw) {
  const data = raw || {};
  return {
    dhcp: {
      enabled: Boolean(data?.dhcp?.enabled),
      state: data?.dhcp?.state || "未知",
      toggleTo: data?.dhcp?.toggleTo ?? "1",
      toggleLabel: data?.dhcp?.toggleLabel || "切换",
    },
    defaults: {
      gateway: data?.defaults?.gateway || "",
      dns: data?.defaults?.dns || "",
    },
    staticLeases: Array.isArray(data?.staticLeases) ? data.staticLeases : [],
    templates: Array.isArray(data?.templates) ? data.templates : [],
    dynamic: {
      lanPrefix: data?.dynamic?.lanPrefix || "",
      lan: Array.isArray(data?.dynamic?.lan) ? data.dynamic.lan : [],
      other: Array.isArray(data?.dynamic?.other) ? data.dynamic.other : [],
    },
  };
}

function resetDataState() {
  state.data = cloneEmptyData();
  state.defaultDnsList = [""];
  state.editingLeaseMac = "";
  state.editingTemplateSec = "";
  state.leaseDraft = null;
  state.templateDraft = null;
  forceCloseDrawer(false);
  closeConfirm();
}

function showSnackbar(text, kind = "info") {
  if (state.snackbarTimer) {
    clearTimeout(state.snackbarTimer);
  }
  state.snackbar = {
    show: true,
    kind,
    text: text || "完成",
  };
  render();
  state.snackbarTimer = setTimeout(() => {
    state.snackbar.show = false;
    render();
  }, 2600);
}

function handleUnauthorized(message = "登录已过期，请重新登录") {
  state.auth.authenticated = false;
  state.auth.checked = true;
  state.loading = false;
  resetDataState();
  showSnackbar(message, "error");
}

async function refreshState(showLoading = true) {
  if (!state.auth.authenticated) {
    state.loading = false;
    render();
    return;
  }
  if (showLoading) {
    state.loading = true;
    render();
  }
  try {
    const data = await getState();
    state.data = normalizeData(data);
    syncDefaultDnsFromData();
  } catch (error) {
    if (error?.code === "UNAUTHORIZED") {
      handleUnauthorized(error.message);
      return;
    }
    showSnackbar(error.message || "拉取状态失败", "error");
  } finally {
    state.loading = false;
    render();
  }
}

async function bootstrap() {
  state.loading = true;
  render();
  try {
    const authInfo = await authStatus();
    state.auth.checked = true;
    state.auth.authenticated = Boolean(authInfo?.authenticated);
    state.auth.defaultPassword = Boolean(authInfo?.defaultPassword);
    if (state.auth.authenticated) {
      await refreshState(false);
      return;
    }
    resetDataState();
  } catch (error) {
    state.auth.checked = true;
    state.auth.authenticated = false;
    showSnackbar(error.message || "认证状态检查失败", "error");
  } finally {
    state.loading = false;
    render();
  }
}

async function runMutation(key, fn, options = {}) {
  if (anyBusy()) {
    return;
  }
  state.pendingKey = key;
  render();
  try {
    const result = await fn();
    const message = options.successMessage || result?.message || "操作成功";
    showSnackbar(message, "success");
    if (options.refresh !== false) {
      await refreshState(false);
    }
  } catch (error) {
    if (error?.code === "UNAUTHORIZED") {
      handleUnauthorized(error.message);
      return;
    }
    showSnackbar(error.message || "操作失败", "error");
  } finally {
    state.pendingKey = "";
    render();
  }
}

async function doLogin(passwordText) {
  if (anyBusy()) {
    return;
  }
  const password = String(passwordText || "").trim();
  if (!password) {
    showSnackbar("请输入登录密码", "error");
    return;
  }
  state.pendingKey = "login";
  render();
  try {
    await login(password);
    state.auth.authenticated = true;
    state.loginForm.password = "";
    showSnackbar("登录成功", "success");
    await refreshState(false);
  } catch (error) {
    showSnackbar(error.message || "登录失败", "error");
  } finally {
    state.pendingKey = "";
    render();
  }
}

async function doLogout() {
  if (anyBusy()) {
    return;
  }
  state.pendingKey = "logout";
  render();
  try {
    await logout();
    state.auth.authenticated = false;
    resetDataState();
    showSnackbar("已退出登录", "success");
  } catch (error) {
    showSnackbar(error.message || "退出失败", "error");
  } finally {
    state.pendingKey = "";
    state.loading = false;
    render();
  }
}

function openLeaseDrawer(prefill = {}) {
  openDrawer("lease", "新增静态租约", {
    name: prefill.name || "",
    mac: prefill.mac || "",
    ip: prefill.ip || "",
    gateway: prefill.gateway || "",
    tag: prefill.tag || "",
    dns: prefill.dns || "",
  });
}

function openTemplateDrawer(prefill = {}) {
  openDrawer("template", "新增标签模板", {
    template_tag: prefill.template_tag || "",
    template_gateway: prefill.template_gateway || "",
    template_dns: prefill.template_dns || "",
  });
}

function closeDrawer() {
  if (state.drawer.phase === "closed" || state.drawer.phase === "closing") {
    return;
  }
  clearDrawerTimer();
  state.drawer.phase = "closing";
  render();
  state.drawer.timer = setTimeout(() => {
    if (state.drawer.phase !== "closing") {
      return;
    }
    state.drawer.phase = "closed";
    state.drawer.payload = {};
    state.drawer.timer = null;
    render();
  }, DRAWER_ANIMATION_MS);
}

function openConfirm({ title, message, confirmText = "确认", onConfirm, pendingKey }) {
  state.confirmDialog = {
    open: true,
    title,
    message,
    confirmText,
    onConfirm,
    pendingKey: pendingKey || "confirm",
  };
  render();
}

function closeConfirm() {
  state.confirmDialog = {
    open: false,
    title: "",
    message: "",
    confirmText: "确认",
    onConfirm: null,
    pendingKey: "",
  };
  render();
}

async function acceptConfirm() {
  if (!state.confirmDialog.open || typeof state.confirmDialog.onConfirm !== "function") {
    return;
  }
  const action = state.confirmDialog.onConfirm;
  const pendingKey = state.confirmDialog.pendingKey || "confirm";
  closeConfirm();
  await runMutation(pendingKey, action);
}

function startLeaseInlineEdit(mac) {
  const target = state.data.staticLeases.find((item) => item.mac === mac);
  if (!target) {
    return;
  }
  state.editingLeaseMac = mac;
  state.leaseDraft = {
    name: target.name || "",
    mac: target.mac || "",
    ip: target.ip || "",
    tag: target.tag || "",
    gateway: target.gateway || "",
    dns: target.dns || "",
  };
  render();
}

function cancelLeaseInlineEdit() {
  state.editingLeaseMac = "";
  state.leaseDraft = null;
  render();
}

function startTemplateInlineEdit(sec) {
  const target = state.data.templates.find((item) => item.sec === sec);
  if (!target) {
    return;
  }
  state.editingTemplateSec = sec;
  state.templateDraft = {
    template_sec: target.sec || "",
    template_tag: target.tag || "",
    template_gateway: target.gateway || "",
    template_dns: target.dns || "",
  };
  render();
}

function cancelTemplateInlineEdit() {
  state.editingTemplateSec = "";
  state.templateDraft = null;
  render();
}

function readInlineRow(row) {
  const payload = {};
  row.querySelectorAll("[data-field]").forEach((input) => {
    payload[input.dataset.field] = input.value.trim();
  });
  return payload;
}

function renderButton({
  key,
  label,
  pendingLabel,
  iconName,
  className = "btn",
  type = "button",
  attrs = "",
  iconOnly = false,
  allowWhenBusy = false,
}) {
  const pending = isBusy(key);
  const blocked = !allowWhenBusy && anyBusy();
  const disabled = pending || blocked ? "disabled" : "";
  const aria = iconOnly ? `aria-label="${escapeHtml(label || "按钮")}"` : "";
  const content = pending
    ? `${icon("i-refresh")}${iconOnly ? "" : `<span>${escapeHtml(pendingLabel || `${label}中...`)}</span>`}`
    : `${icon(iconName)}${iconOnly ? "" : `<span>${escapeHtml(label)}</span>`}`;
  return `<button type="${type}" class="${className}" ${disabled} ${aria} ${attrs}>${content}</button>`;
}

function renderStaticRows() {
  if (!state.data.staticLeases.length) {
    return `<tr><td colspan="7" class="empty-line">暂无静态分配记录</td></tr>`;
  }
  return state.data.staticLeases
    .map((item) => {
      const mac = item.mac || "";
      if (state.editingLeaseMac === mac && state.leaseDraft) {
        return `
          <tr data-edit-row="lease" data-mac="${escapeHtml(mac)}" class="editing-row">
            <td><input data-field="name" value="${escapeHtml(state.leaseDraft.name)}" required /></td>
            <td><input data-field="mac" value="${escapeHtml(state.leaseDraft.mac)}" readonly /></td>
            <td><input data-field="ip" value="${escapeHtml(state.leaseDraft.ip)}" required /></td>
            <td><input data-field="tag" value="${escapeHtml(state.leaseDraft.tag)}" /></td>
            <td><input data-field="gateway" value="${escapeHtml(state.leaseDraft.gateway)}" /></td>
            <td><input data-field="dns" value="${escapeHtml(state.leaseDraft.dns)}" /></td>
            <td class="row-actions">
              ${renderButton({
                key: `lease-inline-save:${mac}`,
                label: "保存",
                pendingLabel: "保存中...",
                iconName: "i-save",
                className: "btn btn-small",
                attrs: `data-action="save-edit-lease"`,
              })}
              ${renderButton({
                key: "cancel-edit-lease",
                label: "取消",
                iconName: "i-close",
                className: "btn btn-text btn-small",
                attrs: `data-action="cancel-edit-lease"`,
                allowWhenBusy: true,
              })}
            </td>
          </tr>
        `;
      }
      return `
        <tr>
          <td>${escapeHtml(item.name || "-")}</td>
          <td>${escapeHtml(mac || "-")}</td>
          <td>${escapeHtml(item.ip || "-")}</td>
          <td>${escapeHtml(item.tag || "-")}</td>
          <td>${escapeHtml(item.gateway || "-")}</td>
          <td>${escapeHtml(item.dns || "-")}</td>
          <td class="row-actions">
            ${renderButton({
              key: `lease-inline-start:${mac}`,
              label: "编辑",
              iconName: "i-edit",
              className: "btn btn-small btn-text",
              attrs: `data-action="start-edit-lease" data-mac="${escapeHtml(mac)}"`,
            })}
            ${renderButton({
              key: `lease-delete:${mac}`,
              label: "删除",
              pendingLabel: "删除中...",
              iconName: "i-delete",
              className: "btn btn-small btn-danger",
              attrs: `data-action="delete-lease" data-mac="${escapeHtml(mac)}"`,
            })}
          </td>
        </tr>
      `;
    })
    .join("");
}

function renderTemplateRows() {
  if (!state.data.templates.length) {
    return `<tr><td colspan="5" class="empty-line">暂无可用标签模板</td></tr>`;
  }
  return state.data.templates
    .map((item) => {
      const sec = item.sec || "";
      if (state.editingTemplateSec === sec && state.templateDraft) {
        return `
          <tr data-edit-row="template" data-sec="${escapeHtml(sec)}" class="editing-row">
            <td><input data-field="template_tag" value="${escapeHtml(state.templateDraft.template_tag)}" required /></td>
            <td><input data-field="template_gateway" value="${escapeHtml(state.templateDraft.template_gateway)}" /></td>
            <td><input data-field="template_dns" value="${escapeHtml(state.templateDraft.template_dns)}" /></td>
            <td><span class="chip ${item.inUse ? "chip-warning" : "chip-ok"}">${item.inUse ? "使用中" : "未使用"}</span></td>
            <td class="row-actions">
              <input type="hidden" data-field="template_sec" value="${escapeHtml(sec)}" />
              ${renderButton({
                key: `template-inline-save:${sec}`,
                label: "保存",
                pendingLabel: "保存中...",
                iconName: "i-save",
                className: "btn btn-small",
                attrs: `data-action="save-edit-template"`,
              })}
              ${renderButton({
                key: "cancel-edit-template",
                label: "取消",
                iconName: "i-close",
                className: "btn btn-text btn-small",
                attrs: `data-action="cancel-edit-template"`,
                allowWhenBusy: true,
              })}
            </td>
          </tr>
        `;
      }
      return `
        <tr>
          <td>${escapeHtml(item.tag || "-")}</td>
          <td>${escapeHtml(item.gateway || "-")}</td>
          <td>${escapeHtml(item.dns || "-")}</td>
          <td><span class="chip ${item.inUse ? "chip-warning" : "chip-ok"}">${item.inUse ? "使用中" : "未使用"}</span></td>
          <td class="row-actions">
            ${renderButton({
              key: `template-inline-start:${sec}`,
              label: "编辑",
              iconName: "i-edit",
              className: "btn btn-small btn-text",
              attrs: `data-action="start-edit-template" data-sec="${escapeHtml(sec)}"`,
            })}
            ${renderButton({
              key: `template-delete:${sec}`,
              label: "删除",
              pendingLabel: "删除中...",
              iconName: "i-delete",
              className: "btn btn-small btn-danger",
              attrs: `data-action="delete-template" data-sec="${escapeHtml(sec)}" ${item.inUse ? "disabled" : ""}`,
            })}
          </td>
        </tr>
      `;
    })
    .join("");
}

function renderDynamicRows(rows) {
  if (!rows.length) {
    return `<tr><td colspan="6" class="empty-line">暂无记录</td></tr>`;
  }
  return rows
    .map((item) => {
      const actionCell = item.isStatic
        ? '<span class="muted">-</span>'
        : renderButton({
            key: `convert:${item.mac}`,
            label: "转为静态",
            iconName: "i-add",
            className: "btn btn-small btn-text",
            attrs: `data-action="open-add-lease" data-name="${escapeHtml(item.hostname || "")}" data-mac="${escapeHtml(item.mac || "")}" data-ip="${escapeHtml(item.ip || "")}"`,
          });
      const chipClass = item.isStatic ? "chip-warning" : "chip-ok";
      return `
        <tr>
          <td>${escapeHtml(item.hostname || "-")}</td>
          <td>${escapeHtml(item.mac || "-")}</td>
          <td>${escapeHtml(item.ip || "-")}</td>
          <td><span class="chip ${chipClass}">${escapeHtml(item.type || "-")}</span></td>
          <td>${escapeHtml(item.remain || "-")}</td>
          <td class="row-actions">${actionCell}</td>
        </tr>
      `;
    })
    .join("");
}

function renderDefaultDnsEditor() {
  return `
    <div class="dns-editor">
      <label>默认 DNS（Option 6，可多条）</label>
      <div class="dns-list">
        ${state.defaultDnsList
          .map(
            (dns, index) => `
              <div class="dns-row">
                <input data-default-dns-index="${index}" value="${escapeHtml(dns)}" placeholder="例如 223.5.5.5" />
                ${renderButton({
                  key: `remove-default-dns:${index}`,
                  label: "删除 DNS",
                  iconName: "i-delete",
                  className: "btn btn-small btn-danger",
                  attrs: `data-action="remove-default-dns" data-index="${index}" ${state.defaultDnsList.length <= 1 ? "disabled" : ""}`,
                  allowWhenBusy: true,
                  iconOnly: true,
                })}
              </div>
            `,
          )
          .join("")}
      </div>
      <div class="dns-tools">
        ${renderButton({
          key: "add-default-dns",
          label: "新增 DNS",
          iconName: "i-add",
          className: "btn btn-small btn-tonal",
          attrs: 'data-action="add-default-dns"',
          allowWhenBusy: true,
        })}
      </div>
    </div>
  `;
}

function renderDrawer() {
  if (state.drawer.phase === "closed") {
    return "";
  }
  const payload = state.drawer.payload || {};
  const isLease = state.drawer.mode === "lease";
  const phaseClass = state.drawer.phase;

  const leaseBody = `
    <label>设备名</label>
    <input name="name" value="${escapeHtml(payload.name || "")}" required />
    <label>MAC 地址</label>
    <input name="mac" value="${escapeHtml(payload.mac || "")}" required />
    <label>静态 IP</label>
    <input name="ip" value="${escapeHtml(payload.ip || "")}" required />
    <label>标签（可选）</label>
    <input name="tag" value="${escapeHtml(payload.tag || "")}" />
    <label>网关（可选）</label>
    <input name="gateway" value="${escapeHtml(payload.gateway || "")}" />
    <label>DNS（可选，逗号分隔）</label>
    <input name="dns" value="${escapeHtml(payload.dns || "")}" />
  `;

  const templateBody = `
    <label>标签名</label>
    <input name="template_tag" value="${escapeHtml(payload.template_tag || "")}" required />
    <label>标签网关（Option 3）</label>
    <input name="template_gateway" value="${escapeHtml(payload.template_gateway || "")}" />
    <label>标签 DNS（Option 6，逗号分隔）</label>
    <input name="template_dns" value="${escapeHtml(payload.template_dns || "")}" />
  `;

  return `
    <div class="drawer-backdrop ${phaseClass}" data-action="close-drawer"></div>
    <aside class="drawer ${phaseClass}" role="dialog" aria-modal="true">
      <header class="drawer-header">
        <h3>${escapeHtml(state.drawer.title || "")}</h3>
        <button type="button" class="btn btn-icon btn-text only-icon" data-action="close-drawer" aria-label="关闭抽屉">${icon("i-close")}</button>
      </header>
      <form data-form="drawer" data-mode="${isLease ? "lease" : "template"}" class="drawer-body">
        ${isLease ? leaseBody : templateBody}
        <div class="drawer-actions">
          ${renderButton({
            key: "drawer-submit",
            label: "提交",
            pendingLabel: "提交中...",
            iconName: "i-save",
            className: "btn",
            type: "submit",
          })}
          ${renderButton({
            key: "drawer-cancel",
            label: "取消",
            iconName: "i-close",
            className: "btn btn-tonal",
            attrs: `data-action="close-drawer"`,
            allowWhenBusy: true,
          })}
        </div>
      </form>
    </aside>
  `;
}

function renderConfirmDialog() {
  const showClass = state.confirmDialog.open ? "show" : "";
  return `
    <div class="dialog-backdrop ${showClass}" data-action="confirm-cancel"></div>
    <div class="dialog ${showClass}" role="dialog" aria-modal="true">
      <h3>${escapeHtml(state.confirmDialog.title || "确认操作")}</h3>
      <p>${escapeHtml(state.confirmDialog.message || "")}</p>
      <div class="dialog-actions">
        ${renderButton({
          key: "confirm-accept",
          label: state.confirmDialog.confirmText || "确认",
          pendingLabel: "处理中...",
          iconName: "i-delete",
          className: "btn btn-danger",
          attrs: `data-action="confirm-accept"`,
        })}
        ${renderButton({
          key: "confirm-cancel",
          label: "取消",
          iconName: "i-close",
          className: "btn btn-tonal",
          attrs: `data-action="confirm-cancel"`,
          allowWhenBusy: true,
        })}
      </div>
    </div>
  `;
}

function renderSnackbar() {
  if (!state.snackbar.show) {
    return "";
  }
  return `
    <div class="snackbar ${escapeHtml(state.snackbar.kind)}">
      ${icon(state.snackbar.kind === "error" ? "i-close" : "i-info")}
      <span>${escapeHtml(state.snackbar.text)}</span>
    </div>
  `;
}

function renderLoginGate() {
  const tipText = state.auth.defaultPassword
    ? "当前使用默认登录密码：admin123456，建议登录后立即在 /data/dhcp_adv/auth.conf 修改。"
    : "登录密码来自 /data/dhcp_adv/auth.conf。";

  const loadingText = state.loading ? "正在检查认证状态..." : "请先登录后再访问 DHCP 配置。";

  return `
    <main class="auth-wrap">
      <section class="auth-card">
        <div class="auth-icon">${icon("i-network")}</div>
        <h2>DHCP 管理登录</h2>
        <p class="muted">${escapeHtml(loadingText)}</p>
        <form data-form="login" class="auth-form">
          <label>访问密码</label>
          <input type="password" name="password" value="${escapeHtml(state.loginForm.password)}" placeholder="请输入密码" autocomplete="current-password" required />
          ${renderButton({
            key: "login",
            label: "登录",
            pendingLabel: "登录中...",
            iconName: "i-save",
            className: "btn auth-submit",
            type: "submit",
          })}
        </form>
        <p class="muted auth-tip">${escapeHtml(tipText)}</p>
      </section>
      ${renderSnackbar()}
    </main>
  `;
}

function renderMainApp() {
  const lanPrefix = state.data.dynamic.lanPrefix
    ? `${state.data.dynamic.lanPrefix}.*`
    : "未知";

  return `
    <header class="app-bar">
      <div class="app-title">
        ${icon("i-network")}
        <div>
          <h1>DHCP 高级管理</h1>
          <p>入口 / ｜ 异步管理静态租约、标签模板与动态租约</p>
        </div>
      </div>
      <div class="app-bar-actions">
        ${renderButton({
          key: "refresh-state",
          label: "刷新",
          pendingLabel: "刷新中...",
          iconName: "i-refresh",
          className: "btn btn-tonal",
          attrs: 'data-action="refresh-state"',
        })}
        ${renderButton({
          key: "logout",
          label: "退出",
          pendingLabel: "退出中...",
          iconName: "i-power",
          className: "btn btn-text",
          attrs: 'data-action="logout"',
          allowWhenBusy: true,
        })}
      </div>
    </header>

    <main class="page-wrap ${state.loading ? "is-loading" : ""}">
      <section class="card">
        <div class="card-head">
          <h2>${icon("i-router")}LAN DHCP 状态</h2>
          <span class="chip ${state.data.dhcp.enabled ? "chip-ok" : "chip-warning"}">${escapeHtml(
            state.data.dhcp.state,
          )}</span>
        </div>
        <div class="card-actions">
          ${renderButton({
            key: "toggle-dhcp",
            label: state.data.dhcp.toggleLabel || "切换",
            pendingLabel: "切换中...",
            iconName: "i-power",
            attrs: 'data-action="toggle-dhcp"',
          })}
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <h2>${icon("i-dns")}默认 DHCP 规则（LAN）</h2>
        </div>
        <form data-form="save-default" class="form-grid">
          <div>
            <label>默认网关（Option 3）</label>
            <input name="default_gateway" value="${escapeHtml(
              state.data.defaults.gateway || "",
            )}" placeholder="例如 10.0.0.1" />
          </div>
          <div>
            ${renderDefaultDnsEditor()}
          </div>
          <div class="form-actions">
            ${renderButton({
              key: "save-default",
              label: "保存默认规则",
              pendingLabel: "保存中...",
              iconName: "i-save",
              type: "submit",
            })}
          </div>
        </form>
      </section>

      <section class="card">
        <div class="card-head">
          <h2>${icon("i-device")}静态租约（行内编辑）</h2>
          ${renderButton({
            key: "open-add-lease",
            label: "新增静态租约",
            iconName: "i-add",
            className: "btn btn-tonal",
            attrs: 'data-action="open-add-lease"',
          })}
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr><th>设备名</th><th>MAC</th><th>IP</th><th>标签</th><th>网关</th><th>DNS</th><th>操作</th></tr>
            </thead>
            <tbody>${renderStaticRows()}</tbody>
          </table>
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <h2>${icon("i-tag")}标签模板（行内编辑）</h2>
          ${renderButton({
            key: "open-add-template",
            label: "新增标签模板",
            iconName: "i-add",
            className: "btn btn-tonal",
            attrs: 'data-action="open-add-template"',
          })}
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr><th>标签</th><th>网关(3)</th><th>DNS(6)</th><th>状态</th><th>操作</th></tr>
            </thead>
            <tbody>${renderTemplateRows()}</tbody>
          </table>
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <h2>${icon("i-lease")}动态租约（LAN）</h2>
          <span class="muted">LAN 前缀：${escapeHtml(lanPrefix)}</span>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr><th>主机名</th><th>MAC</th><th>IP</th><th>类型</th><th>剩余租期</th><th>操作</th></tr>
            </thead>
            <tbody>${renderDynamicRows(state.data.dynamic.lan)}</tbody>
          </table>
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <h2>${icon("i-lease")}其他网段动态租约</h2>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr><th>主机名</th><th>MAC</th><th>IP</th><th>类型</th><th>剩余租期</th><th>操作</th></tr>
            </thead>
            <tbody>${renderDynamicRows(state.data.dynamic.other)}</tbody>
          </table>
        </div>
      </section>
    </main>

    ${renderDrawer()}
    ${renderConfirmDialog()}
    ${renderSnackbar()}
  `;
}

function render() {
  if (!state.root) {
    return;
  }

  if (!state.auth.checked || !state.auth.authenticated) {
    state.root.innerHTML = renderLoginGate();
    return;
  }

  state.root.innerHTML = renderMainApp();
}

function getActionElement(target) {
  return target.closest("[data-action]");
}

async function handleClick(event) {
  const actionEl = getActionElement(event.target);
  if (!actionEl) {
    return;
  }
  const action = actionEl.dataset.action;

  if (action === "close-drawer") {
    closeDrawer();
    return;
  }
  if (action === "confirm-cancel") {
    closeConfirm();
    return;
  }
  if (action === "confirm-accept") {
    await acceptConfirm();
    return;
  }
  if (action === "add-default-dns") {
    state.defaultDnsList.push("");
    render();
    return;
  }
  if (action === "remove-default-dns") {
    const index = Number(actionEl.dataset.index);
    if (Number.isInteger(index) && index >= 0 && index < state.defaultDnsList.length) {
      state.defaultDnsList.splice(index, 1);
      if (!state.defaultDnsList.length) {
        state.defaultDnsList.push("");
      }
      render();
    }
    return;
  }

  if (!state.auth.authenticated) {
    return;
  }

  if (anyBusy()) {
    return;
  }

  if (action === "logout") {
    await doLogout();
    return;
  }

  if (action === "refresh-state") {
    await refreshState(true);
    return;
  }
  if (action === "toggle-dhcp") {
    await runMutation("toggle-dhcp", () => toggleDhcp(state.data.dhcp.toggleTo));
    return;
  }
  if (action === "open-add-lease") {
    openLeaseDrawer({
      name: actionEl.dataset.name || "",
      mac: actionEl.dataset.mac || "",
      ip: actionEl.dataset.ip || "",
    });
    return;
  }
  if (action === "open-add-template") {
    openTemplateDrawer();
    return;
  }
  if (action === "start-edit-lease") {
    startLeaseInlineEdit(actionEl.dataset.mac || "");
    return;
  }
  if (action === "cancel-edit-lease") {
    cancelLeaseInlineEdit();
    return;
  }
  if (action === "save-edit-lease") {
    const row = actionEl.closest('tr[data-edit-row="lease"]');
    if (!row) {
      return;
    }
    const mac = row.dataset.mac || "";
    const payload = readInlineRow(row);
    await runMutation(`lease-inline-save:${mac}`, async () => {
      const result = await upsertLease(payload);
      cancelLeaseInlineEdit();
      return result;
    });
    return;
  }
  if (action === "delete-lease") {
    const mac = actionEl.dataset.mac || "";
    if (!mac) {
      return;
    }
    openConfirm({
      title: "删除静态租约",
      message: `确认删除 ${mac} 这条静态租约吗？`,
      confirmText: "确认删除",
      pendingKey: `lease-delete:${mac}`,
      onConfirm: () => deleteLease(mac),
    });
    return;
  }
  if (action === "start-edit-template") {
    startTemplateInlineEdit(actionEl.dataset.sec || "");
    return;
  }
  if (action === "cancel-edit-template") {
    cancelTemplateInlineEdit();
    return;
  }
  if (action === "save-edit-template") {
    const row = actionEl.closest('tr[data-edit-row="template"]');
    if (!row) {
      return;
    }
    const sec = row.dataset.sec || "";
    const payload = readInlineRow(row);
    await runMutation(`template-inline-save:${sec}`, async () => {
      const result = await upsertTemplate(payload);
      cancelTemplateInlineEdit();
      return result;
    });
    return;
  }
  if (action === "delete-template") {
    const sec = actionEl.dataset.sec || "";
    if (!sec) {
      return;
    }
    openConfirm({
      title: "删除标签模板",
      message: "确认删除该标签模板吗？删除后无法恢复。",
      confirmText: "确认删除",
      pendingKey: `template-delete:${sec}`,
      onConfirm: () => deleteTemplate(sec),
    });
  }
}

function handleInput(event) {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }

  const dnsIndex = target.dataset.defaultDnsIndex;
  if (dnsIndex !== undefined) {
    const index = Number(dnsIndex);
    if (Number.isInteger(index) && index >= 0 && index < state.defaultDnsList.length) {
      state.defaultDnsList[index] = target.value;
    }
    return;
  }

  if (target.name === "password") {
    state.loginForm.password = target.value;
  }
}

function formToObject(form) {
  const payload = {};
  const formData = new FormData(form);
  for (const [key, value] of formData.entries()) {
    payload[key] = String(value).trim();
  }
  return payload;
}

async function handleSubmit(event) {
  const form = event.target;
  if (!(form instanceof HTMLFormElement)) {
    return;
  }
  const formType = form.dataset.form;
  if (!formType) {
    return;
  }
  event.preventDefault();

  if (formType === "login") {
    await doLogin(state.loginForm.password);
    return;
  }

  if (!state.auth.authenticated || anyBusy()) {
    return;
  }

  if (formType === "save-default") {
    const gatewayInput = form.querySelector('input[name="default_gateway"]');
    const defaultGateway = gatewayInput ? gatewayInput.value.trim() : "";
    const defaultDns = state.defaultDnsList
      .map((item) => String(item || "").trim())
      .filter(Boolean)
      .join(",");
    await runMutation("save-default", () =>
      saveDefault({
        default_gateway: defaultGateway,
        default_dns: defaultDns,
      }),
    );
    return;
  }

  if (formType === "drawer") {
    const payload = formToObject(form);
    if (form.dataset.mode === "lease") {
      await runMutation("drawer-submit", async () => {
        const result = await upsertLease(payload);
        closeDrawer();
        return result;
      });
      return;
    }
    await runMutation("drawer-submit", async () => {
      const result = await upsertTemplate(payload);
      closeDrawer();
      return result;
    });
  }
}

function mount(root) {
  state.root = root;
  root.addEventListener("click", handleClick);
  root.addEventListener("submit", handleSubmit);
  root.addEventListener("input", handleInput);
  render();
  bootstrap();
}

createApp({
  mount,
}).mount("#app");
