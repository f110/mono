const STORAGE_KEY = "issue-tracker-mock-v3";

const STATUSES = [
  { id: "open", label: "Open" },
  { id: "in_progress", label: "In Progress" },
  { id: "done", label: "Done" },
  { id: "wont_fix", label: "Won't Fix" },
];
const PRIORITIES = ["P0", "P1", "P2", "P3", "P4"];
const RELATION_TYPES = [
  { id: "parent_of", label: "親", inverse: "子" },
  { id: "blocks", label: "ブロックしている", inverse: "ブロックされている" },
  { id: "duplicate_of", label: "重複元", inverse: "重複" },
  { id: "relates_to", label: "関連", inverse: "関連" },
];

let db = load();
let currentActor = localStorage.getItem(STORAGE_KEY + ":actor") || "fumihiro";

function load() {
  const saved = localStorage.getItem(STORAGE_KEY);
  return saved ? JSON.parse(saved) : structuredClone(SEED);
}

function save() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(db));
}

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
}

function md(s) {
  const linked = String(s ?? "").replace(/(^|[^\w&])#(\d+)\b/g, (m, p, n) => `${p}[#${n}](#/${n})`);
  return DOMPurify.sanitize(marked.parse(linked));
}

function fmt(at) {
  const d = new Date(at);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

function fmtSize(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

const actorOf = (id) => db.actors.find((a) => a.id === id);
const operators = () => db.actors.filter((a) => a.kind !== "system");
const ticketOf = (id) => db.tickets.find((t) => t.id === Number(id));
const statusLabel = (id) => STATUSES.find((s) => s.id === id)?.label ?? id;
const componentName = (id) => db.components.find((c) => c.id === id)?.name;
const repoOf = (id) => db.repositories.find((r) => r.id === id);

function actorBadge(id) {
  const a = actorOf(id);
  if (!a) return `<span class="muted">未割り当て</span>`;
  if (a.kind === "agent") {
    return `<span class="actor agent" title="agent (owner: ${esc(a.owner)})">🤖 ${esc(a.name)}</span>`;
  }
  if (a.kind === "system") {
    return `<span class="actor system">⚙️ ${esc(a.name)}</span>`;
  }
  return `<span class="actor human">👤 ${esc(a.name)}</span>`;
}

function statusBadge(id) {
  return `<span class="status status-${esc(id)}">${esc(statusLabel(id))}</span>`;
}

function priorityBadge(p) {
  return p ? `<span class="priority priority-${esc(p)}">${esc(p)}</span>` : `<span class="muted">-</span>`;
}

function componentBadge(id) {
  return id ? `<span class="component">${esc(componentName(id))}</span>` : `<span class="muted">未分類</span>`;
}

function now() {
  return new Date().toISOString();
}

function record(ticketId, kind, fields) {
  db.activities.push({ ticket: ticketId, actor: currentActor, kind, at: now(), ...fields });
  const t = ticketOf(ticketId);
  if (t) t.updatedAt = now();
  save();
}

function parseHash() {
  const raw = location.hash.replace(/^#/, "") || "/";
  const [path, query] = raw.split("?");
  return { path, params: new URLSearchParams(query || "") };
}

function render() {
  const { path, params } = parseHash();
  const app = document.getElementById("app");
  renderActorSelect();
  let m;
  if (path === "/" || path === "") {
    app.innerHTML = listView(params);
    bindList();
  } else if (path === "/new") {
    app.innerHTML = newView();
    bindNew();
  } else if ((m = path.match(/^\/(\d+)$/))) {
    const t = ticketOf(m[1]);
    app.innerHTML = t ? detailView(t) : `<p>#${esc(m[1])} は存在しません</p>`;
    if (t) bindDetail(t);
  } else {
    app.innerHTML = `<p>Not Found</p>`;
  }
  window.scrollTo(0, 0);
}

function renderActorSelect() {
  const sel = document.getElementById("actor");
  sel.innerHTML = operators()
    .map((a) => `<option value="${esc(a.id)}" ${a.id === currentActor ? "selected" : ""}>${a.kind === "agent" ? "🤖" : "👤"} ${esc(a.name)}</option>`)
    .join("");
}

function filterTickets(params) {
  const q = (params.get("q") || "").toLowerCase();
  const status = params.get("status");
  const priority = params.get("priority");
  const component = params.get("component");
  const assignee = params.get("assignee");
  return db.tickets
    .filter((t) => {
      if (status === "active" && !["open", "in_progress"].includes(t.status)) return false;
      if (status && status !== "active" && status !== "all" && t.status !== status) return false;
      if (priority && t.priority !== priority) return false;
      if (component === "none" && t.component) return false;
      if (component && component !== "none" && t.component !== component) return false;
      if (assignee === "none" && t.assignee) return false;
      if (assignee && assignee !== "none" && t.assignee !== assignee) return false;
      if (q && !t.title.toLowerCase().includes(q) && !t.body.toLowerCase().includes(q) && `#${t.id}` !== q) return false;
      return true;
    })
    .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
}

function options(list, selected, emptyLabel) {
  const head = emptyLabel !== undefined ? `<option value="">${esc(emptyLabel)}</option>` : "";
  return head + list.map(([v, l]) => `<option value="${esc(v)}" ${v === selected ? "selected" : ""}>${esc(l)}</option>`).join("");
}

function listView(params) {
  if (!params.has("status")) params.set("status", "active");
  const tickets = filterTickets(params);
  const rows = tickets
    .map(
      (t) => `<tr>
        <td class="num"><a href="#/${t.id}">#${t.id}</a></td>
        <td><a href="#/${t.id}">${esc(t.title)}</a></td>
        <td>${statusBadge(t.status)}</td>
        <td>${priorityBadge(t.priority)}</td>
        <td>${componentBadge(t.component)}</td>
        <td>${actorBadge(t.assignee)}</td>
        <td class="muted">${fmt(t.updatedAt)}</td>
      </tr>`,
    )
    .join("");
  return `
    <form id="filter" class="filter">
      <input name="q" type="search" placeholder="タイトル・本文を検索" value="${esc(params.get("q") || "")}">
      <select name="status">${options(
        [["active", "未完了"], ["all", "すべて"], ...STATUSES.map((s) => [s.id, s.label])],
        params.get("status"),
      )}</select>
      <select name="priority">${options(PRIORITIES.map((p) => [p, p]), params.get("priority"), "優先度: すべて")}</select>
      <select name="component">${options(
        [["none", "未分類"], ...db.components.map((c) => [c.id, c.name])],
        params.get("component"),
        "分類: すべて",
      )}</select>
      <select name="assignee">${options(
        [["none", "未割り当て"], ...operators().map((a) => [a.id, (a.kind === "agent" ? "🤖 " : "👤 ") + a.name])],
        params.get("assignee"),
        "担当: すべて",
      )}</select>
      <button type="submit">検索</button>
    </form>
    <p class="muted">${tickets.length} 件 ・ 検索条件はURLに反映されるので共有できます</p>
    <table class="tickets">
      <thead><tr><th>#</th><th>タイトル</th><th>ステータス</th><th>優先度</th><th>分類</th><th>担当</th><th>更新</th></tr></thead>
      <tbody>${rows || `<tr><td colspan="7" class="muted">該当なし</td></tr>`}</tbody>
    </table>`;
}

function bindList() {
  const form = document.getElementById("filter");
  const apply = () => {
    const p = new URLSearchParams();
    for (const [k, v] of new FormData(form)) if (v) p.set(k, v);
    location.hash = `#/?${p}`;
  };
  form.addEventListener("submit", (e) => {
    e.preventDefault();
    apply();
  });
  form.querySelectorAll("select").forEach((s) => s.addEventListener("change", apply));
}

function newView() {
  return `
    <h2>新規チケット</h2>
    <form id="new" class="new">
      <label>タイトル<input name="title" required></label>
      <div class="row">
        <label>優先度<select name="priority">${options(PRIORITIES.map((p) => [p, p]), "P2", "なし")}</select></label>
        <label>分類<select name="component">${options(db.components.map((c) => [c.id, c.name]), "", "未分類")}</select></label>
        <label>担当<select name="assignee">${options(operators().map((a) => [a.id, a.name]), "", "未割り当て")}</select></label>
      </div>
      ${editor("body", "")}
      <button type="submit">作成</button>
    </form>`;
}

function bindNew() {
  bindEditors();
  document.getElementById("new").addEventListener("submit", (e) => {
    e.preventDefault();
    const f = new FormData(e.target);
    const id = db.nextId++;
    db.tickets.push({
      id,
      title: f.get("title"),
      body: f.get("body"),
      status: "open",
      priority: f.get("priority") || null,
      component: f.get("component") || null,
      assignee: f.get("assignee") || null,
      reporter: currentActor,
      createdAt: now(),
      updatedAt: now(),
    });
    record(id, "create", {});
    location.hash = `#/${id}`;
  });
}

function editor(name, value) {
  return `
    <div class="editor">
      <div class="tabs"><button type="button" data-tab="write" class="active">Write</button><button type="button" data-tab="preview">Preview</button></div>
      <textarea name="${name}" rows="8" placeholder="Markdownで記述できます。#123 でチケットにリンク">${esc(value)}</textarea>
      <div class="preview markdown" hidden></div>
    </div>`;
}

function bindEditors() {
  document.querySelectorAll(".editor").forEach((ed) => {
    const ta = ed.querySelector("textarea");
    const pv = ed.querySelector(".preview");
    ed.querySelectorAll("[data-tab]").forEach((b) =>
      b.addEventListener("click", () => {
        ed.querySelectorAll("[data-tab]").forEach((x) => x.classList.toggle("active", x === b));
        const preview = b.dataset.tab === "preview";
        pv.innerHTML = preview ? md(ta.value) : "";
        pv.hidden = !preview;
        ta.hidden = preview;
      }),
    );
  });
}

function relationsOf(t) {
  const out = [];
  for (const r of db.relations) {
    const type = RELATION_TYPES.find((x) => x.id === r.type);
    if (r.from === t.id) out.push({ label: type.label, other: r.to, rel: r });
    if (r.to === t.id) out.push({ label: type.inverse, other: r.from, rel: r });
  }
  return out;
}

function ciBadge(ci) {
  if (!ci) return "";
  const icon = { success: "✅", failure: "❌", running: "⏳" }[ci.status] || "";
  return `<a class="ci ci-${esc(ci.status)}" href="${esc(ci.url)}" title="Build">${icon} ${esc(ci.status)}</a>`;
}

function commitRow(c) {
  const repo = repoOf(c.repo);
  return `<tr>
    <td>${esc(repo.name)}</td>
    <td><a href="${esc(repo.url)}/commit/${esc(c.commit)}"><code>${esc(c.commit.slice(0, 8))}</code></a></td>
    <td><code title="jj change-id">${esc(c.changeId.slice(0, 12))}</code></td>
    <td>${esc(c.subject)}</td>
    <td><code>${esc(c.trailer)}</code></td>
    <td>${ciBadge(c.ci)}</td>
    <td class="muted">${fmt(c.at)}</td>
  </tr>`;
}

function pullRequestRow(p) {
  const repo = repoOf(p.repo);
  return `<tr>
    <td>${esc(repo.name)}</td>
    <td><a href="${esc(repo.url)}/pull/${p.number}">#${p.number}</a></td>
    <td>${esc(p.title)}</td>
    <td><span class="pr-state pr-${esc(p.state)}">${esc(p.state)}</span></td>
    <td>${actorBadge(p.author)}</td>
    <td>${ciBadge(p.ci)}</td>
    <td class="muted">${fmt(p.at)}</td>
  </tr>`;
}

function activityItem(a) {
  const who = actorBadge(a.actor);
  const when = `<span class="muted">${fmt(a.at)}</span>`;
  switch (a.kind) {
    case "comment":
      return `<div class="comment"><div class="comment-head">${who} ${when}</div><div class="markdown">${md(a.body)}</div></div>`;
    case "create":
      return `<div class="event">${who} がチケットを作成 ${when}</div>`;
    case "status":
      return `<div class="event">${who} がステータスを ${statusBadge(a.from)} → ${statusBadge(a.to)} に変更 ${when}</div>`;
    case "priority":
      return `<div class="event">${who} が優先度を ${priorityBadge(a.from)} → ${priorityBadge(a.to)} に変更 ${when}</div>`;
    case "component":
      return `<div class="event">${who} が分類を ${componentBadge(a.from)} → ${componentBadge(a.to)} に変更 ${when}</div>`;
    case "assignee":
      return `<div class="event">${who} が担当を ${actorBadge(a.from)} → ${actorBadge(a.to)} に変更 ${when}</div>`;
    case "title":
      return `<div class="event">${who} がタイトルを変更 ${when}</div>`;
    case "body":
      return `<div class="event">${who} が本文を編集 ${when}</div>`;
    case "relation":
      return `<div class="event">${who} が関連を追加: ${md(a.body).replace(/<\/?p>/g, "")} ${when}</div>`;
    case "relation_removed":
      return `<div class="event">${who} が関連を削除: ${md(a.body).replace(/<\/?p>/g, "")} ${when}</div>`;
    case "link":
      return `<div class="event">${who} の差分が紐づけられました: <code>${esc(a.body)}</code> ${when}</div>`;
    case "attachment":
      return `<div class="event">${who} が添付ファイルを追加: ${esc(a.body)} ${when}</div>`;
    default:
      return "";
  }
}

function detailView(t) {
  const relations = relationsOf(t);
  const byAtDesc = (a, b) => b.at.localeCompare(a.at);
  const pullRequests = db.pullRequests.filter((p) => p.ticket === t.id).sort(byAtDesc);
  const commits = db.commits.filter((c) => c.ticket === t.id).sort(byAtDesc);
  const attachments = db.attachments.filter((x) => x.ticket === t.id);
  const activities = db.activities.filter((a) => a.ticket === t.id).sort((a, b) => a.at.localeCompare(b.at));
  const permalink = `${location.origin}${location.pathname}#/${t.id}`;

  return `
    <div class="detail">
      <div class="main">
        <div class="title-row">
          <h2><span class="muted">#${t.id}</span> <span id="title-text">${esc(t.title)}</span></h2>
          <button type="button" id="edit-title" class="link">編集</button>
        </div>
        <p class="muted">${actorBadge(t.reporter)} が ${fmt(t.createdAt)} に作成 ・ <a href="#/${t.id}" title="${esc(permalink)}">permalink</a></p>

        <section class="body">
          <div id="body-view" class="markdown">${md(t.body)}</div>
          <button type="button" id="edit-body" class="link">本文を編集</button>
          <form id="body-form" hidden>${editor("body", t.body)}<button type="submit">保存</button></form>
        </section>

        <section>
          <h3>Pull Requests <span class="muted">(${pullRequests.length})</span></h3>
          ${
            pullRequests.length
              ? `<table class="changes"><thead><tr><th>Repo</th><th>PR</th><th>Title</th><th>State</th><th>Author</th><th>CI</th><th></th></tr></thead><tbody>${pullRequests.map(pullRequestRow).join("")}</tbody></table>`
              : `<p class="muted">まだありません</p>`
          }
        </section>

        <section>
          <h3>Commits <span class="muted">(${commits.length})</span></h3>
          ${
            commits.length
              ? `<table class="changes"><thead><tr><th>Repo</th><th>Commit</th><th>Change-Id</th><th>Subject</th><th>Trailer</th><th>CI</th><th></th></tr></thead><tbody>${commits.map(commitRow).join("")}</tbody></table>`
              : `<p class="muted">まだありません</p>`
          }
          <p class="hint">
            コミットメッセージに trailer <code>Refs: ${t.id}</code> を付けると紐づきます。<code>Fixes: ${t.id}</code> の場合はデフォルトブランチへの push 時に Done になります。<br>
            PR に含まれるコミットに trailer がある場合は PR が、デフォルトブランチに push されたコミットはコミットが紐づきます。
          </p>
        </section>

        <section>
          <h3>添付ファイル <span class="muted">(${attachments.length})</span></h3>
          <ul class="attachments">
            ${attachments.map((x) => `<li><a href="#">📎 ${esc(x.name)}</a> <span class="muted">${fmtSize(x.size)} ・ ${actorBadge(x.uploader)} ・ ${fmt(x.at)}</span></li>`).join("")}
          </ul>
          <label class="upload">ファイルを追加 <input type="file" id="upload" multiple></label>
        </section>

        <section>
          <h3>アクティビティ</h3>
          <div class="timeline">${activities.map(activityItem).join("")}</div>
          <form id="comment" class="comment-form">
            ${editor("body", "")}
            <div class="row"><span class="muted">投稿者: ${actorBadge(currentActor)}</span><button type="submit">コメント</button></div>
          </form>
        </section>
      </div>

      <aside class="side">
        <label>ステータス<select id="f-status">${options(STATUSES.map((s) => [s.id, s.label]), t.status)}</select></label>
        <label>優先度<select id="f-priority">${options(PRIORITIES.map((p) => [p, p]), t.priority, "なし")}</select></label>
        <label>分類<select id="f-component">${options(db.components.map((c) => [c.id, c.name]), t.component, "未分類")}</select></label>
        <label>担当<select id="f-assignee">${options(
          operators().map((a) => [a.id, (a.kind === "agent" ? "🤖 " : "👤 ") + a.name]),
          t.assignee,
          "未割り当て",
        )}</select></label>

        <div class="relations">
          <h4>関連チケット</h4>
          <ul>
            ${relations
              .map(({ label, other, rel }) => {
                const o = ticketOf(other);
                return `<li><span class="muted">${esc(label)}</span> <a href="#/${other}">#${other}</a> ${o ? statusBadge(o.status) : ""}<br><span class="rel-title">${esc(o?.title)}</span>
                  <button type="button" class="link remove-rel" data-from="${rel.from}" data-type="${rel.type}" data-to="${rel.to}">削除</button></li>`;
              })
              .join("")}
          </ul>
          <form id="add-rel" class="add-rel">
            <select name="type">${RELATION_TYPES.flatMap((r) => [
              `<option value="${r.id}">${esc(r.label)}</option>`,
              r.id === "relates_to" ? "" : `<option value="~${r.id}">${esc(r.inverse)}</option>`,
            ]).join("")}</select>
            <input name="to" type="number" min="1" placeholder="#">
            <button type="submit">追加</button>
          </form>
        </div>
      </aside>
    </div>`;
}

function bindDetail(t) {
  bindEditors();

  const fields = [
    ["status", "f-status"],
    ["priority", "f-priority"],
    ["component", "f-component"],
    ["assignee", "f-assignee"],
  ];
  for (const [key, id] of fields) {
    document.getElementById(id).addEventListener("change", (e) => {
      const to = e.target.value || null;
      const from = t[key];
      if (from === to) return;
      t[key] = to;
      record(t.id, key, { from, to });
      render();
    });
  }

  document.getElementById("edit-title").addEventListener("click", () => {
    const title = prompt("タイトル", t.title);
    if (!title || title === t.title) return;
    t.title = title;
    record(t.id, "title", {});
    render();
  });

  document.getElementById("edit-body").addEventListener("click", () => {
    document.getElementById("body-view").hidden = true;
    document.getElementById("edit-body").hidden = true;
    document.getElementById("body-form").hidden = false;
  });
  document.getElementById("body-form").addEventListener("submit", (e) => {
    e.preventDefault();
    t.body = new FormData(e.target).get("body");
    record(t.id, "body", {});
    render();
  });

  document.getElementById("comment").addEventListener("submit", (e) => {
    e.preventDefault();
    const body = new FormData(e.target).get("body").trim();
    if (!body) return;
    record(t.id, "comment", { body });
    render();
  });

  document.getElementById("upload").addEventListener("change", (e) => {
    for (const file of e.target.files) {
      db.attachments.push({ ticket: t.id, name: file.name, size: file.size, uploader: currentActor, at: now() });
      record(t.id, "attachment", { body: file.name });
    }
    render();
  });

  document.getElementById("add-rel").addEventListener("submit", (e) => {
    e.preventDefault();
    const f = new FormData(e.target);
    const other = Number(f.get("to"));
    if (!ticketOf(other) || other === t.id) {
      alert("存在するチケット番号を指定してください");
      return;
    }
    let type = f.get("type");
    let rel = { from: t.id, type, to: other };
    if (type.startsWith("~")) {
      type = type.slice(1);
      rel = { from: other, type, to: t.id };
    }
    db.relations.push(rel);
    const desc = `#${rel.from} ${RELATION_TYPES.find((r) => r.id === rel.type).id} #${rel.to}`;
    record(rel.from, "relation", { body: desc });
    record(rel.to, "relation", { body: desc });
    render();
  });

  document.querySelectorAll(".remove-rel").forEach((b) =>
    b.addEventListener("click", () => {
      const { from, type, to } = b.dataset;
      db.relations = db.relations.filter((r) => !(r.from === Number(from) && r.type === type && r.to === Number(to)));
      const desc = `#${from} ${type} #${to}`;
      record(Number(from), "relation_removed", { body: desc });
      record(Number(to), "relation_removed", { body: desc });
      render();
    }),
  );
}

document.getElementById("actor").addEventListener("change", (e) => {
  currentActor = e.target.value;
  localStorage.setItem(STORAGE_KEY + ":actor", currentActor);
  render();
});

document.getElementById("reset").addEventListener("click", () => {
  if (!confirm("モックのデータを初期状態に戻しますか？")) return;
  localStorage.removeItem(STORAGE_KEY);
  db = load();
  render();
});

window.addEventListener("hashchange", render);
render();
