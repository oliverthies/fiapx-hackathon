const TOKEN_KEY = "fiapx.jwt";
const EMAIL_KEY = "fiapx.email";
const POLL_FAST = 1000;
const POLL_SLOW = 3000;

const $ = (id) => document.getElementById(id);
const authView = $("auth-view");
const appView = $("app-view");
const session = $("session");
const who = $("who");
const whoLabel = $("who-label");
const authError = $("auth-error");
const uploadMsg = $("upload-msg");
const inflightCount = $("inflight-count");
const doneCount = $("done-count");
const pickedEl = $("picked");
const tableEl = $("jobs-table");
const emptyEl = $("jobs-empty");
const dashRecent = $("dash-recent");
const uploadModal = $("upload-modal");
const appMsg = $("app-msg");

let pollTimer = null;
let pollMs = 0;
let tickTimer = null;
let jobsCache = [];
let contentFilter = "all";
let searchQuery = "";
let currentPage = "dashboard";
const thumbURLs = new Map();

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

function clearThumbs() {
  for (const url of thumbURLs.values()) {
    URL.revokeObjectURL(url);
  }
  thumbURLs.clear();
}

function setSession(jwt, email) {
  if (jwt) {
    sessionStorage.setItem(TOKEN_KEY, jwt);
    sessionStorage.setItem(EMAIL_KEY, email || "");
  } else {
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(EMAIL_KEY);
    clearThumbs();
    jobsCache = [];
    tableEl.innerHTML = "";
    dashRecent.innerHTML = "";
  }
  renderShell();
}

function initials(email) {
  const local = String(email || "?").split("@")[0];
  return (local[0] || "?").toUpperCase();
}

function renderShell() {
  const inSession = Boolean(token());
  document.body.classList.toggle("signed-out", !inSession);
  document.body.classList.toggle("signed-in", inSession);
  show(authView, !inSession);
  show(appView, inSession);
  show(session, inSession);
  show($("create-btn"), inSession);
  const email = sessionStorage.getItem(EMAIL_KEY) || "";
  whoLabel.textContent = email;
  who.textContent = initials(email);
  if (inSession) {
    setPage(currentPage);
    loadJobs();
    armPoll(true);
    if (!tickTimer) tickTimer = setInterval(tickLive, 500);
  } else {
    closeUpload();
    stopTimers();
  }
}

function stopTimers() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
  pollMs = 0;
  if (tickTimer) {
    clearInterval(tickTimer);
    tickTimer = null;
  }
}

function armPoll(fast) {
  const ms = fast ? POLL_FAST : POLL_SLOW;
  if (pollTimer && pollMs === ms) return;
  if (pollTimer) clearInterval(pollTimer);
  pollMs = ms;
  pollTimer = setInterval(loadJobs, ms);
}

function syncNav() {
  const uploading = !uploadModal.classList.contains("hidden");
  const active = uploading ? "upload" : currentPage;
  document.querySelectorAll(".nav-item").forEach((btn) => {
    btn.classList.toggle("active", btn.getAttribute("data-nav") === active);
  });
}

function setPage(name) {
  if (name === "upload") {
    currentPage = "content";
    show($("page-dashboard"), false);
    show($("page-content"), true);
    openUpload();
    return;
  }
  currentPage = name;
  document.body.classList.remove("nav-open");
  show($("page-dashboard"), currentPage === "dashboard");
  show($("page-content"), currentPage === "content");
  syncNav();
}

function openUpload() {
  show(uploadModal, true);
  syncNav();
}

function closeUpload() {
  show(uploadModal, false);
  syncNav();
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
$("create-btn").addEventListener("click", openUpload);
$("upload-close").addEventListener("click", closeUpload);
$("nav-toggle").addEventListener("click", () => {
  if (window.matchMedia("(max-width: 720px)").matches) {
    document.body.classList.toggle("nav-open");
  } else {
    document.body.classList.toggle("nav-collapsed");
  }
});
document.querySelectorAll(".nav-item").forEach((btn) => {
  btn.addEventListener("click", () => setPage(btn.getAttribute("data-nav")));
});
document.querySelectorAll(".tab").forEach((btn) => {
  btn.addEventListener("click", () => {
    contentFilter = btn.getAttribute("data-filter");
    document.querySelectorAll(".tab").forEach((t) => t.classList.toggle("active", t === btn));
    renderJobs();
  });
});
$("search-form").addEventListener("submit", (e) => e.preventDefault());
$("search").addEventListener("input", () => {
  searchQuery = $("search").value.trim().toLowerCase();
  if (currentPage !== "content") setPage("content");
  renderJobs();
});
uploadModal.addEventListener("click", (e) => {
  if (e.target === uploadModal) closeUpload();
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") closeUpload();
});
document.addEventListener("click", (e) => {
  const zip = e.target.closest("[data-zip]");
  if (zip) {
    downloadZip(zip.getAttribute("data-zip"));
    return;
  }
  const del = e.target.closest("[data-del]");
  if (del) deleteJob(del.getAttribute("data-del"));
});

$("video").addEventListener("change", renderPicked);

const drop = $("upload-form");
["dragenter", "dragover"].forEach((ev) => {
  drop.addEventListener(ev, (e) => {
    e.preventDefault();
    drop.classList.add("over");
  });
});
["dragleave", "drop"].forEach((ev) => {
  drop.addEventListener(ev, (e) => {
    e.preventDefault();
    drop.classList.remove("over");
  });
});
drop.addEventListener("drop", (e) => {
  const files = e.dataTransfer && e.dataTransfer.files;
  if (!files || !files.length) return;
  $("video").files = files;
  renderPicked();
});

function renderPicked() {
  const files = Array.from($("video").files || []);
  if (!files.length) {
    pickedEl.innerHTML = "";
    show(pickedEl, false);
    return;
  }
  pickedEl.innerHTML = files
    .map((f) => "<li><span class=\"name\">" + escapeHtml(f.name) + "</span><span>" + formatBytes(f.size) + "</span></li>")
    .join("");
  show(pickedEl, true);
}

$("upload-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const files = Array.from($("video").files || []);
  if (!files.length) {
    banner(uploadMsg, "Selecione um ou mais vídeos.", "error");
    return;
  }
  const btn = e.target.querySelector("button[type=submit]");
  btn.disabled = true;
  let ok = 0;
  const fails = [];
  try {
    for (let i = 0; i < files.length; i++) {
      banner(uploadMsg, i + "/" + files.length + " enfileirados…", "ok");
      try {
        await uploadOne(files[i]);
        ok++;
      } catch (err) {
        fails.push(files[i].name + ": " + err.message);
      }
      loadJobs();
    }
    const parts = [ok + "/" + files.length + " enfileirados"];
    if (fails.length) parts.push(fails.join(" · "));
    banner(uploadMsg, parts.join(" — "), fails.length ? "error" : "ok");
    $("video").value = "";
    renderPicked();
    loadJobs();
    setPage("content");
  } finally {
    btn.disabled = false;
  }
});

async function uploadOne(file) {
  const fd = new FormData();
  fd.append("video", file);
  const engine = document.querySelector("#upload-form input[name=processor]:checked");
  fd.append("processor", engine ? engine.value : "ffmpeg");
  await api("/videos", {
    method: "POST",
    headers: authHeaders(),
    body: fd,
  });
}

function isInflight(j) {
  return j.status === "QUEUED" || j.status === "PROCESSING" || j.status === "UPLOADED";
}

async function loadJobs() {
  if (!token()) return;
  try {
    const { body } = await api("/videos", { headers: authHeaders() });
    jobsCache = body.items || [];
    renderJobs();
    armPoll(jobsCache.some(isInflight));
  } catch (err) {
    if (err.status === 401) setSession("", "");
    else {
      tableEl.innerHTML = "";
      emptyEl.textContent = err.message;
      show(emptyEl, true);
    }
  }
}

function matchesFilter(j) {
  if (contentFilter === "inflight") return isInflight(j);
  if (contentFilter === "ready") return j.status === "READY";
  if (contentFilter === "failed") return j.status === "FAILED";
  return true;
}

function matchesSearch(j) {
  if (!searchQuery) return true;
  return String(j.original_filename || "").toLowerCase().includes(searchQuery)
    || String(j.id || "").toLowerCase().includes(searchQuery)
    || String(j.status || "").toLowerCase().includes(searchQuery);
}

function renderJobs() {
  const inflight = jobsCache.filter(isInflight);
  const ready = jobsCache.filter((j) => j.status === "READY");
  const failed = jobsCache.filter((j) => j.status === "FAILED");
  $("kpi-queue").textContent = jobsCache.filter((j) => j.status === "QUEUED" || j.status === "UPLOADED").length;
  $("kpi-run").textContent = jobsCache.filter((j) => j.status === "PROCESSING").length;
  $("kpi-ready").textContent = ready.length;
  $("kpi-fail").textContent = failed.length;
  inflightCount.textContent = inflight.length ? inflight.length + " em andamento" : "Fila vazia";
  doneCount.textContent = jobsCache.length + (jobsCache.length === 1 ? " vídeo" : " vídeos");

  const recent = jobsCache.slice(0, 6);
  dashRecent.innerHTML = recent.length
    ? recent.map(recentItem).join("")
    : "<p class=\"empty\">Nenhum vídeo ainda. Clique em Enviar para começar.</p>";

  const rows = jobsCache.filter((j) => matchesFilter(j) && matchesSearch(j));
  tableEl.innerHTML = rows.map((j) => jobRow(j, false)).join("");
  emptyEl.textContent = searchQuery ? "Nenhum vídeo corresponde à pesquisa." : "Nenhum vídeo neste filtro.";
  show(emptyEl, rows.length === 0);
  hydrateThumbs();
}

function tickLive() {
  document.querySelectorAll("[data-live]").forEach((el) => {
    el.textContent = liveLabel(
      el.getAttribute("data-status"),
      el.getAttribute("data-created"),
      el.getAttribute("data-started")
    );
  });
}

function statusLabel(status) {
  switch (status) {
    case "QUEUED": return "Na fila";
    case "PROCESSING": return "Processando";
    case "READY": return "Pronto";
    case "FAILED": return "Falhou";
    case "UPLOADED": return "Enviado";
    default: return status || "";
  }
}

function thumbStyle(id) {
  let h = 0;
  const s = String(id || "");
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  const hue = 200 + (h % 40);
  return "background:linear-gradient(145deg,hsl(" + hue + ",55%,28%),hsl(" + (hue + 24) + ",45%,16%))";
}

function formatWhen(iso) {
  if (!iso) return "";
  return new Date(iso).toLocaleString("pt-BR", {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function shortId(id) {
  const s = String(id || "");
  return s.length > 12 ? s.slice(0, 8) : s;
}

function thumbMarkup(j, bar) {
  const cached = j.has_thumb ? thumbURLs.get(j.id) : "";
  const img = j.has_thumb
    ? "<img data-thumb=\"" + escapeHtml(j.id) + "\" alt=\"\"" + (cached ? " src=\"" + cached + "\"" : "") + ">"
    : "";
  const frameClass = cached ? " has-frame" : "";
  return (
    "<div class=\"thumb" + frameClass + "\" style=\"" + thumbStyle(j.id) + "\">" +
      img +
      "<svg class=\"play\" viewBox=\"0 0 24 24\"><path fill=\"#fff\" d=\"M8 6.8v10.4L18 12 8 6.8z\"/></svg>" +
      bar +
    "</div>"
  );
}

function hydrateThumbs() {
  document.querySelectorAll("img[data-thumb]").forEach(async (img) => {
    const id = img.getAttribute("data-thumb");
    if (thumbURLs.has(id)) {
      img.src = thumbURLs.get(id);
      img.closest(".thumb")?.classList.add("has-frame");
      return;
    }
    try {
      const res = await fetch("/videos/" + id + "/thumb", { headers: authHeaders(), cache: "no-store" });
      if (!res.ok) return;
      const url = URL.createObjectURL(await res.blob());
      thumbURLs.set(id, url);
      img.src = url;
      img.closest(".thumb")?.classList.add("has-frame");
    } catch (_) { /* keep placeholder */ }
  });
}

function recentItem(j) {
  const live = isInflight(j);
  const bar = live ? "<div class=\"bar\" aria-hidden=\"true\"><span></span></div>" : "";
  return (
    "<article class=\"recent-item\">" +
      thumbMarkup(j, bar) +
      "<div class=\"vmeta\">" +
        "<div class=\"vtitle\">" + escapeHtml(j.original_filename) + "</div>" +
        "<div class=\"vsub\">" + escapeHtml(statusLabel(j.status) + " · " + formatWhen(j.created_at)) + "</div>" +
      "</div>" +
      "<span class=\"badge " + escapeHtml(j.status) + "\">" + escapeHtml(statusLabel(j.status)) + "</span>" +
    "</article>"
  );
}

function jobRow(j, compact) {
  const live = isInflight(j);
  const ready = j.status === "READY";
  const failed = j.status === "FAILED";
  const when = formatWhen(j.created_at);
  const buttons = [];
  if (!compact && ready) {
    buttons.push(
      "<button type=\"button\" class=\"icon-btn row-zip\" data-zip=\"" + j.id + "\" title=\"Baixar ZIP\" aria-label=\"Baixar ZIP\">" +
      "<svg viewBox=\"0 0 24 24\"><path d=\"M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z\"/></svg></button>"
    );
  }
  if (!compact && (ready || failed)) {
    buttons.push(
      "<button type=\"button\" class=\"icon-btn row-del\" data-del=\"" + j.id + "\" title=\"Excluir\" aria-label=\"Excluir vídeo\">" +
      "<svg viewBox=\"0 0 24 24\"><path d=\"M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z\"/></svg></button>"
    );
  }
  const action = buttons.length ? "<div class=\"row-actions\">" + buttons.join("") + "</div>" : "";
  const bar = live
    ? "<div class=\"bar\" aria-hidden=\"true\"><span></span></div>"
    : "";
  const subParts = [];
  if (failed && j.error_message) subParts.push(j.error_message);
  else subParts.push(shortId(j.id));
  if (when) subParts.push(when);
  const sub = subParts.join(" · ");
  return (
    "<tr>" +
      "<td><div class=\"vcell\">" +
        thumbMarkup(j, bar) +
        "<div class=\"vmeta\"><div class=\"vtitle\">" + escapeHtml(j.original_filename) + "</div>" +
        "<div class=\"vsub\">" + escapeHtml(sub) + "</div>" +
        metricsRow(j, live) +
        "</div>" +
      "</div></td>" +
      "<td><span class=\"badge " + escapeHtml(j.status) + "\">" + escapeHtml(statusLabel(j.status)) + "</span></td>" +
      "<td>" + action + "</td>" +
    "</tr>"
  );
}

function metricsRow(j, live) {
  const chips = [];
  if (live) {
    chips.push(
      "<span class=\"metric\" data-live data-status=\"" + escapeHtml(j.status) +
      "\" data-created=\"" + escapeHtml(j.created_at || "") +
      "\" data-started=\"" + escapeHtml(j.started_at || "") + "\">" +
      escapeHtml(liveLabel(j.status, j.created_at, j.started_at)) + "</span>"
    );
  } else {
    if (j.processor) chips.push(chip(processorLabel(j.processor)));
    if (j.queue_wait_ms) chips.push(chip("Fila " + formatMs(j.queue_wait_ms)));
    if (j.process_ms || j.ffmpeg_ms) chips.push(chip("Processamento " + formatMs(j.process_ms || j.ffmpeg_ms)));
    if (j.total_ms) chips.push(chip("Total " + formatMs(j.total_ms)));
    if (j.zip_bytes) chips.push(chip("ZIP " + formatBytes(j.zip_bytes)));
    if (j.frame_count) chips.push(chip(j.frame_count + " frames"));
  }
  if (!chips.length) return "";
  return "<div class=\"metrics\">" + chips.join("") + "</div>";
}

function chip(text) {
  return "<span class=\"metric\">" + escapeHtml(text) + "</span>";
}

function processorLabel(p) {
  if (p === "gstreamer") return "GStreamer";
  return "FFmpeg";
}

function liveLabel(status, created, started) {
  const now = Date.now();
  if (status === "PROCESSING" && started) {
    return "Processando " + formatMs(now - new Date(started).getTime());
  }
  if (created) {
    return "Na fila " + formatMs(now - new Date(created).getTime());
  }
  return statusLabel(status);
}

function formatMs(ms) {
  if (ms == null || Number.isNaN(ms) || ms < 0) return "0 s";
  if (ms < 1000) return Math.round(ms) + " ms";
  if (ms < 60000) return (ms / 1000).toFixed(1) + " s";
  const m = Math.floor(ms / 60000);
  const s = Math.round((ms % 60000) / 1000);
  return m + " min " + s + " s";
}

function formatBytes(n) {
  const v = Number(n);
  if (!v) return "0 B";
  if (v < 1024) return v + " B";
  if (v < 1048576) return (v / 1024).toFixed(1) + " KB";
  return (v / 1048576).toFixed(2) + " MB";
}

async function downloadZip(id) {
  try {
    const res = await fetch("/videos/" + id + "/zip", { headers: authHeaders(), cache: "no-store" });
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
    banner(appMsg, err.message, "error");
  }
}

async function deleteJob(id) {
  const job = jobsCache.find((j) => j.id === id);
  const name = job ? job.original_filename : id;
  if (!window.confirm("Excluir \"" + name + "\"? O original, o ZIP e o registro no banco serão removidos.")) {
    return;
  }
  try {
    const res = await fetch("/videos/" + id, { method: "DELETE", headers: authHeaders() });
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(body.error || res.statusText);
    }
    const url = thumbURLs.get(id);
    if (url) {
      URL.revokeObjectURL(url);
      thumbURLs.delete(id);
    }
    banner(appMsg, "Vídeo excluído.", "ok");
    await loadJobs();
  } catch (err) {
    banner(appMsg, err.message, "error");
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
