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
var import_obsidian5 = require("obsidian");

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
  return [`${home}/.atlas/bin/atlas`, `${home}/go/bin/atlas`];
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
  if (s.settings) parts.push("the harness settings");
  if (parts.length === 0) return "Nothing to heal.";
  return `Synced ${parts.join(", ")}.`;
}
function syncedPaths(s) {
  return [...s.threads ?? [], ...s.lost ?? [], ...s.sessions ?? []];
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
    throw new AtlasError(`atlas ${args[0]} did not print JSON`);
  }
}
async function binaryVersion(bin) {
  return (await exec(bin, ["version"], void 0)).trim();
}
function runProgram(bin, args) {
  return exec(bin, args, void 0);
}

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
var import_obsidian3 = require("obsidian");
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
    new import_obsidian3.Notice("Atlas: this session has no harness id to resume.");
    return;
  }
  if (process.platform !== "darwin") {
    await navigator.clipboard.writeText(command);
    new import_obsidian3.Notice("Atlas: copied the resume command. Run it in a terminal.");
    return;
  }
  try {
    await runProgram("osascript", terminalArgs(command));
  } catch (e) {
    new import_obsidian3.Notice(`Atlas: cannot open Terminal: ${e.message}`);
  }
}
var SessionsView = class extends import_obsidian3.ItemView {
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
var import_obsidian4 = require("obsidian");
var DEFAULT_SETTINGS = {
  binaryPath: "",
  syncOnChange: true,
  badges: true
};
var AtlasSettingTab = class extends import_obsidian4.PluginSettingTab {
  constructor(app, plugin) {
    super(app, plugin);
    this.plugin = plugin;
  }
  plugin;
  display() {
    const { containerEl } = this;
    containerEl.empty();
    const found = findBinary("");
    const binary = new import_obsidian4.Setting(containerEl).setName("Path to the atlas binary").setDesc("Leave empty to use the binary Atlas finds.").addText(
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
    new import_obsidian4.Setting(containerEl).setName("Sync when a thread document changes").setDesc("Runs atlas vault sync after you edit a file under threads/, so the board and the callouts follow.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.syncOnChange).onChange(async (value) => {
        this.plugin.settings.syncOnChange = value;
        await this.plugin.saveSettings();
      })
    );
    new import_obsidian4.Setting(containerEl).setName("Badges in the file explorer").setDesc("Shows the stage of each stub and the status of each session.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.badges).onChange(async (value) => {
        this.plugin.settings.badges = value;
        await this.plugin.saveSettings();
        this.plugin.badges.setEnabled(value);
      })
    );
  }
};

// src/main.ts
var SYNC_DELAY = 1500;
var ECHO_WINDOW = 5e3;
var AtlasPlugin = class extends import_obsidian5.Plugin {
  settings = { ...DEFAULT_SETTINGS };
  badges;
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
  }
  async saveSettings() {
    await this.saveData(this.settings);
  }
  /** Runs one atlas command in this vault and returns its JSON. */
  atlas(args) {
    const adapter = this.app.vault.adapter;
    if (!(adapter instanceof import_obsidian5.FileSystemAdapter)) {
      return Promise.reject(new AtlasError("this vault is not a folder on disk"));
    }
    return runAtlas(findBinary(this.settings.binaryPath), adapter.getBasePath(), args);
  }
  // Sync
  async sync(manual) {
    if (this.syncing) {
      if (manual) new import_obsidian5.Notice("Atlas: a sync is running.");
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
      if (manual) new import_obsidian5.Notice(`Atlas: ${syncSummary(out.synced)}`);
    } catch (e) {
      const message = e.message;
      if (manual || message !== this.lastAutoError) new import_obsidian5.Notice(`Atlas: ${message}`);
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
  refreshSessions = (0, import_obsidian5.debounce)(
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
