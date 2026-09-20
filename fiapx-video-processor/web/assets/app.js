const TOKEN_KEY = "fiapx.jwt";
const EMAIL_KEY = "fiapx.email";

const $ = (id) => document.getElementById(id);
const authView = $("auth-view");
const appView = $("app-view");
const session = $("session");
const who = $("who");
const authError = $("auth-error");
const uploadMsg = $("upload-msg");
const jobsEl = $("jobs");
const jobCount = $("job-count");

let pollTimer = null;

function token() {
  return sessionStorage.getItem(TOKEN_KEY) || "";
}

function show(el, on) {
  el.classList.toggle("hidden", !on);
}

function banner(el, text, kind) {
  el.textContent = text;
  el.className = "banner " + (kind || "");
  show(el, Boolean(text));
}

function authHeaders(extra) {
  const h = extra ? { ...extra } : {};
  const t = token();
  if (t) h.Authorization = "Bearer " + t;
  return h;
}

async function api(path, opts) {
  const res = await fetch(path, opts);
  const ct = res.headers.get("content-type") || "";
  const body = ct.includes("application/json") ? await res.json() : null;
  if (!res.ok) {
    const err = new Error((body && body.error) || res.statusText);
    err.status = res.status;
    throw err;
  }
  return { res, body };
}

function setSession(jwt, email) {
  if (jwt) {
    sessionStorage.setItem(TOKEN_KEY, jwt);
    sessionStorage.setItem(EMAIL_KEY, email || "");
  } else {
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(EMAIL_KEY);
  }
  renderShell();
}

function renderShell() {
  const inSession = Boolean(token());
  show(authView, !inSession);
  show(appView, inSession);
  show(session, inSession);
  who.textContent = sessionStorage.getItem(EMAIL_KEY) || "";
  if (inSession) {
    loadJobs();
    if (!pollTimer) pollTimer = setInterval(loadJobs, 2000);
  } else if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

$("auth-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  banner(authError, "");
  const email = $("email").value.trim();
  const password = $("password").value;
  try {
    const { body } = await api("/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });
    setSession(body.token, email);
  } catch (err) {
    banner(authError, err.message, "error");
  }
});

$("register-btn").addEventListener("click", async () => {
  banner(authError, "");
  const email = $("email").value.trim();
  const password = $("password").value;
  try {
    await api("/auth/register", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });
    const { body } = await api("/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });
    setSession(body.token, email);
  } catch (err) {
    banner(authError, err.message, "error");
  }
});

$("logout").addEventListener("click", () => setSession("", ""));

$("upload-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const file = $("video").files[0];
  if (!file) {
    banner(uploadMsg, "Selecione um vídeo.", "error");
    return;
  }
  const fd = new FormData();
  fd.append("video", file);
  const btn = e.target.querySelector("button[type=submit]");
  btn.disabled = true;
  try {
    const { body, res } = await api("/videos", {
      method: "POST",
      headers: authHeaders(),
      body: fd,
    });
    banner(
      uploadMsg,
      "HTTP " + res.status + " · " + body.status + " · " + body.id,
      "ok"
    );
    $("video").value = "";
    loadJobs();
  } catch (err) {
    banner(uploadMsg, err.message, "error");
  } finally {
    btn.disabled = false;
  }
});

async function loadJobs() {
  if (!token()) return;
  try {
    const { body } = await api("/videos", { headers: authHeaders() });
    const items = body.items || [];
    jobCount.textContent = items.length + (items.length === 1 ? " job" : " jobs");
    if (!items.length) {
      jobsEl.innerHTML = "<p class=\"muted\">Nenhum vídeo ainda.</p>";
      return;
    }
    jobsEl.innerHTML = items.map(jobCard).join("");
    jobsEl.querySelectorAll("[data-zip]").forEach((btn) => {
      btn.addEventListener("click", () => downloadZip(btn.getAttribute("data-zip")));
    });
  } catch (err) {
    if (err.status === 401) setSession("", "");
    else jobsEl.innerHTML = "<p class=\"banner error\">" + escapeHtml(err.message) + "</p>";
  }
}

function jobCard(j) {
  const ready = j.status === "READY";
  const failed = j.status === "FAILED";
  const when = j.created_at ? new Date(j.created_at).toLocaleString("pt-BR") : "";
  const extra = failed && j.error_message
    ? "<div class=\"meta\">" + escapeHtml(j.error_message) + "</div>"
    : "<div class=\"meta\">" + escapeHtml(j.id) + (j.frame_count ? " · " + j.frame_count + " frames" : "") + "</div>";
  const action = ready
    ? "<button type=\"button\" data-zip=\"" + j.id + "\">Baixar ZIP</button>"
    : "";
  return (
    "<article class=\"job\">" +
      "<header><span class=\"name\">" + escapeHtml(j.original_filename) + "</span>" +
      "<span class=\"badge " + escapeHtml(j.status) + "\">" + escapeHtml(j.status) + "</span></header>" +
      action +
      "<div class=\"meta\">" + escapeHtml(when) + "</div>" +
      extra +
    "</article>"
  );
}

async function downloadZip(id) {
  try {
    const res = await fetch("/videos/" + id + "/zip", { headers: authHeaders() });
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(body.error || res.statusText);
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = id + ".zip";
    a.click();
    URL.revokeObjectURL(url);
  } catch (err) {
    banner(uploadMsg, err.message, "error");
  }
}

function escapeHtml(s) {
  return String(s == null ? "" : s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

renderShell();
