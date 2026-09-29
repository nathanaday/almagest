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
var import_obsidian7 = require("obsidian");

// src/badges.ts
var import_obsidian = require("obsidian");
var BADGE = "atlas-badge";
function badgeFor(app, path) {
  const file = app.vault.getAbstractFileByPath(path);
  if (!(file instanceof import_obsidian.TFile) || file.extension !== "md") return null;
  const fm = app.metadataCache.getFileCache(file)?.frontmatter;
  if (!fm) return null;
  if (fm.type === "stub" && typeof fm.stage === "string") return { kind: "stage", value: fm.stage };
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
  const threads = s.threads?.length ?? 0;
  const lost = s.lost?.length ?? 0;
  const sessions = s.sessions?.length ?? 0;
  if (threads) parts.push(plural(threads, "thread document", "thread documents"));
  if (lost) parts.push(plural(lost, "lost session", "lost sessions"));
  if (sessions) parts.push(plural(sessions, "session callout", "session callouts"));
  const scopes = s.scopes?.length ?? 0;
  if (scopes) parts.push(plural(scopes, "wiki page", "wiki pages"));
  if (s.settings) parts.push("the harness settings");
  if (parts.length === 0) return "Nothing to heal.";
  return `Synced ${parts.join(", ")}.`;
}
function syncedPaths(s) {
  return [...s.threads ?? [], ...s.lost ?? [], ...s.sessions ?? [], ...s.scopes ?? []];
}
function countsLine(counts) {
  if (typeof counts === "string") {
    return counts.split(",").map((p) => p.trim()).filter((p) => p !== "" && !/^0\s/.test(p)).join(", ");
  }
  if (counts && typeof counts === "object") {
    const names = {
      create: "create",
      modify: "modify",
      rename: "rename",
      remove: "remove",
      link_rewrites: "link rewrites"
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
function compareSessions(a, b) {
  const rank = (s) => s === "waiting" ? 0 : 1;
  if (rank(a.status) !== rank(b.status)) return rank(a.status) - rank(b.status);
  return String(b.updated).localeCompare(String(a.updated));
}
function shellQuote(s) {
  return `'${s.replace(/'/g, `'\\''`)}'`;
}
function appleScriptString(s) {
  return `"${s.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}
function resumeCommand(session, home) {
  const id = session.harness_id?.trim();
  if (!id) return null;
  const resume2 = session.harness === "codex" ? `codex resume ${shellQuote(id)}` : `claude --resume ${shellQuote(id)}`;
  const cwd = session.cwd?.trim();
  if (!cwd) return resume2;
  return `cd ${shellQuote(expandHome(cwd, home))} && ${resume2}`;
}
function terminalArgs(command) {
  return [
    "-e",
    `tell application "Terminal" to do script ${appleScriptString(command)}`,
    "-e",
    'tell application "Terminal" to activate'
  ];
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
function isThreadPath(path) {
  return path.startsWith("threads/") && path.endsWith(".md");
}
function waitingLabel(n) {
  return n === 1 ? "Atlas: 1 session waits" : `Atlas: ${n} sessions wait`;
}
function baseName(path) {
  return path.slice(path.lastIndexOf("/") + 1);
}
function dirName(path) {
  const i = path.lastIndexOf("/");
  return i < 0 ? "" : path.slice(0, i);
}
function folderPagePath(folder) {
  if (!folder.startsWith("wiki/")) return null;
  return `${folder}/${baseName(folder)}.md`;
}
function isFolderPage(path) {
  return folderPagePath(dirName(path)) === path;
}
function companionRename(isFolder, path, oldPath) {
  if (!path.startsWith("wiki/")) return null;
  if (isFolder) {
    const oldName = baseName(oldPath);
    const name2 = baseName(path);
    if (oldName === name2) return null;
    return { from: `${path}/${oldName}.md`, to: `${path}/${name2}.md` };
  }
  if (!isFolderPage(oldPath) || dirName(path) !== dirName(oldPath) || !path.endsWith(".md")) return null;
  const folder = dirName(path);
  const name = baseName(path).slice(0, -3);
  if (name === "" || name === baseName(folder)) return null;
  return { from: folder, to: `${dirName(folder)}/${name}` };
}
function cssString(s) {
  return `"${s.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, "\\a ")}"`;
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
function runProgram(bin, args) {
  return exec(bin, args, void 0);
}

// src/folders.ts
var import_obsidian3 = require("obsidian");
var SCOPE_TYPES = /* @__PURE__ */ new Set(["area", "repository"]);
var wait = (ms) => new Promise((resolve) => window.setTimeout(resolve, ms));
var ScopeFolders = class extends import_obsidian3.Component {
  constructor(app, onMove) {
    super();
    this.app = app;
    this.onMove = onMove;
  }
  app;
  onMove;
  style = null;
  enabled = false;
  scopes = /* @__PURE__ */ new Set();
  refresh = (0, import_obsidian3.debounce)(() => this.apply(), 300, true);
  onload() {
    this.style = document.head.createEl("style", { attr: { id: "atlas-scope-folders" } });
    this.registerEvent(this.app.metadataCache.on("changed", () => this.refresh()));
    this.registerEvent(this.app.metadataCache.on("resolved", () => this.refresh()));
    this.registerEvent(this.app.vault.on("delete", () => this.refresh()));
    this.registerEvent(
      this.app.vault.on("rename", (file, oldPath) => {
        this.refresh();
        if (!file.path.startsWith("wiki/") && !oldPath.startsWith("wiki/")) return;
        void this.follow(file instanceof import_obsidian3.TFolder, file.path, oldPath).finally(() => this.onMove());
      })
    );
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
  isScopePage(file) {
    const type = this.app.metadataCache.getFileCache(file)?.frontmatter?.type;
    return typeof type === "string" && SCOPE_TYPES.has(type);
  }
  /** The page that makes a folder a scope, or null. */
  pageOf(folder) {
    const path = folderPagePath(folder);
    const file = path ? this.app.vault.getAbstractFileByPath(path) : null;
    return file instanceof import_obsidian3.TFile && this.isScopePage(file) ? file : null;
  }
  apply() {
    this.scopes.clear();
    for (const file of this.app.vault.getMarkdownFiles()) {
      if (isFolderPage(file.path) && this.isScopePage(file)) this.scopes.add(file.parent?.path ?? "");
    }
    if (!this.style) return;
    const rules = [];
    for (const folder of this.scopes) {
      rules.push(`.nav-folder-title[data-path=${cssString(folder)}] .nav-folder-title-content { font-weight: var(--font-semibold); }`);
      if (this.enabled) {
        rules.push(`.nav-folder-title[data-path=${cssString(folder)}] { cursor: pointer; }`);
        rules.push(`.nav-file-title[data-path=${cssString(folderPagePath(folder) ?? "")}] { display: none; }`);
      }
    }
    this.style.textContent = rules.join("\n");
  }
  onClick(evt) {
    if (!this.enabled || !(evt.target instanceof Element)) return;
    if (evt.target.closest(".nav-folder-collapse-indicator")) return;
    const title = evt.target.closest(".nav-folder-title");
    const folder = title?.getAttribute("data-path") ?? "";
    if (!this.scopes.has(folder)) return;
    const page = this.pageOf(folder);
    if (page) void this.app.workspace.getLeaf(import_obsidian3.Keymap.isModEvent(evt)).openFile(page);
  }
  /** Renames the folder or the page that must follow the user's rename. */
  async readsAsScopePage(file) {
    if (this.isScopePage(file)) return true;
    const info = (0, import_obsidian3.getFrontMatterInfo)(await this.app.vault.cachedRead(file));
    if (!info.exists) return false;
    try {
      const type = (0, import_obsidian3.parseYaml)(info.frontmatter)?.type;
      return typeof type === "string" && SCOPE_TYPES.has(type);
    } catch {
      return false;
    }
  }
  async follow(isFolder, path, oldPath) {
    const next = companionRename(isFolder, path, oldPath);
    if (!next) return;
    let from = this.app.vault.getAbstractFileByPath(next.from);
    for (let i = 0; !from && i < 20; i++) {
      await wait(50);
      from = this.app.vault.getAbstractFileByPath(next.from);
    }
    if (!from || this.app.vault.getAbstractFileByPath(next.to)) return;
    const page = isFolder ? from : this.app.vault.getAbstractFileByPath(path);
    if (!(page instanceof import_obsidian3.TFile) || !isFolder && !(from instanceof import_obsidian3.TFolder)) return;
    if (!await this.readsAsScopePage(page)) return;
    await this.renameKeepingLinks(from, next.to);
  }
  /**
   * Renames a file or folder and updates the links to it, whatever the user's setting for
   * links says: a link names a title, so a title that changes without its links breaks
   * them. The setting is not public API; without it the rename follows the setting.
   */
  async renameKeepingLinks(file, to) {
    const vault = this.app.vault;
    const was = vault.getConfig?.("alwaysUpdateLinks");
    if (was === false) vault.setConfig?.("alwaysUpdateLinks", true);
    try {
      await this.app.fileManager.renameFile(file, to);
    } finally {
      if (was === false) vault.setConfig?.("alwaysUpdateLinks", false);
    }
  }
};

// src/graphcolors.ts
var import_obsidian4 = require("obsidian");

// src/graphgroups.ts
var GRAPH_MODES = [
  { mode: "off", label: "Off" },
  { mode: "area", label: "Area" },
  { mode: "type", label: "Type" },
  { mode: "threads", label: "Threads" },
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
var THREAD_TYPES = ["stub", "spec", "task", "receipt"];
var TYPE_GROUPS = [
  { name: "Areas", types: ["area"] },
  { name: "Repositories", types: ["repository"] },
  { name: "Concepts", types: ["concept"] },
  { name: "Entities", types: ["entity"] },
  { name: "Policies", types: ["policy"] },
  { name: "Sources", types: ["source"] },
  { name: "Threads", types: THREAD_TYPES },
  { name: "Sessions and changes", types: ["session", "change"] }
];
var QUARTERS = ["Newest 25%", "25\u201350%", "50\u201375%", "Oldest 25%"];
function graphGroups(mode, docs, resolve, theme) {
  const vault = new Vault(docs, resolve);
  let groups;
  switch (mode) {
    case "area":
      groups = vault.byArea(theme);
      break;
    case "type":
      groups = vault.byType(theme);
      break;
    case "threads":
      groups = vault.byThreads(theme);
      break;
    case "activity":
      groups = byActivity(docs, theme);
      break;
    default:
      groups = [];
  }
  return groups.filter((g) => g.paths.length > 0);
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
  areas = /* @__PURE__ */ new Map();
  type(path) {
    const t = path === null ? void 0 : this.byPath.get(path)?.fields.type;
    return typeof t === "string" ? t : "";
  }
  links(doc, field) {
    return asList(doc.fields[field]).map((l) => this.resolve(linkTitle(l), doc.path)).filter((p) => p !== null && this.byPath.has(p));
  }
  byArea(theme) {
    const members = /* @__PURE__ */ new Map();
    for (const d of this.docs) {
      const area = this.areaOf(d.path);
      if (area === null) continue;
      members.set(area, [...members.get(area) ?? [], d.path]);
    }
    const order = [...members.keys()].sort((a, b) => {
      const ca = String(this.byPath.get(a)?.fields.created ?? "");
      const cb = String(this.byPath.get(b)?.fields.created ?? "");
      return ca.localeCompare(cb) || basename(a).localeCompare(basename(b));
    });
    const palette = CATEGORICAL[theme];
    const groups = order.slice(0, palette.length).map((area, i) => ({
      name: basename(area),
      color: palette[i],
      paths: members.get(area) ?? []
    }));
    const rest = order.slice(palette.length).flatMap((area) => members.get(area) ?? []);
    if (rest.length > 0) groups.push({ name: "Other areas", color: MUTED, paths: rest });
    return groups;
  }
  /** The nearest area of a document: the area it is about, or the area of its scope or thread. */
  areaOf(path, seen = /* @__PURE__ */ new Set()) {
    if (this.areas.has(path)) return this.areas.get(path) ?? null;
    if (seen.has(path)) return null;
    seen.add(path);
    const area = this.findArea(path, seen);
    this.areas.set(path, area);
    return area;
  }
  findArea(path, seen) {
    const doc = this.byPath.get(path);
    if (!doc) return null;
    const first = (paths) => {
      for (const p of paths) {
        const a = this.areaOf(p, seen);
        if (a !== null) return a;
      }
      return null;
    };
    switch (this.type(path)) {
      case "area":
        return path;
      case "repository":
      case "concept":
      case "entity":
      case "policy":
      case "source": {
        const chain = this.links(doc, "chain").filter((p) => this.type(p) === "area");
        if (chain.length > 0) return chain[chain.length - 1];
        return first([...this.links(doc, "parent"), ...this.links(doc, "scope")]);
      }
      case "stub":
        return first(this.links(doc, "scope"));
      case "spec":
      case "task":
      case "receipt":
      case "change":
        return first(this.links(doc, "thread"));
      case "session":
        return first([...this.links(doc, "threads"), ...this.links(doc, "repositories")]);
    }
    return null;
  }
  byType(theme) {
    const palette = CATEGORICAL[theme];
    return TYPE_GROUPS.map((g, i) => ({
      name: g.name,
      color: palette[i],
      paths: this.docs.filter((d) => g.types.includes(this.type(d.path))).map((d) => d.path)
    }));
  }
  byThreads(theme) {
    const open = [];
    const closed = [];
    const none = [];
    for (const d of this.docs) {
      const states = this.threadStates(d);
      if (states.has("open")) open.push(d.path);
      else if (states.has("closed")) closed.push(d.path);
      else none.push(d.path);
    }
    const palette = CATEGORICAL[theme];
    return [
      { name: "Open threads", color: palette[1], paths: open },
      { name: "Closed threads", color: palette[0], paths: closed },
      { name: "No threads", color: MUTED, paths: none }
    ];
  }
  /** The stubs a document belongs to. */
  threadsOf(doc) {
    switch (this.type(doc.path)) {
      case "stub":
        return [doc.path];
      case "spec":
      case "task":
      case "receipt":
      case "change":
        return this.links(doc, "thread");
      case "session":
        return this.links(doc, "threads");
    }
    return [];
  }
  /**
   * The states of the threads a document belongs to or shares a link with. A thread
   * document takes only its own thread's state.
   */
  threadStates(doc) {
    const stubs = new Set(this.threadsOf(doc));
    if (!THREAD_TYPES.includes(this.type(doc.path))) {
      const near = [...doc.links, ...this.backlinks.get(doc.path) ?? []];
      for (const p of near) {
        const other = this.byPath.get(p);
        if (other && THREAD_TYPES.includes(this.type(p))) this.threadsOf(other).forEach((s) => stubs.add(s));
      }
    }
    const states = /* @__PURE__ */ new Set();
    for (const s of stubs) {
      if (this.type(s) !== "stub") continue;
      states.add(this.byPath.get(s)?.fields.stage === "closed" ? "closed" : "open");
    }
    return states;
  }
};
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
function basename(path) {
  return path.slice(path.lastIndexOf("/") + 1).replace(/\.md$/, "");
}
function pathQuery(paths) {
  const alternatives = paths.map((p) => p.replace(/[.*+?^${}()|[\]\\]/g, "\\$&").replace(/\//g, "\\/"));
  return `path:/^(?:${alternatives.join("|")})$/`;
}
function colorGroups(groups) {
  return groups.map((g) => ({
    query: pathQuery(g.paths),
    color: { a: 1, rgb: parseInt(g.color.slice(1), 16) }
  }));
}
function isAtlasQuery(query) {
  return query.startsWith("path:/^(?:") && query.endsWith(")$/");
}
function mergeColorGroups(current, ours) {
  return [...ours, ...current.filter((g) => !isAtlasQuery(g.query))];
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
var GraphColors = class extends import_obsidian4.Component {
  constructor(host) {
    super();
    this.host = host;
  }
  host;
  groups = [];
  refresh = (0, import_obsidian4.debounce)(() => this.apply(), 1e3, true);
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
  async setMode(mode) {
    this.host.settings.graphColors = mode;
    await this.host.saveSettings();
    this.apply();
  }
  apply() {
    const mode = this.host.settings.graphColors;
    const theme = document.body.hasClass("theme-dark") ? "dark" : "light";
    const cache = this.app.metadataCache;
    this.groups = mode === "off" ? [] : graphGroups(mode, this.docs(), (link, from) => cache.getFirstLinkpathDest(link, from)?.path ?? null, theme);
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
    const merged = mergeColorGroups(instance.options.colorGroups ?? [], ours);
    if (!same(instance.options.colorGroups ?? [], merged)) {
      instance.options.colorGroups = merged;
      instance.saveOptions();
    }
    for (const type of ["graph", "localgraph"]) {
      for (const leaf of this.app.workspace.getLeavesOfType(type)) {
        const engine = engineOf(leaf.view);
        if (!engine) continue;
        const current = engine.getOptions().colorGroups ?? [];
        const next = mergeColorGroups(current, ours);
        if (!same(current, next)) engine.setOptions({ colorGroups: next });
      }
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

// src/sessions.ts
var import_obsidian5 = require("obsidian");
var import_os2 = require("os");
var SESSIONS_VIEW = "atlas-sessions";
function activeSessions(app) {
  const out = [];
  for (const file of app.vault.getMarkdownFiles()) {
    if (!file.path.startsWith("sessions/")) continue;
    const fm = app.metadataCache.getFileCache(file)?.frontmatter;
    if (!fm || fm.type !== "session") continue;
    if (fm.status !== "running" && fm.status !== "waiting") continue;
    const threads = asList(fm.threads);
    out.push({
      file,
      status: fm.status,
      description: typeof fm.description === "string" && fm.description.trim() ? fm.description : file.basename,
      thread: linkTitle(threads[threads.length - 1]),
      tasks: asList(fm.tasks),
      updated: String(fm.updated ?? ""),
      harness: String(fm.harness ?? "claude"),
      harness_id: String(fm.harness_id ?? ""),
      cwd: String(fm.cwd ?? "")
    });
  }
  return out.sort(compareSessions);
}
function currentTask(app, s) {
  let fallback = null;
  for (let i = s.tasks.length - 1; i >= 0; i--) {
    const title = linkTitle(s.tasks[i]);
    if (!title) continue;
    const file = app.metadataCache.getFirstLinkpathDest(title, s.file.path);
    const entry = { title, file };
    fallback ??= entry;
    if (file && app.metadataCache.getFileCache(file)?.frontmatter?.status === "open") return entry;
  }
  return fallback;
}
async function resume(s) {
  const command = resumeCommand(s, (0, import_os2.homedir)());
  if (!command) {
    new import_obsidian5.Notice("Atlas: this session has no harness id to resume.");
    return;
  }
  if (process.platform !== "darwin") {
    await navigator.clipboard.writeText(command);
    new import_obsidian5.Notice("Atlas: copied the resume command. Run it in a terminal.");
    return;
  }
  try {
    await runProgram("osascript", terminalArgs(command));
  } catch (e) {
    new import_obsidian5.Notice(`Atlas: cannot open Terminal: ${e.message}`);
  }
}
var SessionsView = class extends import_obsidian5.ItemView {
  generation = 0;
  constructor(leaf) {
    super(leaf);
  }
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
    this.render();
  }
  /** Rewrites the "ago" times only. */
  tick() {
    const now = /* @__PURE__ */ new Date();
    this.contentEl.querySelectorAll(".atlas-session-ago").forEach((el) => {
      el.setText(formatAgo(el.dataset.updated, now));
    });
  }
  render() {
    const generation = ++this.generation;
    const root = this.contentEl;
    root.empty();
    root.addClass("atlas-sessions");
    const sessions = activeSessions(this.app);
    if (sessions.length === 0) {
      root.createDiv({ cls: "atlas-sessions-empty", text: "No session is running or waiting." });
      return;
    }
    const now = /* @__PURE__ */ new Date();
    for (const s of sessions) {
      const card = root.createDiv({ cls: "atlas-session" });
      card.dataset.status = s.status;
      card.onclick = () => void this.app.workspace.getLeaf(false).openFile(s.file);
      const head = card.createDiv({ cls: "atlas-session-head" });
      head.createSpan({ cls: "atlas-session-status", text: s.status });
      head.createSpan({ cls: "atlas-session-title", text: s.description });
      const task = currentTask(this.app, s);
      const where = [s.thread, task?.title].filter((t) => t).join(" \xB7 ");
      if (where) card.createDiv({ cls: "atlas-session-where", text: where });
      const progress = card.createDiv({ cls: "atlas-session-progress" });
      if (task?.file) {
        void this.app.vault.cachedRead(task.file).then((text) => {
          if (generation === this.generation) progress.setText(lastProgressLine(text));
        });
      }
      const foot = card.createDiv({ cls: "atlas-session-foot" });
      const ago = foot.createSpan({ cls: "atlas-session-ago", text: formatAgo(s.updated, now) });
      ago.dataset.updated = s.updated;
      const button = foot.createEl("button", { text: "Resume" });
      button.onclick = (e) => {
        e.stopPropagation();
        void resume(s);
      };
    }
  }
};

// src/settings.ts
var import_obsidian6 = require("obsidian");
var DEFAULT_SETTINGS = {
  binaryPath: "",
  syncOnChange: true,
  badges: true,
  folderPages: true,
  graphColors: "area"
};
var AtlasSettingTab = class extends import_obsidian6.PluginSettingTab {
  constructor(app, plugin) {
    super(app, plugin);
    this.plugin = plugin;
  }
  plugin;
  display() {
    const { containerEl } = this;
    containerEl.empty();
    const found = findBinary("");
    const binary = new import_obsidian6.Setting(containerEl).setName("Path to the atlas-obsidian binary").setDesc("Leave empty to use the binary Atlas finds.").addText(
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
    new import_obsidian6.Setting(containerEl).setName("Sync when a thread document changes or a wiki page moves").setDesc("Runs atlas-obsidian vault sync after you edit a file under threads/ or move a file in wiki/, so the board, the callouts, and each page's scope follow.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.syncOnChange).onChange(async (value) => {
        this.plugin.settings.syncOnChange = value;
        await this.plugin.saveSettings();
      })
    );
    new import_obsidian6.Setting(containerEl).setName("Badges in the file explorer").setDesc("Shows the stage of each stub and the status of each session.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.badges).onChange(async (value) => {
        this.plugin.settings.badges = value;
        await this.plugin.saveSettings();
        this.plugin.badges.setEnabled(value);
      })
    );
    new import_obsidian6.Setting(containerEl).setName("Open a scope folder's page from the folder").setDesc("In the file explorer, a click on an area's or a repository's folder opens its page, and the page itself is hidden inside the folder.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.folderPages).onChange(async (value) => {
        this.plugin.settings.folderPages = value;
        await this.plugin.saveSettings();
        this.plugin.scopeFolders.setEnabled(value);
      })
    );
    new import_obsidian6.Setting(containerEl).setName("Graph colors").setDesc("Colors the nodes of the graph by area, by type, by the state of their threads, or by how recently they changed. The graph view has the same buttons.").addDropdown((dropdown) => {
      for (const { mode, label } of GRAPH_MODES) dropdown.addOption(mode, label);
      dropdown.setValue(this.plugin.settings.graphColors).onChange((value) => void this.plugin.graphColors.setMode(value));
    });
  }
};

// src/main.ts
var SYNC_DELAY = 1500;
var ECHO_WINDOW = 5e3;
var AtlasPlugin = class extends import_obsidian7.Plugin {
  settings = { ...DEFAULT_SETTINGS };
  badges;
  scopeFolders;
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
    this.scopeFolders = this.addChild(
      new ScopeFolders(this.app, () => {
        if (this.settings.syncOnChange) this.scheduleSync();
      })
    );
    this.scopeFolders.setEnabled(this.settings.folderPages);
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
    this.registerView(SESSIONS_VIEW, (leaf) => new SessionsView(leaf));
    this.sessionsRibbon = this.addRibbonIcon("bot", "Atlas: open the sessions", () => void this.openSessions());
    this.sessionsRibbon.addClass("atlas-sessions-ribbon");
    this.addCommand({ id: "open-sessions", name: "Open the sessions", callback: () => void this.openSessions() });
    this.statusItem = this.addStatusBarItem();
    this.statusItem.addClass("atlas-status-waiting");
    this.statusItem.onClickEvent(() => void this.openSessions());
    this.registerEditorExtension(mentionEditor);
    this.registerMarkdownPostProcessor(mentionReading);
    this.registerEvent(
      this.app.metadataCache.on("changed", (file) => {
        if (file.path.startsWith("sessions/")) this.refreshSessions();
        this.onThreadChange(file);
      })
    );
    this.registerEvent(this.app.vault.on("delete", () => this.refreshSessions()));
    this.registerEvent(this.app.vault.on("rename", () => this.refreshSessions()));
    this.registerInterval(window.setInterval(() => this.sessionViews().forEach((v) => v.tick()), 3e4));
    this.app.workspace.onLayoutReady(() => this.refreshSessions());
    const first = this.app.metadataCache.on("resolved", () => {
      this.app.metadataCache.offref(first);
      this.refreshSessions();
    });
    this.registerEvent(first);
  }
  onunload() {
    if (this.syncTimer !== null) window.clearTimeout(this.syncTimer);
  }
  async loadSettings() {
    this.settings = { ...DEFAULT_SETTINGS, ...await this.loadData() };
    if (!isGraphMode(this.settings.graphColors)) this.settings.graphColors = DEFAULT_SETTINGS.graphColors;
  }
  async saveSettings() {
    await this.saveData(this.settings);
  }
  /** Runs one atlas command in this vault and returns its JSON. */
  atlas(args) {
    const adapter = this.app.vault.adapter;
    if (!(adapter instanceof import_obsidian7.FileSystemAdapter)) {
      return Promise.reject(new AtlasError("this vault is not a folder on disk"));
    }
    return runAtlas(findBinary(this.settings.binaryPath), adapter.getBasePath(), args);
  }
  // Sync
  async sync(manual) {
    if (this.syncing) {
      if (manual) new import_obsidian7.Notice("Atlas: a sync is running.");
      return;
    }
    if (this.syncTimer !== null) window.clearTimeout(this.syncTimer);
    this.syncTimer = null;
    this.syncing = true;
    this.pending.clear();
    let wrote = [];
    try {
      const out = await this.atlas(["vault", "sync"]);
      wrote = syncedPaths(out.synced);
      this.lastAutoError = "";
      if (manual) new import_obsidian7.Notice(`Atlas: ${syncSummary(out.synced)}`);
    } catch (e) {
      const message = e.message;
      if (manual || message !== this.lastAutoError) new import_obsidian7.Notice(`Atlas: ${message}`);
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
  onThreadChange(file) {
    if (!this.settings.syncOnChange || !isThreadPath(file.path)) return;
    if (this.syncing) {
      this.pending.add(file.path);
      return;
    }
    if (Date.now() < this.echoUntil && this.echoes.has(file.path)) {
      this.echoes.delete(file.path);
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
  // Sessions
  sessionViews() {
    return this.app.workspace.getLeavesOfType(SESSIONS_VIEW).map((leaf) => leaf.view).filter((v) => v instanceof SessionsView);
  }
  refreshSessions = (0, import_obsidian7.debounce)(
    () => {
      const waiting = activeSessions(this.app).filter((s) => s.status === "waiting").length;
      this.statusItem?.setText(waiting > 0 ? waitingLabel(waiting) : "");
      this.statusItem?.toggleClass("is-hidden", waiting === 0);
      if (this.sessionsRibbon) {
        if (waiting > 0) this.sessionsRibbon.dataset.atlasCount = String(waiting);
        else delete this.sessionsRibbon.dataset.atlasCount;
      }
      this.sessionViews().forEach((v) => v.render());
    },
    500,
    true
  );
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
