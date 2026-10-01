"use strict";
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

// src/main.ts
var main_exports = {};
__export(main_exports, {
  default: () => AtlasPlugin
});
module.exports = __toCommonJS(main_exports);
var import_obsidian12 = require("obsidian");

// src/badges.ts
var import_obsidian = require("obsidian");
var BADGE = "atlas-badge";
function badgeFor(app, path) {
  const file = app.vault.getAbstractFileByPath(path);
  if (!(file instanceof import_obsidian.TFile) || file.extension !== "md") return null;
  const fm = app.metadataCache.getFileCache(file)?.frontmatter;
  if (!fm) return null;
  if ((fm.type === "stub" || fm.type === "chord") && typeof fm.status === "string") {
    return { kind: "status", value: fm.blocked ? "blocked" : fm.status };
  }
  if (fm.type === "event" && typeof fm.kind === "string") return { kind: "event", value: fm.kind };
  if (fm.type === "session" && typeof fm.status === "string") return { kind: "status", value: fm.status };
  return null;
}
function ownMutation(m) {
  const inBadge = (n) => n instanceof Element ? n.classList.contains(BADGE) || n.closest(`.${BADGE}`) !== null : n.parentElement?.closest(`.${BADGE}`) != null;
  const nodes = [...Array.from(m.addedNodes), ...Array.from(m.removedNodes)];
  return inBadge(m.target) || nodes.length > 0 && nodes.every(inBadge);
}
var Badges = class extends import_obsidian.Component {
  constructor(app) {
    super();
    this.app = app;
  }
  app;
  observers = [];
  observed = [];
  enabled = false;
  refresh = (0, import_obsidian.debounce)(() => this.apply(), 300, true);
  onload() {
    this.registerEvent(this.app.metadataCache.on("changed", () => this.refresh()));
    this.registerEvent(this.app.vault.on("rename", () => this.refresh()));
    this.registerEvent(this.app.vault.on("delete", () => this.refresh()));
    this.registerEvent(this.app.workspace.on("layout-change", () => this.refresh()));
    this.app.workspace.onLayoutReady(() => this.refresh());
  }
  onunload() {
    this.disconnect();
    this.clear();
  }
  setEnabled(on) {
    this.enabled = on;
    if (on) this.refresh();
    else {
      this.disconnect();
      this.clear();
    }
  }
  explorers() {
    return this.app.workspace.getLeavesOfType("file-explorer").map((leaf) => leaf.view.containerEl);
  }
  apply() {
    if (!this.enabled) return;
    this.observe();
    for (const root of this.explorers()) {
      root.querySelectorAll(".nav-file-title[data-path]").forEach((title) => {
        const path = title.getAttribute("data-path") ?? "";
        const badge = badgeFor(this.app, path);
        let span = title.querySelector(`:scope > .${BADGE}`);
        if (!badge) {
          span?.remove();
          return;
        }
        if (!span) span = title.createSpan({ cls: BADGE });
        if (span.dataset.kind !== badge.kind) span.dataset.kind = badge.kind;
        if (span.dataset.value !== badge.value) span.dataset.value = badge.value;
        if (span.textContent !== badge.value) span.textContent = badge.value;
      });
    }
  }
  // A folder opened in the explorer renders new rows without a workspace event.
  observe() {
    const roots = this.explorers();
    if (roots.length === this.observed.length && roots.every((r, i) => r === this.observed[i])) return;
    this.disconnect();
    this.observed = roots;
    for (const root of roots) {
      const o = new MutationObserver((records) => {
        if (records.every(ownMutation)) return;
        this.refresh();
      });
      o.observe(root, { childList: true, subtree: true });
      this.observers.push(o);
    }
  }
  disconnect() {
    this.observers.forEach((o) => o.disconnect());
    this.observers = [];
    this.observed = [];
  }
  clear() {
    for (const root of this.explorers()) {
      root.querySelectorAll(`.${BADGE}`).forEach((el) => el.remove());
    }
  }
};

// src/changebar.ts
var import_obsidian2 = require("obsidian");

// src/helpers.ts
function binaryCandidates(home) {
  return [`${home}/.atlas/bin/atlas-obsidian`, `${home}/go/bin/atlas-obsidian`];
}
function chooseBinary(override, candidates, exists, home) {
  const set = override.trim();
  if (set !== "") return expandHome(set, home);
  return candidates.find(exists) ?? null;
}
function expandHome(path, home) {
  if (path === "~") return home;
  if (path.startsWith("~/")) return home + path.slice(1);
  return path;
}
function errorMessage(stderr) {
  const lines = stderr.split("\n").map((l) => l.trim()).filter((l) => l !== "");
  for (let i = lines.length - 1; i >= 0; i--) {
    if (lines[i].startsWith("atlas: ")) return lines[i].slice("atlas: ".length);
  }
  return lines[lines.length - 1] ?? "";
}
function plural(n, one, many) {
  return `${n} ${n === 1 ? one : many}`;
}
function syncSummary(s) {
  const parts = [];
  const add = (list, one, many) => {
    const n = list?.length ?? 0;
    if (n) parts.push(plural(n, one, many));
  };
  add(s.threads, "thread document", "thread documents");
  add(s.knowledge, "knowledge document", "knowledge documents");
  add(s.moved, "document moved back", "documents moved back");
  add(s.lost, "lost session", "lost sessions");
  add(s.sessions, "session callout", "session callouts");
  if (s.settings) parts.push("the harness settings");
  if (s.views) parts.push(plural(s.views, "view", "views"));
  if (parts.length === 0) return "Nothing to heal.";
  return `Synced ${parts.join(", ")}.`;
}
function syncedPaths(s) {
  return [...s.threads ?? [], ...s.knowledge ?? [], ...s.moved ?? [], ...s.lost ?? [], ...s.sessions ?? []];
}
function countsLine(counts) {
  if (typeof counts === "string") {
    return counts.split(",").map((p) => p.trim()).filter((p) => p !== "" && !/^0\s/.test(p)).join(", ");
  }
  if (counts && typeof counts === "object") {
    const names = {
      create: "create",
      modify: "modify",
      promote: "promote",
      rename: "rename",
      remove: "remove",
      confirm: "confirm",
      retag: "retag",
      link_rewrites: "link rewrites",
      tag_rewrites: "tag rewrites"
    };
    return Object.entries(names).map(([key, name]) => [counts[key], name]).filter(([n]) => typeof n === "number" && n > 0).map(([n, name]) => `${n} ${name}`).join(", ");
  }
  return "";
}
function formatAgo(when, now) {
  if (when === void 0 || when === "") return "";
  const t = when instanceof Date ? when : new Date(String(when));
  if (Number.isNaN(t.getTime())) return "";
  const s = Math.max(0, Math.floor((now.getTime() - t.getTime()) / 1e3));
  if (s < 60) return "just now";
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} min ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} h ago`;
  const d = Math.floor(h / 24);
  return d === 1 ? "1 day ago" : `${d} days ago`;
}
function linkTitle(value) {
  if (typeof value !== "string") return "";
  const m = /^\s*\[\[([^\]|#]*)(?:[#|][^\]]*)?\]\]\s*$/.exec(value);
  return (m ? m[1] : value).trim();
}
function asList(value) {
  if (Array.isArray(value)) return value.filter((v) => typeof v === "string");
  if (typeof value === "string" && value !== "") return [value];
  return [];
}
function lastProgressLine(markdown) {
  let inside = false;
  let last = "";
  for (const raw of markdown.split("\n")) {
    const line = raw.trim();
    if (/^#{1,6}\s/.test(line)) {
      if (inside) break;
      inside = /^##\s+Progress\s*$/i.test(line);
      continue;
    }
    if (inside && line !== "") last = line.replace(/^[-*+]\s+(\[.\]\s+)?/, "");
  }
  return last;
}
var TASK_LINE = /^\s*(?:[-*+]|\d+[.)])\s+\[.\]\s/;
var MENTION = /@atlas(?![\w-])/g;
function mentionRanges(line) {
  if (!TASK_LINE.test(line)) return [];
  return textMentions(line);
}
function textMentions(text) {
  const out = [];
  for (const m of text.matchAll(MENTION)) {
    const i = m.index ?? 0;
    if (i > 0 && /[\w@.]/.test(text[i - 1])) continue;
    out.push([i, i + m[0].length]);
  }
  return out;
}
function isWatchedPath(path) {
  return path.endsWith(".md") && !path.startsWith("views/") && !path.startsWith(".");
}
function waitingLabel(n) {
  return n === 1 ? "Atlas: 1 session waits" : `Atlas: ${n} sessions wait`;
}
function cssString(s) {
  return `"${s.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, "\\a ")}"`;
}
var TAG_FOLDER = "views/tags/";
function tagTitle(tag) {
  return "Tag \xB7 " + tag.split("/").join(" \u203A ");
}
function tagOfFolder(folder) {
  if (!folder.startsWith(TAG_FOLDER)) return null;
  const tag = folder.slice(TAG_FOLDER.length);
  return tag === "" ? null : tag;
}
function tagViewPath(tag) {
  return `${TAG_FOLDER}${tag}/${tagTitle(tag)}.md`;
}
function isTagView(path) {
  const i = path.lastIndexOf("/");
  if (i < 0) return false;
  const tag = tagOfFolder(path.slice(0, i));
  return tag !== null && tagViewPath(tag) === path;
}
function normalTag(t) {
  return t.trim().replace(/^#/, "").toLowerCase();
}
function holds(list, t) {
  return list.some((x) => x === t || x.startsWith(t + "/"));
}
function expandTags(list) {
  const out = /* @__PURE__ */ new Set();
  for (const t of list) {
    const parts = t.split("/");
    for (let i = 1; i <= parts.length; i++) out.add(parts.slice(0, i).join("/"));
  }
  return [...out];
}
function relativeTag(tag, chosen) {
  const above = chosen.filter((c) => tag.startsWith(c + "/")).sort((a, b) => b.length - a.length)[0];
  return above ? "\u203A " + tag.slice(above.length + 1).split("/").join(" \u203A ") : "#" + tag;
}
function narrow(docs, chosen) {
  const matches = docs.filter((d) => chosen.every((t) => holds(d.tags, t)));
  const counts = /* @__PURE__ */ new Map();
  const skip = new Set(expandTags(chosen));
  for (const d of matches) {
    for (const t of expandTags(d.tags)) {
      if (skip.has(t)) continue;
      counts.set(t, (counts.get(t) ?? 0) + 1);
    }
  }
  const withTags = [...counts.entries()].map(([tag, count]) => ({ tag, count })).sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag));
  return { matches, with: withTags };
}
function topTags(docs) {
  const counts = /* @__PURE__ */ new Map();
  for (const d of docs) {
    for (const t of expandTags(d.tags)) {
      if (!t.includes("/")) counts.set(t, (counts.get(t) ?? 0) + 1);
    }
  }
  return [...counts.entries()].map(([tag, count]) => ({ tag, count })).sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag));
}
var ENDED_STATUS = /* @__PURE__ */ new Set(["closed", "dropped", "resolved"]);
var NAV_GROUPS = [
  { name: "Open chords", test: (d) => d.type === "chord" && !ENDED_STATUS.has(d.status) },
  { name: "Open threads", test: (d) => d.type === "stub" && !ENDED_STATUS.has(d.status) },
  { name: "Topics", test: (d) => d.type === "topic" },
  { name: "Sources", test: (d) => d.type === "source" },
  { name: "Repositories", test: (d) => d.type === "repository" },
  { name: "Specs, tasks, and verifications", test: (d) => d.type === "spec" || d.type === "tasks" || d.type === "verification" },
  { name: "Ended threads and chords", test: (d) => d.type === "stub" || d.type === "chord" },
  { name: "Events", test: (d) => d.type === "event" }
];
function groupDocs(docs) {
  const out = NAV_GROUPS.map((g) => ({ name: g.name, docs: [] }));
  for (const d of docs) {
    const i = NAV_GROUPS.findIndex((g) => g.test(d));
    if (i >= 0) out[i].docs.push(d);
  }
  for (const g of out) g.docs.sort((a, b) => a.title.localeCompare(b.title));
  return out.filter((g) => g.docs.length > 0);
}
function repoBlock(source) {
  const [id, path] = source.trim().split(" \xB7 ");
  return { id: (id ?? "").trim(), path: (path ?? "").trim() };
}
function layoutOf(fields) {
  const n = Number(fields?.layout ?? 0);
  return Number.isFinite(n) ? n : 0;
}
var LAYOUT = 4;
function layoutName(layout) {
  if (layout >= LAYOUT) return "8.0";
  return layout === 3 ? "7.x" : "6.x";
}
function handoffLine(type, id) {
  return `Resume Atlas ${type === "chord" ? "chord" : "thread"} ${id}`;
}
var ENDED = ["closed", "dropped", "resolved"];
function barStatus(d) {
  const parts = [d.blocked ? `${d.status}, blocked` : d.status];
  if (d.type === "stub" && d.tasks) parts.push(`${d.tasks} tasks`);
  if (d.type === "chord" && d.threads) parts.push(`${d.threads} threads closed`);
  return parts.join(" \xB7 ");
}
function barButtons(d) {
  const out = [];
  const ended = ENDED.includes(d.status);
  if (!ended) {
    out.push({ id: "agent", label: "Start agent" });
    if (d.type === "chord") out.push({ id: "new", label: "New thread" });
    out.push({ id: "handoff", label: "Copy hand-off" });
  }
  if (d.type === "chord") out.push({ id: "canvas", label: "Canvas" });
  if (d.status === "dropped" || d.type === "stub" && d.status === "resolved") {
    out.push({ id: "reopen", label: "Reopen" });
    return out;
  }
  if (ended) return out;
  if (d.type === "stub") out.push(d.blocked ? { id: "unblock", label: "Unblock" } : { id: "block", label: "Block" });
  out.push({ id: "drop", label: "Drop" });
  return out;
}
function chordOfCanvas(path) {
  const m = /^chords\/([^/]+)\.canvas$/.exec(path);
  return m ? m[1] : null;
}
function canvasSummary(s) {
  if (!s.differs) return "Saved";
  const names = s.threads ?? [];
  const n = names.length;
  if (n === 0) return "Not saved";
  const shown = names.slice(0, 2).join(", ");
  return `${plural(n, "thread moves", "threads move")}: ${shown}${n > 2 ? `, +${n - 2}` : ""}`;
}

// src/changebar.ts
var BAR = "atlas-change-bar";
function proposed(app, view) {
  if (!view.file) return null;
  const fm = app.metadataCache.getFileCache(view.file)?.frontmatter;
  if (!fm || fm.type !== "change" || fm.status !== "proposed" || !fm.id) return null;
  return { id: String(fm.id), counts: countsLine(fm.counts) };
}
var ChangeBar = class extends import_obsidian2.Component {
  constructor(plugin) {
    super();
    this.plugin = plugin;
  }
  plugin;
  busy = false;
  refresh = (0, import_obsidian2.debounce)(() => this.update(), 100, true);
  get app() {
    return this.plugin.app;
  }
  onload() {
    const ws = this.app.workspace;
    this.registerEvent(ws.on("active-leaf-change", () => this.refresh()));
    this.registerEvent(ws.on("file-open", () => this.refresh()));
    this.registerEvent(ws.on("layout-change", () => this.refresh()));
    this.registerEvent(this.app.metadataCache.on("changed", () => this.refresh()));
    ws.onLayoutReady(() => this.refresh());
  }
  onunload() {
    document.querySelectorAll(`.${BAR}`).forEach((el) => el.remove());
  }
  update() {
    for (const leaf of this.app.workspace.getLeavesOfType("markdown")) {
      if (leaf.view instanceof import_obsidian2.MarkdownView) this.updateView(leaf.view);
    }
  }
  updateView(view) {
    const change = proposed(this.app, view);
    const existing = view.containerEl.querySelector(`:scope > .${BAR}`);
    if (!change) {
      existing?.remove();
      return;
    }
    const key = `${change.id}
${change.counts}`;
    if (existing?.dataset.key === key) return;
    existing?.remove();
    const bar = createDiv({ cls: BAR });
    bar.dataset.key = key;
    bar.createSpan({ cls: "atlas-change-bar-label", text: "Proposed change" });
    if (change.counts) bar.createSpan({ cls: "atlas-change-bar-counts", text: change.counts });
    const buttons = bar.createDiv({ cls: "atlas-change-bar-buttons" });
    const apply = buttons.createEl("button", { cls: "mod-cta", text: "Apply" });
    const reject = buttons.createEl("button", { cls: "mod-warning", text: "Reject" });
    apply.onclick = () => void this.apply(view, change.id, [apply, reject]);
    reject.onclick = () => new ReasonModal(this.app, (reason) => void this.reject(change.id, reason, [apply, reject])).open();
    view.containerEl.insertBefore(bar, view.contentEl);
  }
  async run(buttons, action) {
    if (this.busy) return;
    this.busy = true;
    buttons.forEach((b) => b.disabled = true);
    try {
      await action();
    } catch (e) {
      new import_obsidian2.Notice(`Atlas: ${e.message}`);
    } finally {
      this.busy = false;
      buttons.forEach((b) => b.disabled = false);
      this.refresh();
    }
  }
  apply(view, id, buttons) {
    return this.run(buttons, async () => {
      await view.save();
      const p = await this.plugin.atlas(["change", "apply", id]);
      const counts = countsLine(p.counts);
      const commit = p.commit ? ` Commit ${p.commit.slice(0, 7)}.` : "";
      new import_obsidian2.Notice(`Applied ${p.ref?.title ?? id}${counts ? `: ${counts}` : ""}.${commit}`);
      warn(p.warnings);
    });
  }
  reject(id, reason, buttons) {
    return this.run(buttons, async () => {
      const p = await this.plugin.atlas(["change", "reject", id, "--reason", reason]);
      new import_obsidian2.Notice(`Rejected ${p.ref?.title ?? id}.`);
      warn(p.warnings);
    });
  }
};
function warn(warnings) {
  for (const w of warnings ?? []) new import_obsidian2.Notice(`Atlas: ${w}`);
}
var ReasonModal = class extends import_obsidian2.Modal {
  constructor(app, done) {
    super(app);
    this.done = done;
  }
  done;
  reason = "";
  onOpen() {
    this.setTitle("Reject the change");
    const submit = () => {
      this.close();
      this.done(this.reason);
    };
    new import_obsidian2.Setting(this.contentEl).setName("Reason").addText((text) => {
      text.setPlaceholder("One line").onChange((v) => this.reason = v);
      text.inputEl.addClass("atlas-reason-input");
      text.inputEl.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.isComposing) {
          e.preventDefault();
          submit();
        }
      });
      window.setTimeout(() => text.inputEl.focus(), 0);
    });
    new import_obsidian2.Setting(this.contentEl).addButton((b) => b.setButtonText("Cancel").onClick(() => this.close())).addButton((b) => b.setButtonText("Reject").setWarning().onClick(submit));
  }
  onClose() {
    this.contentEl.empty();
  }
};

// src/cli.ts
var import_child_process = require("child_process");
var import_fs = require("fs");
var import_os = require("os");
var AtlasError = class extends Error {
};
function findBinary(override) {
  const home = (0, import_os.homedir)();
  return chooseBinary(override, binaryCandidates(home), import_fs.existsSync, home);
}
function childEnv() {
  const extra = ["/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin"];
  const path = [process.env.PATH ?? "", ...extra].filter((p) => p !== "").join(":");
  return { ...process.env, PATH: path };
}
function exec(bin, args, cwd) {
  return new Promise((resolve, reject) => {
    (0, import_child_process.execFile)(
      bin,
      args,
      { cwd, env: childEnv(), maxBuffer: 32 * 1024 * 1024, timeout: 5 * 60 * 1e3 },
      (err, stdout, stderr) => {
        if (!err) return resolve(stdout);
        const code = err.code;
        if (code === "ENOENT") {
          return reject(new AtlasError(`the atlas binary was not found at ${bin}`));
        }
        reject(new AtlasError(errorMessage(String(stderr)) || err.message));
      }
    );
  });
}
async function runAtlas(bin, vault, args) {
  if (!bin) throw new AtlasError("the atlas binary was not found; set its path in the Atlas settings");
  const out = await exec(bin, [...args, "--vault", vault, "--json"], vault);
  try {
    return JSON.parse(out);
  } catch {
    throw new AtlasError(`atlas-obsidian ${args[0]} did not print JSON`);
  }
}
async function binaryVersion(bin) {
  return (await exec(bin, ["version"], void 0)).trim();
}

// src/graphcolors.ts
var import_obsidian3 = require("obsidian");

// src/graphgroups.ts
var GRAPH_MODES = [
  { mode: "off", label: "Off" },
  { mode: "tag", label: "Tag" },
  { mode: "focus", label: "Focus" },
  { mode: "type", label: "Type" },
  { mode: "work", label: "Threads" },
  { mode: "activity", label: "Activity" }
];
function isGraphMode(value) {
  return GRAPH_MODES.some((m) => m.mode === value);
}
var CATEGORICAL = {
  light: ["#2a78d6", "#eb6834", "#1baf7a", "#eda100", "#e87ba4", "#008300", "#4a3aa7", "#e34948"],
  dark: ["#3987e5", "#d95926", "#199e70", "#c98500", "#d55181", "#008300", "#9085e9", "#e66767"]
};
var RECENCY = {
  light: ["#104281", "#256abf", "#5598e7", "#9ec5f4"],
  dark: ["#b7d3f6", "#6da7ec", "#2a78d6", "#1c5cab"]
};
var MUTED = "#898781";
var THREAD_TYPES = ["stub", "spec", "tasks", "verification", "chord"];
var THREAD_PARTS = ["spec", "tasks", "verification"];
var TYPE_GROUPS = [
  { name: "Events", query: "[type:event]", test: (f) => f.type === "event" },
  { name: "Sources", query: "[type:source]", test: (f) => f.type === "source" },
  { name: "Repositories", query: "[type:repository]", test: (f) => f.type === "repository" },
  { name: "Concepts", query: "[type:topic] [kind:concept]", test: (f) => f.type === "topic" && f.kind === "concept" },
  { name: "Entities", query: "[type:topic] [kind:entity]", test: (f) => f.type === "topic" && f.kind === "entity" },
  { name: "Policies", query: "[type:topic] [kind:policy]", test: (f) => f.type === "topic" && f.kind === "policy" },
  { name: "Overviews", query: "[type:topic] [kind:overview]", test: (f) => f.type === "topic" && f.kind === "overview" },
  {
    name: "Threads and chords",
    query: THREAD_TYPES.map((t) => `[type:${t}]`).join(" OR "),
    test: (f) => THREAD_TYPES.includes(String(f.type))
  }
];
var QUARTERS = ["Newest 25%", "25\u201350%", "50\u201375%", "Oldest 25%"];
function graphGroups(mode, docs, resolve, theme, focus = []) {
  const vault = new Vault(docs, resolve);
  let groups;
  switch (mode) {
    case "tag":
      groups = vault.byTag(theme);
      break;
    case "focus":
      groups = byFocus(docs, focus, theme);
      break;
    case "type":
      groups = vault.byType(theme);
      break;
    case "work":
      groups = vault.byWork(theme);
      break;
    case "activity":
      groups = byActivity(docs, theme);
      break;
    default:
      groups = [];
  }
  return groups.filter((g) => g.paths.length > 0);
}
function tagsOf(fields) {
  const list = asList(fields.tags);
  if (typeof fields.defines === "string" && fields.defines) list.push(fields.defines);
  return list.map(normalTag).filter((t) => t !== "");
}
function topTags2(fields) {
  return [...new Set(tagsOf(fields).map((t) => t.split("/")[0]))];
}
function tagQuery(tag) {
  return `tag:#${tag} OR [defines:/^${tag}(\\/|$)/]`;
}
function allTagsQuery(tags) {
  return tags.length === 1 ? tagQuery(tags[0]) : tags.map((t) => `(${tagQuery(t)})`).join(" ");
}
var Vault = class {
  constructor(docs, resolve) {
    this.docs = docs;
    this.resolve = resolve;
    for (const d of docs) this.byPath.set(d.path, d);
    for (const d of docs) {
      for (const to of d.links) {
        const from = this.backlinks.get(to) ?? [];
        from.push(d.path);
        this.backlinks.set(to, from);
      }
    }
  }
  docs;
  resolve;
  byPath = /* @__PURE__ */ new Map();
  backlinks = /* @__PURE__ */ new Map();
  type(path) {
    const t = path === null ? void 0 : this.byPath.get(path)?.fields.type;
    return typeof t === "string" ? t : "";
  }
  links(doc, field) {
    return asList(doc.fields[field]).map((l) => this.resolve(linkTitle(l), doc.path)).filter((p) => p !== null && this.byPath.has(p));
  }
  /** The top tag a document belongs to: its own; an event's subject's; a session's or a change's first document's. */
  tagOf(doc, depth = 0) {
    if (depth > 3) return null;
    const own = topTags2(doc.fields)[0];
    if (own) return own;
    const via = (field) => {
      for (const p of this.links(doc, field)) {
        const t = this.tagOf(this.byPath.get(p), depth + 1);
        if (t) return t;
      }
      return null;
    };
    switch (this.type(doc.path)) {
      case "event":
        return via("subject");
      case "session":
        return via("threads") ?? via("specs") ?? via("work");
      case "change":
        return via("absorbs") ?? via("work");
    }
    return null;
  }
  /**
   * A group per top tag of the typed documents, at most eight: the tags held first take
   * the first colors, so a new tag never repaints the others, and of two tags held first
   * on one day, the one more documents hold. The graph colors a node by the first group
   * that matches, so the legend counts each document in the first group whose tag it
   * holds. Sessions and changes hold no tags: each joins the group of the work it touched,
   * by its path. A tag past the eighth, and a note with no type, get no group.
   */
  byTag(theme) {
    const typed = this.docs.filter((d) => this.type(d.path) !== "");
    const first = /* @__PURE__ */ new Map();
    const count = /* @__PURE__ */ new Map();
    for (const d of typed) {
      const created = String(d.fields.created ?? "9999").slice(0, 10);
      for (const t of topTags2(d.fields)) {
        if (!first.has(t) || created < (first.get(t) ?? "")) first.set(t, created);
        count.set(t, (count.get(t) ?? 0) + 1);
      }
    }
    const order = [...first.keys()].sort(
      (a, b) => (first.get(a) ?? "").localeCompare(first.get(b) ?? "") || (count.get(b) ?? 0) - (count.get(a) ?? 0) || a.localeCompare(b)
    );
    const palette = CATEGORICAL[theme];
    const groups = order.slice(0, palette.length).map((t, i) => ({ name: "#" + t, color: palette[i], tag: t, paths: [], records: [] }));
    for (const d of typed) {
      const own = topTags2(d.fields);
      if (own.length > 0) {
        groups.find((g2) => own.includes(g2.tag))?.paths.push(d.path);
        continue;
      }
      const type = this.type(d.path);
      if (type !== "session" && type !== "change") continue;
      const g = groups.find((x) => x.tag === this.tagOf(d));
      if (g) {
        g.paths.push(d.path);
        g.records.push(d.path);
      }
    }
    return groups.map((g) => ({
      name: g.name,
      color: g.color,
      paths: g.paths,
      query: [tagQuery(g.tag), ...g.records.sort().map((p) => `path:"${p}"`)].join(" OR ")
    }));
  }
  byType(theme) {
    const palette = CATEGORICAL[theme];
    const groups = TYPE_GROUPS.map((g, i) => ({
      name: g.name,
      color: palette[i],
      paths: this.docs.filter((d) => g.test(d.fields)).map((d) => d.path),
      query: g.query
    }));
    groups.push({
      name: "Sessions and changes",
      color: MUTED,
      paths: this.docs.filter((d) => this.type(d.path) === "session" || this.type(d.path) === "change").map((d) => d.path),
      query: "[type:session] OR [type:change]"
    });
    return groups;
  }
  byWork(theme) {
    const open = [];
    const done = [];
    const none = [];
    for (const d of this.docs) {
      const states = this.workStates(d);
      if (states.has("open")) open.push(d.path);
      else if (states.has("done")) done.push(d.path);
      else none.push(d.path);
    }
    const palette = CATEGORICAL[theme];
    return [
      { name: "Open threads", color: palette[1], paths: open },
      { name: "Ended threads", color: palette[0], paths: done },
      { name: "No thread", color: MUTED, paths: none }
    ];
  }
  /**
   * The state of a thread or a chord: open, or done when it is closed, dropped, or
   * resolved. A spec, a task list, and a verification take their thread's. Null for any
   * other document.
   */
  stateOf(path) {
    const d = this.byPath.get(path);
    if (!d) return null;
    const t = this.type(path);
    if (t === "stub" || t === "chord") {
      const s = String(d.fields.status ?? "");
      return s === "closed" || s === "dropped" || s === "resolved" ? "done" : "open";
    }
    if (THREAD_PARTS.includes(t)) {
      for (const p of this.links(d, "thread")) {
        if (this.type(p) === "stub") return this.stateOf(p);
      }
    }
    return null;
  }
  /**
   * The states of the threads a document belongs to or shares a link with. A thread
   * document takes only its own state; an event takes its subject's.
   */
  workStates(doc) {
    const states = /* @__PURE__ */ new Set();
    const own = this.stateOf(doc.path);
    if (own) {
      states.add(own);
      return states;
    }
    if (this.type(doc.path) === "event") {
      for (const p of this.links(doc, "subject")) {
        const s = this.stateOf(p);
        if (s) states.add(s);
      }
      return states;
    }
    const near = [...doc.links, ...this.backlinks.get(doc.path) ?? []];
    for (const p of near) {
      if (!THREAD_TYPES.includes(this.type(p))) continue;
      const s = this.stateOf(p);
      if (s) states.add(s);
    }
    return states;
  }
};
function byFocus(docs, chosen, theme) {
  const tags = [...new Set(chosen.map(normalTag).filter((t) => t !== ""))];
  if (tags.length === 0) return [];
  return [
    {
      name: tags.map((t) => "#" + t).join(" + "),
      color: CATEGORICAL[theme][0],
      paths: docs.filter((d) => tags.every((t) => holds(tagsOf(d.fields), t))).map((d) => d.path),
      query: allTagsQuery(tags)
    }
  ];
}
function byActivity(docs, theme) {
  const key = (d) => {
    const updated = String(d.fields.updated ?? "").slice(0, 10);
    return /^\d{4}-\d{2}-\d{2}$/.test(updated) ? updated : localDate(d.mtime);
  };
  const sorted = [...docs].sort((a, b) => key(b).localeCompare(key(a)) || b.mtime - a.mtime);
  const groups = QUARTERS.map((name, i) => ({ name, color: RECENCY[theme][i], paths: [] }));
  sorted.forEach((d, i) => groups[Math.floor(i * 4 / sorted.length)].paths.push(d.path));
  return groups;
}
function localDate(ms) {
  const t = new Date(ms);
  const pad = (n) => String(n).padStart(2, "0");
  return `${t.getFullYear()}-${pad(t.getMonth() + 1)}-${pad(t.getDate())}`;
}
function pathQuery(paths) {
  const alternatives = paths.map((p) => p.replace(/[.*+?^${}()|[\]\\]/g, "\\$&").replace(/\//g, "\\/"));
  return `path:/^(?:${alternatives.join("|")})$/`;
}
function colorGroups(groups) {
  return groups.map((g) => ({
    query: g.query ?? pathQuery(g.paths),
    color: { a: 1, rgb: parseInt(g.color.slice(1), 16) }
  }));
}
function isAtlasQuery(query) {
  return query.startsWith("path:/^(?:") && query.endsWith(")$/");
}
function mergeColorGroups(current, ours, owned = []) {
  const mine = /* @__PURE__ */ new Set([...owned, ...ours.map((g) => g.query)]);
  return [...ours, ...current.filter((g) => !isAtlasQuery(g.query) && !mine.has(g.query))];
}

// src/graphcolors.ts
var BAR2 = "atlas-graph-colors";
function graphInstance(app) {
  const internal = app.internalPlugins;
  const plugin = internal?.getPluginById?.("graph");
  const instance = plugin?.enabled ? plugin.instance : void 0;
  return instance && typeof instance.saveOptions === "function" && instance.options ? instance : null;
}
function engineOf(view) {
  const v = view;
  const engine = v.dataEngine ?? v.engine;
  return engine && typeof engine.setOptions === "function" && typeof engine.getOptions === "function" ? engine : null;
}
function same(a, b) {
  return JSON.stringify(a) === JSON.stringify(b);
}
var GraphColors = class extends import_obsidian3.Component {
  constructor(host) {
    super();
    this.host = host;
  }
  host;
  groups = [];
  refresh = (0, import_obsidian3.debounce)(() => this.apply(), 1e3, true);
  get app() {
    return this.host.app;
  }
  onload() {
    this.registerEvent(this.app.metadataCache.on("resolved", () => this.refresh()));
    this.registerEvent(this.app.vault.on("rename", () => this.refresh()));
    this.registerEvent(this.app.vault.on("delete", () => this.refresh()));
    this.registerEvent(this.app.workspace.on("css-change", () => this.refresh()));
    this.registerEvent(this.app.workspace.on("layout-change", () => this.refresh()));
    this.app.workspace.onLayoutReady(() => this.apply());
  }
  onunload() {
    this.write([]);
    for (const leaf of this.app.workspace.getLeavesOfType("graph")) {
      leaf.view.containerEl.querySelector(`.${BAR2}`)?.remove();
    }
  }
  /** The tags Focus mode crosses: the Atlas navigator's choice. */
  async setFocus(tags) {
    if (JSON.stringify(tags) === JSON.stringify(this.host.settings.focusTags)) return;
    this.host.settings.focusTags = [...tags];
    await this.host.saveSettings();
    if (this.host.settings.graphColors === "focus") this.apply();
  }
  async setMode(mode) {
    this.host.settings.graphColors = mode;
    await this.host.saveSettings();
    this.apply();
  }
  apply() {
    const mode = this.host.settings.graphColors;
    const theme = document.body.hasClass("theme-dark") ? "dark" : "light";
    const cache = this.app.metadataCache;
    this.groups = mode === "off" ? [] : graphGroups(mode, this.docs(), (link, from) => cache.getFirstLinkpathDest(link, from)?.path ?? null, theme, this.host.settings.focusTags);
    this.write(colorGroups(this.groups));
    this.renderBars();
  }
  docs() {
    const cache = this.app.metadataCache;
    return this.app.vault.getMarkdownFiles().map((file) => ({
      path: file.path,
      fields: cache.getFileCache(file)?.frontmatter ?? {},
      mtime: file.stat.mtime,
      links: Object.keys(cache.resolvedLinks[file.path] ?? {})
    }));
  }
  /** Puts our groups in the graph's saved options and in every open graph. */
  write(ours) {
    const instance = graphInstance(this.app);
    if (!instance) return;
    const owned = this.host.settings.graphOwned;
    const merged = mergeColorGroups(instance.options.colorGroups ?? [], ours, owned);
    if (!same(instance.options.colorGroups ?? [], merged)) {
      instance.options.colorGroups = merged;
      instance.saveOptions();
    }
    for (const type of ["graph", "localgraph"]) {
      for (const leaf of this.app.workspace.getLeavesOfType(type)) {
        const engine = engineOf(leaf.view);
        if (!engine) continue;
        const current = engine.getOptions().colorGroups ?? [];
        const next = mergeColorGroups(current, ours, owned);
        if (!same(current, next)) engine.setOptions({ colorGroups: next });
      }
    }
    const queries = ours.map((g) => g.query);
    if (JSON.stringify(queries) !== JSON.stringify(owned)) {
      this.host.settings.graphOwned = queries;
      void this.host.saveSettings();
    }
  }
  /** The mode buttons and the legend, over each graph view. */
  renderBars() {
    const mode = this.host.settings.graphColors;
    const on = graphInstance(this.app) !== null;
    for (const leaf of this.app.workspace.getLeavesOfType("graph")) {
      const content = leaf.view.containerEl.querySelector(".view-content");
      if (!content) continue;
      let bar = content.querySelector(`:scope > .${BAR2}`);
      if (!on) {
        bar?.remove();
        continue;
      }
      if (!bar) bar = content.createDiv({ cls: BAR2 });
      bar.empty();
      const modes = bar.createDiv({ cls: "atlas-graph-modes" });
      for (const m of GRAPH_MODES) {
        const button = modes.createEl("button", { text: m.label, cls: "atlas-graph-mode" });
        button.toggleClass("is-active", m.mode === mode);
        button.setAttr("aria-pressed", String(m.mode === mode));
        button.onClickEvent(() => void this.setMode(m.mode));
      }
      if (mode === "focus" && this.host.settings.focusTags.length === 0) {
        bar.createDiv({ cls: "atlas-graph-legend-row", text: "Open the Atlas navigator (left ribbon) and choose tags to focus on them." });
        continue;
      }
      if (mode === "focus" && this.groups.length === 0) {
        bar.createDiv({ cls: "atlas-graph-legend-row", text: "No document holds all the chosen tags." });
        continue;
      }
      if (this.groups.length === 0) continue;
      const legend = bar.createDiv({ cls: "atlas-graph-legend" });
      for (const g of this.groups) {
        const row = legend.createDiv({ cls: "atlas-graph-legend-row" });
        row.createSpan({ cls: "atlas-graph-swatch" }).style.backgroundColor = g.color;
        row.createSpan({ cls: "atlas-graph-legend-name", text: g.name });
        row.createSpan({ cls: "atlas-graph-legend-count", text: String(g.paths.length) });
      }
    }
  }
};

// src/canvasbar.ts
var import_obsidian4 = require("obsidian");
var BAR3 = "atlas-canvas-bar";
var CanvasBar = class extends import_obsidian4.Component {
  constructor(plugin) {
    super();
    this.plugin = plugin;
  }
  plugin;
  busy = false;
  states = /* @__PURE__ */ new Map();
  refresh = (0, import_obsidian4.debounce)(() => void this.update(), 400, true);
  get app() {
    return this.plugin.app;
  }
  onload() {
    const ws = this.app.workspace;
    this.registerEvent(ws.on("active-leaf-change", () => this.refresh()));
    this.registerEvent(ws.on("layout-change", () => this.refresh()));
    this.registerEvent(
      this.app.vault.on("modify", (file) => {
        if (chordOfCanvas(file.path) !== null) this.refresh();
      })
    );
    this.registerEvent(this.app.metadataCache.on("changed", () => this.refresh()));
    ws.onLayoutReady(() => this.refresh());
  }
  onunload() {
    document.querySelectorAll(`.${BAR3}`).forEach((el) => el.remove());
  }
  views() {
    return this.app.workspace.getLeavesOfType("canvas").map((leaf) => leaf.view).filter((v) => v instanceof import_obsidian4.FileView && v.file !== null && chordOfCanvas(v.file.path) !== null);
  }
  async update() {
    const views = this.views();
    const open = new Set(views.map((v) => v.file?.path ?? ""));
    document.querySelectorAll(`.${BAR3}`).forEach((el) => {
      if (!open.has(el.dataset.path ?? "")) el.remove();
    });
    for (const view of views) {
      const path = view.file?.path ?? "";
      const chord = chordOfCanvas(path);
      if (chord === null) continue;
      let state = null;
      try {
        state = (await this.plugin.atlas(["chord", "canvas", chord])).canvas;
      } catch {
      }
      this.states.set(path, state);
      this.render(view, chord, state);
    }
  }
  render(view, chord, state) {
    const path = view.file?.path ?? "";
    const existing = view.containerEl.querySelector(`:scope > .${BAR3}`);
    if (!state) {
      existing?.remove();
      return;
    }
    const key = `${path}
${state.differs}
${(state.changes ?? []).join("|")}`;
    if (existing?.dataset.key === key) return;
    existing?.remove();
    const bar = createDiv({ cls: BAR3 });
    bar.dataset.key = key;
    bar.dataset.path = path;
    bar.toggleClass("is-changed", state.differs);
    bar.createSpan({ cls: "atlas-canvas-bar-label", text: "Chord" });
    const text = bar.createSpan({ cls: "atlas-canvas-bar-status", text: canvasSummary(state) });
    if (state.differs) text.setAttr("title", (state.threads ?? []).join("\n"));
    const buttons = bar.createDiv({ cls: "atlas-canvas-bar-buttons" });
    const add = (label, cls, args, done) => {
      const b = buttons.createEl("button", { text: label, cls });
      b.onclick = () => void this.run(["chord", "canvas", chord, ...args], done);
    };
    if (state.differs) {
      add("Save order", "mod-cta", ["--save"], "Saved the order to the stubs");
      add("Revert", "", ["--write"], "Took the stubs' order back");
    }
    add("Tidy", "", ["--tidy"], "Placed the cards again");
    buttons.createEl("button", { text: "New thread" }).onclick = () => {
      const note = this.app.metadataCache.getFirstLinkpathDest(chord, "");
      const id = note ? this.app.metadataCache.getFileCache(note)?.frontmatter?.id : void 0;
      if (id) this.plugin.newThread({ id: String(id), title: chord });
    };
    buttons.createEl("button", { text: "Open the chord" }).onclick = () => void this.app.workspace.openLinkText(chord, "", "tab");
    view.containerEl.insertBefore(bar, view.contentEl);
  }
  async run(args, done) {
    if (this.busy) return;
    this.busy = true;
    try {
      await new Promise((resolve) => window.setTimeout(resolve, 600));
      await this.plugin.atlas(args);
      new import_obsidian4.Notice(`Atlas: ${done}.`);
    } catch (e) {
      new import_obsidian4.Notice(`Atlas: ${e.message}`, 1e4);
    } finally {
      this.busy = false;
      this.refresh();
    }
  }
};

// src/mentions.ts
var import_state = require("@codemirror/state");
var import_view = require("@codemirror/view");
var mark = import_view.Decoration.mark({ class: "atlas-mention" });
function build(view) {
  const builder = new import_state.RangeSetBuilder();
  for (const { from, to } of view.visibleRanges) {
    let pos = from;
    while (pos <= to) {
      const line = view.state.doc.lineAt(pos);
      for (const [a, b] of mentionRanges(line.text)) builder.add(line.from + a, line.from + b, mark);
      pos = line.to + 1;
    }
  }
  return builder.finish();
}
var mentionEditor = import_view.ViewPlugin.fromClass(
  class {
    decorations;
    constructor(view) {
      this.decorations = build(view);
    }
    update(u) {
      if (u.docChanged || u.viewportChanged) this.decorations = build(u.view);
    }
  },
  { decorations: (v) => v.decorations }
);
var mentionReading = (el) => {
  el.querySelectorAll("li.task-list-item").forEach((item) => {
    const walker = document.createTreeWalker(item, NodeFilter.SHOW_TEXT);
    const nodes = [];
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const text = n;
      if (text.parentElement?.closest("li.task-list-item") !== item) continue;
      if (text.parentElement?.closest(".atlas-mention, code, a")) continue;
      if (textMentions(text.data).length > 0) nodes.push(text);
    }
    nodes.forEach(wrap);
  });
};
function wrap(node) {
  const ranges = textMentions(node.data);
  const frag = document.createDocumentFragment();
  let at = 0;
  for (const [a, b] of ranges) {
    if (a > at) frag.append(node.data.slice(at, a));
    frag.append(createSpan({ cls: "atlas-mention", text: node.data.slice(a, b) }));
    at = b;
  }
  if (at < node.data.length) frag.append(node.data.slice(at));
  node.replaceWith(frag);
}

// src/repo.ts
var import_obsidian5 = require("obsidian");
var RepoPanel = class extends import_obsidian5.MarkdownRenderChild {
  constructor(containerEl, plugin, id, branch) {
    super(containerEl);
    this.plugin = plugin;
    this.id = id;
    this.branch = branch;
  }
  plugin;
  id;
  branch;
  onload() {
    void this.refresh();
  }
  async refresh() {
    const el = this.containerEl;
    el.empty();
    el.addClass("atlas-repo");
    el.createDiv({ cls: "atlas-repo-loading", text: "Reading git\u2026" });
    try {
      const b = await this.plugin.atlas(["context", this.id]);
      this.render(b.repository);
    } catch (e) {
      el.empty();
      el.createDiv({ cls: "atlas-repo-error", text: `Atlas: ${e.message}` });
      this.refreshButton(el);
    }
  }
  render(f) {
    const el = this.containerEl;
    el.empty();
    if (!f || !f.exists) {
      el.createDiv({ cls: "atlas-repo-error", text: `${f?.path ?? "The path"} is gone, or is no git work tree.` });
      this.refreshButton(el);
      return;
    }
    const head = el.createDiv({ cls: "atlas-repo-head" });
    head.createSpan({ cls: "atlas-repo-branch", text: f.branch ?? "?" });
    if (this.branch && f.branch && f.branch !== this.branch) {
      head.createSpan({ cls: "atlas-repo-note", text: `not ${this.branch}` });
    }
    head.createSpan({ cls: "atlas-repo-sha", text: f.head ?? "" });
    if (f.upstream) {
      head.createSpan({ cls: "atlas-repo-sync", text: `${f.ahead ?? 0} ahead \xB7 ${f.behind_remote ?? 0} behind the remote` });
    }
    this.refreshButton(head);
    const dirty = f.dirty ?? [];
    if (dirty.length === 0) {
      el.createDiv({ cls: "atlas-repo-clean", text: "No uncommitted changes." });
    } else {
      const d = el.createEl("details", { cls: "atlas-repo-dirty" });
      d.createEl("summary", { text: `${dirty.length} ${dirty.length === 1 ? "file" : "files"} changed and not committed` });
      const ul = d.createEl("ul");
      for (const p of dirty) ul.createEl("li", { text: p });
    }
    const now = /* @__PURE__ */ new Date();
    for (const c of f.recent ?? []) {
      const row = el.createDiv({ cls: "atlas-repo-commit" });
      row.createSpan({ cls: "atlas-repo-sha", text: c.commit });
      row.createSpan({ cls: "atlas-repo-subject", text: c.subject });
      row.createSpan({ cls: "atlas-repo-ago", text: formatAgo(c.time, now) });
    }
    if (f.described && (f.behind ?? 0) > 0) {
      el.createDiv({ cls: "atlas-repo-note", text: `The description is ${f.behind} commits behind (repo-ingest).` });
    }
  }
  refreshButton(parent) {
    const b = parent.createEl("button", { cls: "atlas-repo-refresh", text: "Refresh" });
    b.onclick = () => void this.refresh();
  }
};
function repoProcessor(plugin) {
  return (source, el, ctx) => {
    const { id } = repoBlock(source);
    if (!id) return;
    ctx.addChild(new RepoPanel(el, plugin, id, String(repoFields(plugin, id, ctx.sourcePath)?.branch ?? "")));
  };
}
function repoFields(plugin, id, sourcePath) {
  const cache = plugin.app.metadataCache;
  const own = plugin.app.vault.getFileByPath(sourcePath);
  const fm = own ? cache.getFileCache(own)?.frontmatter : void 0;
  if (fm?.id === id) return fm;
  for (const f of plugin.app.vault.getMarkdownFiles()) {
    if (!f.path.startsWith("wiki/documents/")) continue;
    const other = cache.getFileCache(f)?.frontmatter;
    if (other?.id === id) return other;
  }
  return void 0;
}

// src/sessions.ts
var import_obsidian6 = require("obsidian");
var import_os3 = require("os");

// src/agents.ts
var LIVE = ["running", "waiting", "idle"];
var RECENT_MS = 2 * 60 * 60 * 1e3;
function isOpen(row, alive, now, staleHours) {
  if (!LIVE.includes(row.status)) return false;
  if (row.pid > 0) return alive(row.pid);
  const t = Date.parse(row.updated);
  return !Number.isNaN(t) && now.getTime() - t < staleHours * 3600 * 1e3;
}
function sessionState(row, open) {
  if (!open) return row.status === "lost" ? "lost" : "ended";
  if (row.status === "waiting") return "needs you";
  if (row.status === "running") return "working";
  return "idle";
}
var STATE_RANK = { "needs you": 0, working: 1, idle: 2, ended: 3, lost: 3 };
function groupSessions(rows, alive, now, staleHours) {
  const out = { open: [], recent: [], older: 0 };
  for (const row of rows) {
    if (row.parent) continue;
    const open = isOpen(row, alive, now, staleHours);
    const state = sessionState(row, open);
    if (open) {
      out.open.push({ row, state });
      continue;
    }
    const t = Date.parse(row.ended || row.updated);
    if (!Number.isNaN(t) && now.getTime() - t < RECENT_MS) out.recent.push({ row, state });
    else out.older++;
  }
  const newest = (a, b) => String(b.updated).localeCompare(String(a.updated));
  out.open.sort((a, b) => STATE_RANK[a.state] - STATE_RANK[b.state] || newest(a.row, b.row));
  out.recent.sort((a, b) => String(b.row.ended || b.row.updated).localeCompare(String(a.row.ended || a.row.updated)));
  return out;
}
function shellQuote(s) {
  return `'${s.replace(/'/g, `'\\''`)}'`;
}
function appleScriptString(s) {
  return `"${s.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}
function configDirOf(transcript) {
  const i = transcript.lastIndexOf("/projects/");
  return i > 0 ? transcript.slice(0, i) : "";
}
function firstCwd(jsonl) {
  for (const line of jsonl.split("\n")) {
    if (!line.includes('"cwd"')) continue;
    try {
      const cwd = JSON.parse(line).cwd;
      if (typeof cwd === "string" && cwd) return cwd;
    } catch {
    }
  }
  return "";
}
function resumeCommand(t) {
  const parts = [];
  if (t.cwd) parts.push(`cd ${shellQuote(t.cwd)}`);
  if (t.harness === "codex") {
    parts.push(`codex resume ${shellQuote(t.id)}`);
  } else {
    if (t.configDir) parts.push(`export CLAUDE_CONFIG_DIR=${shellQuote(t.configDir)}`);
    parts.push(`claude --resume ${shellQuote(t.id)}`);
  }
  return parts.join(" && ");
}
function startCommand(dir, agent, prompt) {
  const program = agent.trim() || "claude";
  return `cd ${shellQuote(dir)} && ${program} ${shellQuote(prompt)}`;
}
var TERMINALS = ["terminal", "iterm", "wezterm", "ghostty", "custom"];
var TERMINAL_NAMES = {
  terminal: "Terminal",
  iterm: "iTerm2",
  wezterm: "WezTerm",
  ghostty: "Ghostty",
  custom: "Custom command"
};
var AGENTS = ["claude", "codex"];
var AGENT_NAMES = { claude: "Claude Code", codex: "Codex" };
function preference(p, key) {
  if (!p) return "";
  if (key.startsWith("agent_commands.")) return p.agent_commands?.[key.slice("agent_commands.".length)] ?? "";
  return String(p[key] ?? "");
}
function inherited(config, key) {
  const g = preference(config.global, key);
  if (g) return g;
  if (key === "agent") return "claude";
  if (key === "terminal") return "terminal";
  if (key.startsWith("agent_commands.")) return key.slice("agent_commands.".length);
  return "";
}
function legacyPreferences(saved, vault) {
  if (!saved) return [];
  const out = [];
  const take = (key, value, old) => {
    const v = typeof value === "string" ? value.trim() : "";
    if (v && v !== old && !preference(vault, key)) out.push([key, v]);
  };
  take("agent_commands.claude", saved.agentCommand, "claude");
  if (TERMINALS.includes(String(saved.terminal))) take("terminal", saved.terminal, "terminal");
  if (String(saved.terminalCommand ?? "").includes("{command}")) take("terminal_command", saved.terminalCommand, "");
  return out;
}
var TERMINAL_APPS = { iterm: "iTerm.app", wezterm: "WezTerm.app", ghostty: "Ghostty.app" };
function terminalLaunch(app, command, shell, custom) {
  const sh = shell || "/bin/zsh";
  const interactive = [sh, "-lic", `${command}; exec ${sh} -l`];
  switch (app) {
    case "iterm":
      return {
        program: "osascript",
        args: [
          "-e",
          'tell application "iTerm" to create window with default profile',
          "-e",
          `tell application "iTerm" to tell current session of current window to write text ${appleScriptString(command)}`,
          "-e",
          'tell application "iTerm" to activate'
        ]
      };
    case "wezterm":
      return { program: "/Applications/WezTerm.app/Contents/MacOS/wezterm", args: ["start", "--", ...interactive] };
    case "ghostty":
      return { program: "open", args: ["-na", "Ghostty", "--args", "-e", ...interactive] };
    case "custom":
      return { program: "/bin/sh", args: ["-c", custom.split("{command}").join(shellQuote(command))] };
    default:
      return {
        program: "osascript",
        args: [
          "-e",
          `tell application "Terminal" to do script ${appleScriptString(command)}`,
          "-e",
          'tell application "Terminal" to activate'
        ]
      };
  }
}
function plainLinks(text) {
  return text.replace(/\[\[([^\]|]*)(?:\|([^\]]*))?\]\]/g, (_m, target, alias) => (alias ?? target).split("#")[0]);
}
function threadStage(t) {
  if (t.blocked) return "blocked";
  switch (t.status) {
    case "stub":
      return "needs spec";
    case "specified":
      return "writing tasks";
    case "planned":
      return "ready";
    case "started":
      return t.tasks ? `tasks ${t.tasks}` : "started";
    case "unverified":
      return "verifying";
    case "verified":
      return "to close";
    default:
      return t.status;
  }
}

// src/launcher.ts
var import_child_process2 = require("child_process");
var import_fs2 = require("fs");
var import_os2 = require("os");
var import_path = require("path");
function openTerminal(app, command, custom) {
  const bundle = TERMINAL_APPS[app];
  const found = bundle ? ["/Applications", (0, import_path.join)((0, import_os2.homedir)(), "Applications")].map((d) => (0, import_path.join)(d, bundle)).find((p) => (0, import_fs2.existsSync)(p)) : void 0;
  if (bundle && !found) return Promise.reject(new Error(`${TERMINAL_NAMES[app]} is not in /Applications`));
  if (app === "custom" && !custom.includes("{command}")) {
    return Promise.reject(new Error("the custom terminal command has no {command}"));
  }
  const launch = terminalLaunch(app, command, process.env.SHELL ?? "/bin/zsh", custom);
  if (app === "wezterm" && found) launch.program = (0, import_path.join)(found, "Contents/MacOS/wezterm");
  return new Promise((resolve, reject) => {
    const child = (0, import_child_process2.spawn)(launch.program, launch.args, { detached: true, stdio: "ignore" });
    child.once("error", reject);
    child.once("spawn", () => {
      child.unref();
      window.setTimeout(resolve, 300);
    });
  });
}
function liveAgents(pids) {
  const list = [...new Set(pids.filter((p) => p > 1))];
  if (list.length === 0) return Promise.resolve(/* @__PURE__ */ new Set());
  return new Promise((resolve) => {
    (0, import_child_process2.execFile)("ps", ["-o", "pid=,comm=", "-p", list.join(",")], (_err, stdout) => {
      const out = /* @__PURE__ */ new Set();
      for (const line of String(stdout ?? "").split("\n")) {
        const m = /^\s*(\d+)\s+(.*)$/.exec(line);
        if (!m) continue;
        const name = m[2].trim().split("/").pop() ?? "";
        if (name === "claude" || name === "codex" || name.startsWith("claude-") || name.startsWith("codex-")) out.add(Number(m[1]));
      }
      resolve(out);
    });
  });
}
function configDirs() {
  const home = (0, import_os2.homedir)();
  const out = /* @__PURE__ */ new Set();
  if (process.env.CLAUDE_CONFIG_DIR) out.add(process.env.CLAUDE_CONFIG_DIR);
  try {
    for (const name of (0, import_fs2.readdirSync)(home)) {
      if (name === ".claude" || name.startsWith(".claude-")) out.add((0, import_path.join)(home, name));
    }
  } catch {
  }
  return [...out];
}
function findTranscript(recorded, id) {
  if (recorded && (0, import_fs2.existsSync)(recorded)) return recorded;
  if (!id) return "";
  for (const dir of configDirs()) {
    const projects = (0, import_path.join)(dir, "projects");
    let folders = [];
    try {
      folders = (0, import_fs2.readdirSync)(projects);
    } catch {
      continue;
    }
    for (const f of folders) {
      const file = (0, import_path.join)(projects, f, `${id}.jsonl`);
      if ((0, import_fs2.existsSync)(file)) return file;
    }
  }
  return "";
}
function resumePlace(transcript) {
  let head = "";
  try {
    const fd = (0, import_fs2.openSync)(transcript, "r");
    const buf = Buffer.alloc(256 * 1024);
    const n = (0, import_fs2.readSync)(fd, buf, 0, buf.length, 0);
    (0, import_fs2.closeSync)(fd);
    head = buf.subarray(0, n).toString("utf8");
  } catch {
  }
  const dir = configDirOf(transcript);
  return { cwd: firstCwd(head), configDir: dir === (0, import_path.join)((0, import_os2.homedir)(), ".claude") ? "" : dir };
}

// src/sessions.ts
var SESSIONS_VIEW = "atlas-sessions";
function readSessions(app) {
  const out = [];
  for (const file of app.vault.getMarkdownFiles()) {
    if (!file.path.startsWith("sessions/")) continue;
    const fm = app.metadataCache.getFileCache(file)?.frontmatter;
    if (!fm || fm.type !== "session") continue;
    const work = asList(fm.work).map(linkTitle);
    out.push({
      file,
      path: file.path,
      status: String(fm.status ?? ""),
      updated: String(fm.updated ?? ""),
      ended: String(fm.ended ?? ""),
      pid: Number(fm.pid ?? 0) || 0,
      parent: String(fm.parent ?? ""),
      description: typeof fm.description === "string" && fm.description.trim() ? fm.description : file.basename,
      work,
      threads: [...asList(fm.specs), ...asList(fm.threads)],
      harness: String(fm.harness ?? "claude"),
      harness_id: String(fm.harness_id ?? ""),
      cwd: String(fm.cwd ?? ""),
      transcript: String(fm.transcript ?? "")
    });
  }
  return out;
}
async function sessionGroups(app, staleHours) {
  const rows = readSessions(app);
  const live = await liveAgents(rows.map((r) => r.pid));
  return { rows, groups: groupSessions(rows, (pid) => live.has(pid), /* @__PURE__ */ new Date(), staleHours) };
}
function sessionThread(app, s) {
  const stubOf = (title) => {
    const file = title ? app.metadataCache.getFirstLinkpathDest(title, s.file.path) : null;
    const fm = file ? app.metadataCache.getFileCache(file)?.frontmatter : void 0;
    if (!file || !fm) return null;
    if (fm.type === "spec" || fm.type === "tasks" || fm.type === "verification") return stubOf(linkTitle(fm.thread));
    if (fm.type !== "stub") return null;
    return { file, id: String(fm.id ?? ""), status: String(fm.status ?? ""), tasks: String(fm.tasks ?? ""), blocked: String(fm.blocked ?? "") };
  };
  let fallback = null;
  for (let i = s.threads.length - 1; i >= 0; i--) {
    const t = stubOf(linkTitle(s.threads[i]));
    if (!t) continue;
    fallback ??= t;
    if (t.status === "started") return t;
  }
  if (fallback) return fallback;
  for (let i = s.work.length - 1; i >= 0; i--) {
    const t = stubOf(s.work[i]);
    if (t) return t;
  }
  return null;
}
async function resume(plugin, s) {
  const id = s.harness_id.trim();
  if (!id || s.parent) {
    new import_obsidian6.Notice("Atlas: this session has no conversation of its own to resume.");
    return;
  }
  let target = { harness: s.harness, id, cwd: expandHome(s.cwd, (0, import_os3.homedir)()), configDir: "" };
  if (s.harness !== "codex") {
    const transcript = findTranscript(s.transcript, id);
    if (!transcript) {
      new import_obsidian6.Notice("Atlas: Claude Code has no saved conversation for this session, so it cannot resume.", 8e3);
      return;
    }
    const place = resumePlace(transcript);
    target = { ...target, cwd: place.cwd || target.cwd, configDir: place.configDir };
  }
  await plugin.runInTerminal(resumeCommand(target), "the resume command");
}
var STATE_CLASS = { "needs you": "waiting", working: "working", idle: "idle", ended: "ended", lost: "ended" };
var SessionsView = class extends import_obsidian6.ItemView {
  constructor(leaf, plugin) {
    super(leaf);
    this.plugin = plugin;
  }
  plugin;
  generation = 0;
  getViewType() {
    return SESSIONS_VIEW;
  }
  getDisplayText() {
    return "Atlas sessions";
  }
  getIcon() {
    return "bot";
  }
  async onOpen() {
    void this.render();
  }
  /** Checks the processes again and redraws; a closed terminal ends its card. */
  tick() {
    void this.render();
  }
  async render() {
    const generation = ++this.generation;
    const { rows, groups } = await sessionGroups(this.app, this.plugin.staleHours());
    if (generation !== this.generation) return;
    const root = this.contentEl;
    root.empty();
    root.addClass("atlas-sessions");
    const now = /* @__PURE__ */ new Date();
    const subagents = /* @__PURE__ */ new Map();
    for (const r of rows) {
      if (r.parent && r.status === "running") {
        const key = linkTitle(r.parent);
        subagents.set(key, (subagents.get(key) ?? 0) + 1);
      }
    }
    if (groups.open.length === 0) {
      root.createDiv({ cls: "atlas-sessions-empty", text: "No agent session is open." });
    }
    for (const { row, state } of groups.open) this.card(root, row, state, now, generation, subagents.get(row.file.basename) ?? 0);
    if (groups.recent.length > 0) {
      root.createDiv({ cls: "atlas-sessions-heading", text: "Closed in the last 2 hours" });
      for (const { row, state } of groups.recent) this.card(root, row, state, now, generation, 0);
    }
    if (groups.older > 0) {
      const more = root.createDiv({ cls: "atlas-sessions-more" });
      more.setText(`${groups.older} older ${groups.older === 1 ? "session" : "sessions"} in sessions/`);
      more.onclick = () => void this.app.workspace.openLinkText("sessions/Sessions.base", "", false);
    }
  }
  card(root, s, state, now, generation, subagents) {
    const card = root.createDiv({ cls: "atlas-session" });
    card.dataset.state = STATE_CLASS[state];
    card.onclick = () => void this.app.workspace.getLeaf(false).openFile(s.file);
    const thread = sessionThread(this.app, s);
    const top = card.createDiv({ cls: "atlas-session-top" });
    top.createSpan({ cls: "atlas-session-status", text: state });
    if (thread?.id) top.createSpan({ cls: "atlas-session-id", text: thread.id });
    if (subagents > 0) top.createSpan({ cls: "atlas-session-sub", text: `+${subagents} ${subagents === 1 ? "subagent" : "subagents"}` });
    const closed = state === "ended" || state === "lost";
    top.createSpan({ cls: "atlas-session-ago", text: formatAgo(closed ? s.ended || s.updated : s.updated, now) });
    if (closed) {
      const button = top.createEl("button", { cls: "atlas-session-resume", text: "Resume" });
      button.onclick = (e) => {
        e.stopPropagation();
        void resume(this.plugin, s);
      };
    }
    const title = plainLinks(s.description);
    card.createDiv({ cls: "atlas-session-title", text: title }).setAttr("title", title);
    if (thread) {
      const line = card.createDiv({ cls: "atlas-session-thread" });
      line.createSpan({ cls: "atlas-session-thread-name", text: thread.file.basename });
      line.createSpan({ cls: "atlas-session-stage", text: threadStage(thread) }).dataset.stage = thread.blocked ? "blocked" : thread.status;
    }
    void this.app.vault.cachedRead(s.file).then((text) => {
      const last = lastProgressLine(text);
      if (generation === this.generation && last) card.setAttr("title", plainLinks(last));
    });
  }
};

// src/newthread.ts
var import_obsidian7 = require("obsidian");
var NewThreadModal = class extends import_obsidian7.Modal {
  constructor(app, chord, done) {
    super(app);
    this.chord = chord;
    this.done = done;
  }
  chord;
  done;
  title = "";
  idea = "";
  onOpen() {
    this.setTitle(`New thread in ${this.chord}`);
    const submit = () => {
      const title = this.title.trim();
      if (!title) return;
      this.close();
      this.done(title, this.idea.trim());
    };
    new import_obsidian7.Setting(this.contentEl).setName("Title").addText((text) => {
      text.setPlaceholder("Annotate the dataset").onChange((v) => this.title = v);
      text.inputEl.addClass("atlas-reason-input");
      text.inputEl.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.isComposing) {
          e.preventDefault();
          submit();
        }
      });
      window.setTimeout(() => text.inputEl.focus(), 0);
    });
    new import_obsidian7.Setting(this.contentEl).setName("Idea").setDesc("What this thread delivers, in your words. An agent writes the spec from it later.").addTextArea((area) => {
      area.setPlaceholder("Label every image of the train and val splits.").onChange((v) => this.idea = v);
      area.inputEl.rows = 4;
      area.inputEl.addClass("atlas-idea-input");
      area.inputEl.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
          e.preventDefault();
          submit();
        }
      });
    });
    new import_obsidian7.Setting(this.contentEl).addButton((b) => b.setButtonText("Cancel").onClick(() => this.close())).addButton((b) => b.setButtonText("Plant the thread").setCta().onClick(submit));
  }
  onClose() {
    this.contentEl.empty();
  }
};

// src/settings.ts
var import_obsidian8 = require("obsidian");
var import_os4 = require("os");
var shortHome = (p) => p.startsWith((0, import_os4.homedir)() + "/") ? "~" + p.slice((0, import_os4.homedir)().length) : p;
var DEFAULT_SETTINGS = {
  binaryPath: "",
  syncOnChange: true,
  badges: true,
  viewFolders: true,
  tagClick: false,
  graphColors: "tag",
  graphOwned: [],
  focusTags: []
};
var AtlasSettingTab = class extends import_obsidian8.PluginSettingTab {
  constructor(app, plugin) {
    super(app, plugin);
    this.plugin = plugin;
  }
  plugin;
  display() {
    const { containerEl } = this;
    containerEl.empty();
    const found = findBinary("");
    const binary = new import_obsidian8.Setting(containerEl).setName("Path to the atlas-obsidian binary").setDesc("Leave empty to use the binary Atlas finds.").addText(
      (text) => text.setPlaceholder(found ?? "Not found").setValue(this.plugin.settings.binaryPath).onChange(async (value) => {
        this.plugin.settings.binaryPath = value.trim();
        await this.plugin.saveSettings();
        void showVersion();
      })
    );
    const status = binary.descEl.createDiv({ cls: "atlas-setting-status" });
    const showVersion = async () => {
      const bin = findBinary(this.plugin.settings.binaryPath);
      if (!bin) {
        status.setText("No binary found.");
        return;
      }
      try {
        status.setText(`Uses ${bin} (${await binaryVersion(bin)}).`);
      } catch (e) {
        status.setText(`Cannot run ${bin}: ${e.message}`);
      }
    };
    void showVersion();
    new import_obsidian8.Setting(containerEl).setName("Keep the views fresh").setDesc("Runs atlas-obsidian vault sync --views two seconds after a note changes, so the views, the statuses, and the callouts follow your edits.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.syncOnChange).onChange(async (value) => {
        this.plugin.settings.syncOnChange = value;
        await this.plugin.saveSettings();
      })
    );
    new import_obsidian8.Setting(containerEl).setName("Badges in the file explorer").setDesc("Shows the status of each stub, plan, and session, and the kind of each event.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.badges).onChange(async (value) => {
        this.plugin.settings.badges = value;
        await this.plugin.saveSettings();
        this.plugin.badges.setEnabled(value);
      })
    );
    new import_obsidian8.Setting(containerEl).setName("Open a tag's view from its folder").setDesc("In the file explorer, a click on a folder under views/tags opens the tag's view, and the view itself is hidden inside the folder.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.viewFolders).onChange(async (value) => {
        this.plugin.settings.viewFolders = value;
        await this.plugin.saveSettings();
        this.plugin.viewFolders.setEnabled(value);
      })
    );
    new import_obsidian8.Setting(containerEl).setName("Open a tag in the Atlas navigator").setDesc("A click on a #tag in a note opens the Atlas navigator at that tag, in place of Obsidian's search.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.tagClick).onChange(async (value) => {
        this.plugin.settings.tagClick = value;
        await this.plugin.saveSettings();
      })
    );
    new import_obsidian8.Setting(containerEl).setName("Agents").setHeading();
    const agents = containerEl.createDiv();
    void this.agents(agents);
    new import_obsidian8.Setting(containerEl).setName("Graph").setHeading();
    new import_obsidian8.Setting(containerEl).setName("Graph colors").setDesc("Colors the nodes of the graph by top tag, by type, by the state of their work, or by how recently they changed. The graph view has the same buttons.").addDropdown((dropdown) => {
      for (const { mode, label } of GRAPH_MODES) dropdown.addOption(mode, label);
      dropdown.setValue(this.plugin.settings.graphColors).onChange((value) => void this.plugin.graphColors.setMode(value));
    });
  }
  /**
   * The agent preferences, from the binary: one group for every vault, one for this vault.
   * A vault key left empty takes the global value, shown as its placeholder.
   */
  async agents(el) {
    let config;
    try {
      config = await this.plugin.agentConfig();
    } catch (e) {
      el.empty();
      el.createDiv({ cls: "setting-item-description", text: `Atlas cannot read the agent preferences: ${e.message}` });
      return;
    }
    el.empty();
    const intro = el.createDiv({ cls: "setting-item-description atlas-setting-intro" });
    intro.setText(`Start agent and Resume read these. ${shortHome(config.files.global)} holds them for every vault; .atlas/config.json in this vault overrides them, key by key. atlas-obsidian config shows the result.`);
    const set = async (key, value, global) => {
      try {
        await this.plugin.setPreference(key, value, global);
      } catch (e) {
        new import_obsidian8.Notice(`Atlas: ${e.message}`, 8e3);
      }
      void this.agents(el);
    };
    const group = (name, global) => {
      new import_obsidian8.Setting(el).setName(name).setHeading();
      const own = global ? config.global : config.vault;
      const same2 = (key, label) => global ? "" : `Same as all vaults (${label || inherited(config, key)})`;
      new import_obsidian8.Setting(el).setName("Agent").addDropdown((d) => {
        if (!global) d.addOption("", same2("agent", AGENT_NAMES[inherited(config, "agent")]));
        for (const a of AGENTS) d.addOption(a, AGENT_NAMES[a]);
        d.setValue(preference(own, "agent") || (global ? "claude" : "")).onChange((v) => void set("agent", v, global));
      });
      for (const a of AGENTS) {
        const key = `agent_commands.${a}`;
        new import_obsidian8.Setting(el).setName(`${AGENT_NAMES[a]} command`).setDesc(global ? `As typed in a shell: ${a}, or a shell function that picks an account.` : "Empty takes the value for all vaults.").addText((t) => {
          t.setPlaceholder(global ? a : inherited(config, key)).setValue(preference(own, key));
          t.inputEl.addEventListener("change", () => void set(key, t.getValue(), global));
        });
      }
      new import_obsidian8.Setting(el).setName("Terminal").setDesc(global ? "Runs the command in your login shell, so your PATH and shell functions apply." : "").addDropdown((d) => {
        if (!global) d.addOption("", same2("terminal", TERMINAL_NAMES[inherited(config, "terminal")]));
        for (const t of TERMINALS) d.addOption(t, TERMINAL_NAMES[t]);
        d.setValue(preference(own, "terminal") || (global ? "terminal" : "")).onChange((v) => void set("terminal", v, global));
      });
      const terminal = preference(own, "terminal") || inherited(config, "terminal");
      if (terminal === "custom") {
        new import_obsidian8.Setting(el).setName("Custom terminal command").setDesc("Runs with /bin/sh. {command} stands for the agent's command, quoted. Example: kitty sh -lic {command}").addText((t) => {
          t.setPlaceholder(global ? "" : inherited(config, "terminal_command")).setValue(preference(own, "terminal_command"));
          t.inputEl.addEventListener("change", () => void set("terminal_command", t.getValue(), global));
        });
      }
    };
    group("All vaults", true);
    group("This vault", false);
  }
};

// src/tagnav.ts
var import_obsidian9 = require("obsidian");
var TAG_NAV_VIEW = "atlas-tag-navigator";
var NAV_ICON = "compass";
var DOC_TYPES = /* @__PURE__ */ new Set(["source", "repository", "topic", "stub", "spec", "event"]);
var MAX_WITH = 30;
function tagDocs(app) {
  const out = [];
  for (const file of app.vault.getMarkdownFiles()) {
    if (!file.path.startsWith("wiki/documents/")) continue;
    const fm = app.metadataCache.getFileCache(file)?.frontmatter;
    if (!fm || !DOC_TYPES.has(String(fm.type))) continue;
    const own = asList(fm.tags).map(normalTag);
    if (typeof fm.defines === "string" && fm.defines) own.push(normalTag(fm.defines));
    out.push({
      path: file.path,
      title: file.basename,
      type: String(fm.type),
      kind: String(fm.kind ?? ""),
      status: String(fm.status ?? ""),
      description: String(fm.description ?? ""),
      tags: own
    });
  }
  return out;
}
var TagNavigator = class extends import_obsidian9.ItemView {
  constructor(leaf, onChoose = () => {
  }, onGraph = () => {
  }) {
    super(leaf);
    this.onChoose = onChoose;
    this.onGraph = onGraph;
  }
  onChoose;
  onGraph;
  chosen = [];
  rerender = (0, import_obsidian9.debounce)(() => this.render(), 500, true);
  getViewType() {
    return TAG_NAV_VIEW;
  }
  getDisplayText() {
    return "Atlas navigator";
  }
  getIcon() {
    return NAV_ICON;
  }
  getState() {
    return { chosen: this.chosen };
  }
  async setState(state, result) {
    const s = state;
    if (s && Array.isArray(s.chosen)) this.chosen = s.chosen.filter((t) => typeof t === "string");
    this.render();
    await super.setState(state, result);
  }
  async onOpen() {
    this.registerEvent(this.app.metadataCache.on("resolved", () => this.rerender()));
    this.registerEvent(this.app.vault.on("rename", () => this.rerender()));
    this.render();
  }
  /** Starts again at one tag. */
  show(tag) {
    this.chosen = [normalTag(tag)];
    this.render();
  }
  add(tag) {
    if (!this.chosen.includes(tag)) this.chosen.push(tag);
    this.render();
  }
  drop(tag) {
    this.chosen = this.chosen.filter((t) => t !== tag);
    this.render();
  }
  render() {
    this.onChoose([...this.chosen]);
    const root = this.contentEl;
    root.empty();
    root.addClass("atlas-tagnav");
    const docs = tagDocs(this.app);
    const path = root.createDiv({ cls: "atlas-tagnav-path" });
    const home = path.createEl("button", { cls: "atlas-tagnav-home", text: "All tags" });
    home.onclick = () => {
      this.chosen = [];
      this.render();
    };
    for (const t of this.chosen) {
      const chip = path.createSpan({ cls: "atlas-tagnav-chip" });
      chip.createSpan({ text: "#" + t });
      chip.setAttr("title", "#" + t);
      const x = chip.createEl("button", { cls: "atlas-tagnav-x", text: "\xD7", attr: { "aria-label": `Remove ${t}` } });
      x.onclick = () => this.drop(t);
    }
    if (this.chosen.length === 0) {
      root.createDiv({ cls: "atlas-tagnav-hint", text: "Choose a tag, then narrow by the tags that occur with it." });
      const list = this.section(root, "Tags");
      for (const f of topTags(docs)) this.tagButton(list, f.tag, f.count);
      if (docs.length === 0) root.createDiv({ cls: "atlas-tagnav-empty", text: "No document holds a tag yet." });
      return;
    }
    const { matches, with: facets } = narrow(docs, this.chosen);
    const count = `${matches.length} ${matches.length === 1 ? "document holds" : "documents hold"} ${this.chosen.length === 1 ? "this tag" : this.chosen.length === 2 ? "both tags" : `all ${this.chosen.length} tags`}`;
    root.createDiv({ cls: "atlas-tagnav-count", text: count });
    const view = this.section(root, "View");
    const search = view.createEl("button", { text: "Search" });
    search.setAttr("aria-label", "Find these documents in Obsidian's search");
    search.onclick = () => this.openSearch();
    const graph = view.createEl("button", { text: "Graph" });
    graph.setAttr("aria-label", "Color these documents in the graph");
    graph.onclick = () => this.onGraph();
    const page = view.createEl("button", { text: "Tag view" });
    page.setAttr("aria-label", "Open the view of #" + this.chosen[this.chosen.length - 1]);
    page.onclick = () => void this.openView(this.chosen[this.chosen.length - 1]);
    if (facets.length > 0) {
      const list = this.section(root, "Narrow");
      for (const f of facets.slice(0, MAX_WITH)) this.tagButton(list, f.tag, f.count, relativeTag(f.tag, this.chosen));
    }
    for (const group of groupDocs(matches)) {
      const g = root.createDiv({ cls: "atlas-tagnav-group" });
      g.createDiv({ cls: "atlas-tagnav-group-name", text: `${group.name} (${group.docs.length})` });
      for (const d of group.docs) {
        const row = g.createDiv({ cls: "atlas-tagnav-doc" });
        row.dataset.type = d.type;
        row.setAttr("title", d.description);
        row.createSpan({ cls: "atlas-tagnav-doc-title", text: d.title });
        const meta = [d.kind || d.type, d.status].filter((x) => x).join(" \xB7 ");
        row.createSpan({ cls: "atlas-tagnav-doc-meta", text: meta });
        row.onclick = (evt) => {
          const file = this.app.vault.getAbstractFileByPath(d.path);
          if (file instanceof import_obsidian9.TFile) void this.app.workspace.getLeaf(evt.metaKey || evt.ctrlKey).openFile(file);
        };
      }
    }
  }
  /** A labeled section; returns the element its items go in. */
  section(root, label) {
    const el = root.createDiv({ cls: "atlas-tagnav-section" });
    el.createDiv({ cls: "atlas-tagnav-label", text: label });
    return el.createDiv({ cls: "atlas-tagnav-items" });
  }
  tagButton(parent, tag, count, label = "#" + tag) {
    const b = parent.createEl("button", { cls: "atlas-tagnav-tag" });
    b.createSpan({ text: label });
    b.setAttr("title", "#" + tag);
    b.createSpan({ cls: "atlas-tagnav-tag-count", text: String(count) });
    b.onclick = () => this.add(tag);
  }
  openSearch() {
    const query = this.chosen.map((t) => `tag:#${t}`).join(" ");
    const search = this.app.internalPlugins?.getPluginById?.("global-search")?.instance;
    if (search?.openGlobalSearch) search.openGlobalSearch(query);
  }
  async openView(tag) {
    const file = this.app.vault.getAbstractFileByPath(tagViewPath(tag));
    if (file instanceof import_obsidian9.TFile) await this.app.workspace.getLeaf(false).openFile(file);
  }
};

// src/viewfolders.ts
var import_obsidian10 = require("obsidian");
var ViewFolders = class extends import_obsidian10.Component {
  constructor(app) {
    super();
    this.app = app;
  }
  app;
  style = null;
  enabled = false;
  refresh = (0, import_obsidian10.debounce)(() => this.apply(), 300, true);
  onload() {
    this.style = document.head.createEl("style", { attr: { id: "atlas-view-folders" } });
    this.registerEvent(this.app.vault.on("create", () => this.refresh()));
    this.registerEvent(this.app.vault.on("delete", () => this.refresh()));
    this.registerEvent(this.app.vault.on("rename", () => this.refresh()));
    this.registerDomEvent(document, "click", (evt) => this.onClick(evt), { capture: true });
    this.app.workspace.onLayoutReady(() => this.refresh());
  }
  onunload() {
    this.style?.remove();
    this.style = null;
  }
  setEnabled(on) {
    this.enabled = on;
    this.apply();
  }
  apply() {
    if (!this.style) return;
    const rules = [
      `.nav-folder-title[data-path^="views/tags/"] .nav-folder-title-content { font-weight: var(--font-semibold); }`
    ];
    if (this.enabled) {
      rules.push(`.nav-folder-title[data-path^="views/tags/"] { cursor: pointer; }`);
      for (const file of this.app.vault.getMarkdownFiles()) {
        if (isTagView(file.path)) rules.push(`.nav-file-title[data-path=${cssString(file.path)}] { display: none; }`);
      }
    }
    this.style.textContent = rules.join("\n");
  }
  onClick(evt) {
    if (!this.enabled || !(evt.target instanceof Element)) return;
    if (evt.target.closest(".nav-folder-collapse-indicator")) return;
    const title = evt.target.closest(".nav-folder-title");
    const tag = tagOfFolder(title?.getAttribute("data-path") ?? "");
    if (!tag) return;
    const note = this.app.vault.getAbstractFileByPath(tagViewPath(tag));
    if (note instanceof import_obsidian10.TFile) void this.app.workspace.getLeaf(import_obsidian10.Keymap.isModEvent(evt)).openFile(note);
  }
};

// src/threadbar.ts
var import_obsidian11 = require("obsidian");
var BAR4 = "atlas-thread-bar";
function barDoc(app, view) {
  if (!view.file) return null;
  const fm = app.metadataCache.getFileCache(view.file)?.frontmatter;
  if (!fm || !fm.id || fm.type !== "stub" && fm.type !== "chord") return null;
  return {
    id: String(fm.id),
    type: fm.type,
    title: view.file.basename,
    status: String(fm.status ?? (fm.type === "stub" ? "stub" : "open")),
    blocked: Boolean(fm.blocked),
    tasks: String(fm.tasks ?? ""),
    threads: String(fm.threads ?? "")
  };
}
function foldProperties(view) {
  const editor = view.metadataEditor;
  try {
    editor?.setCollapse?.(true, false);
  } catch {
  }
}
var ThreadBar = class extends import_obsidian11.Component {
  constructor(plugin) {
    super();
    this.plugin = plugin;
  }
  plugin;
  busy = false;
  refresh = (0, import_obsidian11.debounce)(() => this.update(), 100, true);
  get app() {
    return this.plugin.app;
  }
  onload() {
    const ws = this.app.workspace;
    this.registerEvent(ws.on("active-leaf-change", () => this.refresh()));
    this.registerEvent(ws.on("file-open", () => this.refresh()));
    this.registerEvent(ws.on("layout-change", () => this.refresh()));
    this.registerEvent(this.app.metadataCache.on("changed", () => this.refresh()));
    ws.onLayoutReady(() => this.refresh());
  }
  onunload() {
    document.querySelectorAll(`.${BAR4}`).forEach((el) => el.remove());
  }
  /** Copies the hand-off line of the stub or chord in the active view. */
  copyActive() {
    const view = this.app.workspace.getActiveViewOfType(import_obsidian11.MarkdownView);
    const d = view ? barDoc(this.app, view) : null;
    if (!d) return false;
    void this.copy(d);
    return true;
  }
  async copy(d) {
    const line = handoffLine(d.type, d.id);
    try {
      await navigator.clipboard.writeText(line);
      new import_obsidian11.Notice(`Atlas: copied "${line}". Paste it into an agent session.`);
    } catch {
      new import_obsidian11.Notice(`Atlas: ${line}`);
    }
  }
  update() {
    for (const leaf of this.app.workspace.getLeavesOfType("markdown")) {
      if (leaf.view instanceof import_obsidian11.MarkdownView) this.updateView(leaf.view);
    }
  }
  updateView(view) {
    const d = barDoc(this.app, view);
    const existing = view.containerEl.querySelector(`:scope > .${BAR4}`);
    if (!d) {
      existing?.remove();
      return;
    }
    const key = [d.id, d.status, d.blocked, d.tasks, d.threads, d.title].join("\n");
    if (existing?.dataset.key === key) return;
    if (existing?.dataset.id !== d.id) foldProperties(view);
    existing?.remove();
    const bar = createDiv({ cls: BAR4 });
    bar.dataset.key = key;
    bar.dataset.id = d.id;
    bar.dataset.status = d.blocked ? "blocked" : d.status;
    bar.createSpan({ cls: "atlas-thread-bar-label", text: d.type === "stub" ? "Thread" : "Chord" });
    bar.createSpan({ cls: "atlas-thread-bar-status", text: barStatus(d) });
    const buttons = bar.createDiv({ cls: "atlas-thread-bar-buttons" });
    const tool = d.type === "stub" ? "thread" : "chord";
    const noun = d.type === "stub" ? "thread" : "chord";
    for (const b of barButtons(d)) {
      const el = buttons.createEl("button", { text: b.label });
      if (b.id === "agent") el.addClass("mod-cta");
      el.onclick = () => {
        switch (b.id) {
          case "agent":
            return void this.plugin.startAgent(d.type, d.id);
          case "new":
            return this.plugin.newThread({ id: d.id, title: d.title });
          case "handoff":
            return void this.copy(d);
          case "canvas":
            return void this.app.workspace.openLinkText(`chords/${d.title}.canvas`, "", false);
          case "unblock":
            return void this.run([tool, "unblock", d.id], "Unblocked");
          case "block":
            return new ReasonModal2(this.app, `Block the ${noun}`, "What it waits on, in one line", true, (r) => void this.run([tool, "block", d.id, "--reason", r], "Blocked")).open();
          case "drop":
            return new ReasonModal2(this.app, `Drop the ${noun}`, "Why", true, (r) => void this.run([tool, "drop", d.id, "--reason", r], "Dropped")).open();
          case "reopen":
            return new ReasonModal2(this.app, `Reopen the ${noun}`, "Why (optional)", false, (r) => void this.run([tool, "reopen", d.id, "--reason", r], "Reopened")).open();
        }
      };
    }
    view.containerEl.insertBefore(bar, view.contentEl);
  }
  async run(args, done) {
    if (this.busy) return;
    this.busy = true;
    try {
      await this.plugin.atlas(args);
      new import_obsidian11.Notice(`Atlas: ${done}.`);
    } catch (e) {
      new import_obsidian11.Notice(`Atlas: ${e.message}`);
    } finally {
      this.busy = false;
      this.refresh();
    }
  }
};
var ReasonModal2 = class extends import_obsidian11.Modal {
  constructor(app, heading, placeholder, required, done) {
    super(app);
    this.heading = heading;
    this.placeholder = placeholder;
    this.required = required;
    this.done = done;
  }
  heading;
  placeholder;
  required;
  done;
  reason = "";
  onOpen() {
    this.setTitle(this.heading);
    const submit = () => {
      if (this.required && this.reason.trim() === "") return;
      this.close();
      this.done(this.reason.trim());
    };
    new import_obsidian11.Setting(this.contentEl).setName("Reason").addText((text) => {
      text.setPlaceholder(this.placeholder).onChange((v) => this.reason = v);
      text.inputEl.addClass("atlas-reason-input");
      text.inputEl.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.isComposing) {
          e.preventDefault();
          submit();
        }
      });
      window.setTimeout(() => text.inputEl.focus(), 0);
    });
    new import_obsidian11.Setting(this.contentEl).addButton((b) => b.setButtonText("Cancel").onClick(() => this.close())).addButton((b) => b.setButtonText(this.heading).setCta().onClick(submit));
  }
  onClose() {
    this.contentEl.empty();
  }
};

// src/main.ts
var SYNC_DELAY = 2e3;
var LEGACY_KEYS = ["agentCommand", "terminal", "terminalCommand"];
var ECHO_WINDOW = 5e3;
var AtlasPlugin = class extends import_obsidian12.Plugin {
  settings = { ...DEFAULT_SETTINGS };
  /** The agent settings of 8.0.2 and 8.0.3, kept in data.json until they move to the vault's config file. */
  legacy = null;
  badges;
  viewFolders;
  graphColors;
  syncing = false;
  syncTimer = null;
  pending = /* @__PURE__ */ new Set();
  echoes = /* @__PURE__ */ new Set();
  echoUntil = 0;
  lastAutoError = "";
  sessionsRibbon = null;
  statusItem = null;
  async onload() {
    await this.loadSettings();
    this.addSettingTab(new AtlasSettingTab(this.app, this));
    this.badges = this.addChild(new Badges(this.app));
    this.badges.setEnabled(this.settings.badges);
    this.addChild(new ChangeBar(this));
    const threadBar = this.addChild(new ThreadBar(this));
    this.addChild(new CanvasBar(this));
    this.addCommand({
      id: "copy-handoff",
      name: "Copy the hand-off line of this thread or chord",
      checkCallback: (checking) => {
        const file = this.app.workspace.getActiveFile();
        const fm = file ? this.app.metadataCache.getFileCache(file)?.frontmatter : void 0;
        if (fm?.type !== "stub" && fm?.type !== "chord") return false;
        if (!checking) threadBar.copyActive();
        return true;
      }
    });
    this.viewFolders = this.addChild(new ViewFolders(this.app));
    this.viewFolders.setEnabled(this.settings.viewFolders);
    this.registerMarkdownCodeBlockProcessor("atlas-repo", repoProcessor(this));
    this.graphColors = this.addChild(new GraphColors(this));
    for (const { mode, label } of GRAPH_MODES) {
      this.addCommand({
        id: `graph-colors-${mode}`,
        name: mode === "off" ? "Stop coloring the graph" : `Color the graph by ${label.toLowerCase()}`,
        callback: () => void this.graphColors.setMode(mode)
      });
    }
    this.addRibbonIcon("refresh-cw", "Atlas: sync the vault", () => void this.sync(true));
    this.addCommand({ id: "sync", name: "Sync the vault", callback: () => void this.sync(true) });
    this.registerView(
      TAG_NAV_VIEW,
      (leaf) => new TagNavigator(
        leaf,
        (tags) => void this.graphColors.setFocus(tags),
        () => void this.focusGraph()
      )
    );
    this.addRibbonIcon(NAV_ICON, "Atlas: open the Atlas navigator", () => void this.openTags());
    this.addCommand({ id: "open-tags", name: "Open the Atlas navigator", callback: () => void this.openTags() });
    this.registerDomEvent(document, "click", (evt) => this.onTagClick(evt), { capture: true });
    this.addCommand({
      id: "start-agent",
      name: "Start an agent on this thread or chord",
      checkCallback: (checking) => {
        const target = this.activeWork();
        if (!target) return false;
        if (!checking) void this.startAgent(target.type, target.id);
        return true;
      }
    });
    this.addCommand({
      id: "new-thread-in-chord",
      name: "New thread in this chord",
      checkCallback: (checking) => {
        const chord = this.activeChord();
        if (!chord) return false;
        if (!checking) this.newThread(chord);
        return true;
      }
    });
    this.registerView(SESSIONS_VIEW, (leaf) => new SessionsView(leaf, this));
    this.sessionsRibbon = this.addRibbonIcon("bot", "Atlas: open the sessions", () => void this.openSessions());
    this.sessionsRibbon.addClass("atlas-sessions-ribbon");
    this.addCommand({ id: "open-sessions", name: "Open the sessions", callback: () => void this.openSessions() });
    this.statusItem = this.addStatusBarItem();
    this.statusItem.addClass("atlas-status-waiting");
    this.statusItem.onClickEvent(() => void this.openSessions());
    this.addCommand({ id: "migrate", name: "Migrate this vault to the 8.0 layout", callback: () => void this.migrate() });
    this.registerEditorExtension(mentionEditor);
    this.registerMarkdownPostProcessor(mentionReading);
    this.registerEvent(
      this.app.metadataCache.on("changed", (file) => {
        if (file.path.startsWith("sessions/")) this.refreshSessions();
        this.onDocChange(file.path);
      })
    );
    this.registerEvent(
      this.app.vault.on("delete", (file) => {
        this.refreshSessions();
        this.onDocChange(file.path);
      })
    );
    this.registerEvent(
      this.app.vault.on("rename", (file, oldPath) => {
        this.refreshSessions();
        this.onDocChange(file.path);
        this.onDocChange(oldPath);
      })
    );
    this.registerInterval(window.setInterval(() => this.sessionViews().forEach((v) => v.tick()), 3e4));
    this.app.workspace.onLayoutReady(() => {
      this.refreshSessions();
      this.checkLayout();
      void this.moveLegacyPreferences();
    });
    const first = this.app.metadataCache.on("resolved", () => {
      this.app.metadataCache.offref(first);
      this.refreshSessions();
    });
    this.registerEvent(first);
  }
  onunload() {
    if (this.syncTimer !== null) window.clearTimeout(this.syncTimer);
  }
  /** Colors the graph by the navigator's tags, and opens the graph. */
  async focusGraph() {
    await this.graphColors.setMode("focus");
    const open = this.app.workspace.getLeavesOfType("graph")[0];
    if (open) this.app.workspace.revealLeaf(open);
    else this.app.commands?.executeCommandById?.("graph:open");
  }
  async loadSettings() {
    const saved = await this.loadData();
    const old = Object.entries(saved ?? {}).filter(([k]) => LEGACY_KEYS.includes(k));
    this.legacy = old.length > 0 ? Object.fromEntries(old) : null;
    this.settings = { ...DEFAULT_SETTINGS };
    for (const key of Object.keys(DEFAULT_SETTINGS)) {
      if (saved && saved[key] !== void 0) this.settings[key] = saved[key];
    }
    if (saved?.folderPages !== void 0 && saved.viewFolders === void 0) this.settings.viewFolders = saved.folderPages;
    if (!isGraphMode(this.settings.graphColors)) this.settings.graphColors = DEFAULT_SETTINGS.graphColors;
    for (const key of ["graphOwned", "focusTags"]) {
      const list = this.settings[key];
      this.settings[key] = Array.isArray(list) ? list.filter((x) => typeof x === "string") : [];
    }
  }
  async saveSettings() {
    await this.saveData({ ...this.legacy, ...this.settings });
  }
  /** Runs one atlas command in this vault and returns its JSON. */
  atlas(args) {
    const adapter = this.app.vault.adapter;
    if (!(adapter instanceof import_obsidian12.FileSystemAdapter)) {
      return Promise.reject(new AtlasError("this vault is not a folder on disk"));
    }
    return runAtlas(findBinary(this.settings.binaryPath), adapter.getBasePath(), args);
  }
  // Sync
  /** A manual sync heals everything; an automatic one runs the steps that read no git. */
  async sync(manual) {
    if (this.syncing) {
      if (manual) new import_obsidian12.Notice("Atlas: a sync is running.");
      return;
    }
    if (this.syncTimer !== null) window.clearTimeout(this.syncTimer);
    this.syncTimer = null;
    this.syncing = true;
    this.pending.clear();
    let wrote = [];
    try {
      const args = manual ? ["vault", "sync"] : ["vault", "sync", "--views"];
      const out = await this.atlas(args);
      wrote = syncedPaths(out.synced);
      this.lastAutoError = "";
      if (manual) new import_obsidian12.Notice(`Atlas: ${syncSummary(out.synced)}`);
    } catch (e) {
      const message = e.message;
      if (manual || message !== this.lastAutoError) new import_obsidian12.Notice(`Atlas: ${message}`);
      if (!manual) this.lastAutoError = message;
    } finally {
      this.syncing = false;
      this.echoes = new Set(wrote);
      this.echoUntil = Date.now() + ECHO_WINDOW;
      const left = [];
      for (const p of this.pending) {
        if (this.echoes.has(p)) this.echoes.delete(p);
        else left.push(p);
      }
      this.pending.clear();
      if (left.length > 0) this.scheduleSync();
    }
  }
  onDocChange(path) {
    if (!this.settings.syncOnChange || !isWatchedPath(path) || !this.migrated()) return;
    if (this.syncing) {
      this.pending.add(path);
      return;
    }
    if (Date.now() < this.echoUntil && this.echoes.has(path)) {
      this.echoes.delete(path);
      return;
    }
    this.scheduleSync();
  }
  scheduleSync() {
    if (this.syncTimer !== null) window.clearTimeout(this.syncTimer);
    this.syncTimer = window.setTimeout(() => {
      this.syncTimer = null;
      void this.sync(false);
    }, SYNC_DELAY);
  }
  // Layout
  /** The layout the vault document records; this plugin's when there is none to read. */
  layout() {
    const atlas = this.app.vault.getFileByPath("Atlas.md");
    if (!atlas) return LAYOUT;
    return layoutOf(this.app.metadataCache.getFileCache(atlas)?.frontmatter);
  }
  /** Whether the vault has the 8.0 layout. */
  migrated() {
    return this.layout() >= LAYOUT;
  }
  checkLayout() {
    if (this.migrated()) return;
    const notice = new import_obsidian12.Notice("", 0);
    const el = notice.messageEl;
    el.createDiv({ text: `Atlas: this vault has the ${layoutName(this.layout())} layout. This plugin needs the 8.0 layout: threads and chords.` });
    const button = el.createEl("button", { text: "Show the migration", cls: "mod-cta atlas-notice-button" });
    button.onclick = () => {
      notice.hide();
      void this.migrate();
    };
  }
  async migrate() {
    try {
      const report = await this.atlas(["vault", "migrate", "--dry-run"]);
      new MigrationModal(this.app, report, async () => {
        try {
          const done = await this.atlas(["vault", "migrate"]);
          new import_obsidian12.Notice(`Atlas: migrated in one commit, ${String(done.commit ?? "").slice(0, 7)}.${done.problems ? ` Lint finds ${done.problems} errors.` : ""}`, 1e4);
        } catch (e) {
          new import_obsidian12.Notice(`Atlas: ${e.message}`, 1e4);
        }
      }).open();
    } catch (e) {
      new import_obsidian12.Notice(`Atlas: ${e.message}`, 1e4);
    }
  }
  // Tags
  onTagClick(evt) {
    if (!this.settings.tagClick || !(evt.target instanceof Element)) return;
    const el = evt.target.closest("a.tag, .cm-hashtag");
    if (!el) return;
    let tag = el.getAttribute("href") ?? el.textContent ?? "";
    if (el.classList.contains("cm-hashtag")) {
      const line = el.closest(".cm-line");
      const parts = line ? Array.from(line.querySelectorAll(".cm-hashtag")) : [el];
      const i = parts.indexOf(el);
      const begin = parts[i]?.classList.contains("cm-hashtag-begin") ? i : i - 1;
      tag = (parts[begin]?.textContent ?? "") + (parts[begin + 1]?.textContent ?? "");
    }
    tag = normalTag(tag);
    if (!tag) return;
    evt.preventDefault();
    evt.stopPropagation();
    void this.openTags(tag);
  }
  async openTags(tag) {
    const { workspace } = this.app;
    let leaf = workspace.getLeavesOfType(TAG_NAV_VIEW)[0];
    if (!leaf) {
      const left = workspace.getLeftLeaf(false);
      if (!left) return;
      await left.setViewState({ type: TAG_NAV_VIEW, active: true });
      leaf = left;
    }
    await workspace.revealLeaf(leaf);
    if (tag && leaf.view instanceof TagNavigator) leaf.view.show(tag);
  }
  // Sessions
  sessionViews() {
    return this.app.workspace.getLeavesOfType(SESSIONS_VIEW).map((leaf) => leaf.view).filter((v) => v instanceof SessionsView);
  }
  refreshSessions = (0, import_obsidian12.debounce)(
    () => {
      void sessionGroups(this.app, this.staleHours()).then(({ groups }) => {
        const waiting = groups.open.filter((s) => s.state === "needs you").length;
        this.statusItem?.setText(waiting > 0 ? waitingLabel(waiting) : "");
        this.statusItem?.toggleClass("is-hidden", waiting === 0);
        if (this.sessionsRibbon) {
          if (waiting > 0) this.sessionsRibbon.dataset.atlasCount = String(waiting);
          else delete this.sessionsRibbon.dataset.atlasCount;
        }
      });
      this.sessionViews().forEach((v) => void v.render());
    },
    500,
    true
  );
  /** How long a session with no recorded process may go quiet before it counts as gone. */
  staleHours() {
    const atlas = this.app.vault.getFileByPath("Atlas.md");
    const n = Number(atlas ? this.app.metadataCache.getFileCache(atlas)?.frontmatter?.stale_hours : 0);
    return n > 0 ? n : 12;
  }
  // Agents
  /** The agent preferences: the vault's config file over ~/.atlas/config.json. */
  agentConfig() {
    return this.atlas(["config"]);
  }
  /** Sets or unsets (value "") one agent preference, in the vault's file or the global one. */
  async setPreference(key, value, global) {
    const args = value ? ["config", "set", key, value] : ["config", "unset", key];
    return this.atlas(global ? [...args, "--global"] : args);
  }
  /** Moves the agent settings of 8.0.2 and 8.0.3 into the vault's config file, once. */
  async moveLegacyPreferences() {
    const saved = this.legacy;
    if (!saved) return;
    try {
      const config = await this.agentConfig();
      for (const [key, value] of legacyPreferences(saved, config.vault)) await this.setPreference(key, value, false);
      this.legacy = null;
      await this.saveSettings();
    } catch (e) {
      console.warn("Atlas: the agent settings did not move to .atlas/config.json", e);
    }
  }
  /** Runs a command in a new terminal; off macOS, or when that fails, copies it. */
  async runInTerminal(command, what, config) {
    if (process.platform === "darwin") {
      try {
        const prefs = (config ?? await this.agentConfig()).preferences;
        await openTerminal(prefs.terminal, command, prefs.terminal_command);
        return;
      } catch (e) {
        new import_obsidian12.Notice(`Atlas: cannot open the terminal (${e.message}). The Atlas settings choose it.`, 8e3);
      }
    }
    await navigator.clipboard.writeText(command);
    new import_obsidian12.Notice(`Atlas: copied ${what}. Run it in a terminal.`);
  }
  /** Starts an agent in the vault, with the hand-off line of a thread or a chord as its first prompt. */
  async startAgent(type, id) {
    const adapter = this.app.vault.adapter;
    if (!(adapter instanceof import_obsidian12.FileSystemAdapter)) return;
    const prompt = `Resume Atlas ${type === "chord" ? "chord" : "thread"} ${id}`;
    let config;
    try {
      config = await this.agentConfig();
    } catch (e) {
      new import_obsidian12.Notice(`Atlas: cannot read the agent preferences: ${e.message}`, 8e3);
      return;
    }
    await this.runInTerminal(startCommand(adapter.getBasePath(), config.preferences.agent_command, prompt), "the agent command", config);
  }
  /** The stub or chord open in the active view, or the chord of an open canvas. */
  activeWork() {
    const file = this.app.workspace.getActiveFile();
    if (!file) return null;
    if (file.extension === "canvas") {
      const chord = this.activeChord();
      return chord ? { type: "chord", ...chord } : null;
    }
    const fm = this.app.metadataCache.getFileCache(file)?.frontmatter;
    if ((fm?.type === "stub" || fm?.type === "chord") && fm.id) return { type: fm.type, id: String(fm.id), title: file.basename };
    return null;
  }
  /** The chord of the active note or canvas, or null. */
  activeChord() {
    const file = this.app.workspace.getActiveFile();
    if (!file) return null;
    const title = file.basename;
    const note = file.extension === "canvas" ? this.app.metadataCache.getFirstLinkpathDest(title, "") : file;
    const fm = note ? this.app.metadataCache.getFileCache(note)?.frontmatter : void 0;
    if (fm?.type !== "chord" || !fm.id) return null;
    return { id: String(fm.id), title };
  }
  /** Asks for a new thread's title and idea, and plants it in a chord. */
  newThread(chord) {
    new NewThreadModal(this.app, chord.title, async (title, idea) => {
      try {
        await this.atlas(["thread", "stub", idea || title, "--title", title, "--chord", chord.id]);
        new import_obsidian12.Notice(`Atlas: planted ${title} in ${chord.title}.`);
      } catch (e) {
        new import_obsidian12.Notice(`Atlas: ${e.message}`, 8e3);
      }
    }).open();
  }
  async openSessions() {
    const { workspace } = this.app;
    let leaf = workspace.getLeavesOfType(SESSIONS_VIEW)[0];
    if (!leaf) {
      const right = workspace.getRightLeaf(false);
      if (!right) return;
      await right.setViewState({ type: SESSIONS_VIEW, active: true });
      leaf = right;
    }
    await workspace.revealLeaf(leaf);
  }
};
var MigrationModal = class extends import_obsidian12.Modal {
  constructor(app, report, run) {
    super(app);
    this.report = report;
    this.run = run;
  }
  report;
  run;
  onOpen() {
    const r = this.report;
    this.setTitle("Migrate to the 8.0 layout");
    const el = this.contentEl;
    el.addClass("atlas-migration");
    if (r.from === "6.x") {
      el.createEl("p", { text: `The migration of ${r.vault} moves ${r.documents} documents into wiki/documents, writes ${r.events} events, and moves ${r.assets} files into wiki/assets. Then it makes each plan a thread: a stub, a spec, a task list, and a verification. It is one commit; git revert takes it back.` });
    } else {
      el.createEl("p", { text: `The migration of ${r.vault} makes each plan a thread, in one commit: ${r.threads ?? 0} threads, ${r.specs ?? 0} specs, ${r.task_lists ?? 0} task lists, ${r.verifications ?? 0} verifications, ${r.chords ?? 0} chords. Each plan keeps its id, its title, and its file, as the stub. ${r.notes ?? 0} notes keep the sections a spec does not hold. git revert takes it back.` });
    }
    const list = (title, rows) => {
      if (rows.length === 0) return;
      const d = el.createEl("details");
      d.createEl("summary", { text: `${title} (${rows.length})` });
      const ul = d.createEl("ul");
      for (const row of rows) ul.createEl("li", { text: row });
    };
    list("Tags from the scope tree", (r.tags ?? []).map((t) => `${t.scope} \u2192 #${t.tag}`));
    list("Titles that change; links follow", (r.retitles ?? []).map((t) => `${t.old} \u2192 ${t.new}`));
    list("Notes with no type, to the inbox", r.inbox ?? []);
    list("Files to the scratchpad", r.scratchpad ?? []);
    list("Warnings", r.warnings ?? []);
    const buttons = el.createDiv({ cls: "atlas-migration-buttons" });
    buttons.createEl("button", { text: "Cancel" }).onclick = () => this.close();
    const go = buttons.createEl("button", { text: "Migrate", cls: "mod-cta" });
    go.onclick = async () => {
      go.disabled = true;
      await this.run();
      this.close();
    };
  }
  onClose() {
    this.contentEl.empty();
  }
};
