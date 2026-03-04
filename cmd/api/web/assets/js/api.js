const API_ENDPOINT = "/cgi-bin/xiaomi-dnsmasq-gui_api";

function toFormBody(payload) {
  const form = new URLSearchParams();
  Object.keys(payload).forEach((key) => {
    const value = payload[key];
    if (value === undefined || value === null) {
      return;
    }
    form.append(key, String(value));
  });
  return form.toString();
}

async function parseJsonResponse(response) {
  const text = await response.text();
  if (!text) {
    const error = new Error("接口返回为空");
    error.code = "EMPTY_BODY";
    throw error;
  }
  let json;
  try {
    json = JSON.parse(text);
  } catch (error) {
    const parseError = new Error("接口返回非 JSON");
    parseError.code = "INVALID_JSON";
    throw parseError;
  }
  if (!response.ok) {
    const message = json?.message || `HTTP ${response.status}`;
    const httpError = new Error(message);
    httpError.code = json?.code || `HTTP_${response.status}`;
    throw httpError;
  }
  if (!json.ok) {
    const apiError = new Error(json.message || "接口处理失败");
    apiError.code = json.code || "API_ERROR";
    throw apiError;
  }
  return json;
}

async function requestGet(action) {
  const url = `${API_ENDPOINT}?action=${encodeURIComponent(action)}`;
  const response = await fetch(url, {
    method: "GET",
    cache: "no-store",
    credentials: "same-origin",
    headers: {
      Accept: "application/json",
    },
  });
  return parseJsonResponse(response);
}

async function requestPost(action, payload = {}) {
  const body = toFormBody({
    action,
    ...payload,
  });
  const response = await fetch(API_ENDPOINT, {
    method: "POST",
    cache: "no-store",
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      Accept: "application/json",
    },
    body,
  });
  return parseJsonResponse(response);
}

export async function getState() {
  const json = await requestGet("get_state");
  return json.data;
}

export async function authStatus() {
  const json = await requestGet("auth_status");
  return json.data;
}

export async function login(password) {
  return requestPost("login", { password });
}

export async function logout() {
  return requestPost("logout", {});
}

export async function toggleDhcp(enable) {
  return requestPost("toggle_dhcp", { enable });
}

export async function saveDefault(payload) {
  return requestPost("save_default", payload);
}

export async function upsertLease(payload) {
  return requestPost("lease_upsert", payload);
}

export async function deleteLease(mac) {
  return requestPost("lease_delete", { mac });
}

export async function upsertTemplate(payload) {
  return requestPost("template_upsert", payload);
}

export async function deleteTemplate(templateSec) {
  return requestPost("template_delete", { template_sec: templateSec });
}
