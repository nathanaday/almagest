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
var import_obsidian10 = require("obsidian");

// src/change.ts
var import_obsidian = require("obsidian");

// src/helpers.ts
var DOCUMENTS = "source-core/documents/";
var WIKI_VIEW = "wiki-view/";
var NAV_FOLDER = "wiki-view/nav/";
var INGEST = "ingest/";
function isDocumentPath(path) {
  return path.startsWith(DOCUMENTS);
}
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
  add(s.knowledge, "knowledge document", "knowledge documents");
  add(s.moved, "document moved back", "documents moved back");
  add(s.lost, "lost session", "lost sessions");
  add(s.sessions, "session callout", "session callouts");
  if (s.settings) parts.push("the harness settings");
  if (s.views) parts.push(plural(s.views, "view", "views"));
  add(s.skipped, "document left as saved", "documents left as saved");
  if (parts.length === 0) return "Generated files are up to date.";
  return `Synced ${parts.join(", ")}.`;
}
function syncedPaths(s) {
  const strays = (s.strays ?? []).flatMap((m) => [m.from, m.to]);
  return [...s.knowledge ?? [], ...s.moved ?? [], ...s.lost ?? [], ...s.sessions ?? [], ...strays];
}
function strayNotices(s) {
  return (s.strays ?? []).map(movedLine);
}
function movedNotices(out) {
  if (typeof out !== "object" || out === null) return [];
  const moved = out.moved_from_wiki_view;
  if (!Array.isArray(moved)) return [];
  return moved.filter((m) => typeof m?.from === "string" && typeof m?.to === "string").map(movedLine);
}
function movedLine(m) {
  return `Moved ${m.from} to ${m.to}: code writes every file in ${WIKI_VIEW}, so your note waits in ${INGEST}.`;
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
var DOCUMENT_TYPES = ["source", "repository", "topic"];
function isDocumentType(type) {
  return DOCUMENT_TYPES.includes(String(type));
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
function isWatchedPath(path) {
  return path.endsWith(".md") && !path.startsWith(WIKI_VIEW) && !path.startsWith(".");
}
function isSnapshotPath(path, configDir) {
  return path !== "" && !path.startsWith(configDir + "/") && !path.startsWith(WIKI_VIEW);
}
var SNAPSHOT_QUIET_DEFAULT = 120;
var SNAPSHOT_QUIET_MAX = 86400;
function quietSeconds(value) {
  const raw = typeof value === "string" ? value.trim() : value;
  if (raw === "" || raw === null || raw === void 0 || typeof raw === "boolean") return SNAPSHOT_QUIET_DEFAULT;
  const n = Number(raw);
  if (!Number.isFinite(n)) return SNAPSHOT_QUIET_DEFAULT;
  return Math.min(SNAPSHOT_QUIET_MAX, Math.max(0, Math.round(n)));
}
function isLockHeld(message) {
  return /atlas\.lock|holds the lock/.test(message);
}
function tagTitle(tag) {
  return "Tag \xB7 " + tag.split("/").join(" \u203A ");
}
function tagViewPath(tag) {
  return `${NAV_FOLDER}${tag}/${tagTitle(tag)}.md`;
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
var NAV_GROUPS = [
  { name: "Topics", test: (d) => d.type === "topic" },
  { name: "Sources", test: (d) => d.type === "source" },
  { name: "Repositories", test: (d) => d.type === "repository" }
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
var LAYOUT = 6;
var MIGRATES_FROM = 4;
function migrationSteps(layout) {
  const steps = [];
  if (layout === 4) {
    steps.push("Moves the thread documents of 8.x (stubs, specs, task lists, verifications, chords, and events) and the chord canvases to threads/, an archive Atlas does not read.");
  }
  steps.push(
    "Moves wiki/documents/ to source-core/documents/.",
    "Moves wiki/assets/ to source-core/originals/, and any other file of wiki/ to source-core/.",
    "Moves inbox/ to ingest/.",
    "Removes views/ and writes the views again in wiki-view/, with the tag views in wiki-view/nav/. A note of yours in views/ goes to ingest/.",
    "Rewrites each link, embed, and Base that names one of these folders. Prose that names a folder stays as you wrote it.",
    "Sets origin: ingest on each source that came from the inbox.",
    "Sends new attachments to source-core/originals/ and keeps wiki-view/ out of Obsidian's search, unless you chose other settings."
  );
  return steps;
}
function migrationSummary(r) {
  const parts = [`Migrated to the ${layoutName(LAYOUT)} layout in one commit${r.commit ? `, ${r.commit.slice(0, 7)}` : ""}.`];
  const moved = r.moved?.length ?? 0;
  const edited = r.edited?.length ?? 0;
  if (moved || edited) parts.push(`${plural(moved, "file", "files")} moved, ${edited} edited.`);
  if (r.problems) parts.push(`Lint finds ${plural(r.problems, "error", "errors")}.`);
  if (r.plugin) parts.push(`The Obsidian plugin is now ${r.plugin}; reload Obsidian to use it.`);
  return parts.join(" ");
}
function layoutName(layout) {
  if (layout >= LAYOUT) return "10.0";
  if (layout === 5) return "9.0";
  if (layout === 4) return "8.x";
  return layout === 3 ? "7.x" : "6.x";
}

// src/changestate.ts
var RESULT = {
  applying: () => "Being applied. If this stays, the apply stopped; the next Atlas write puts the documents back and sets the change to proposed.",
  applied: (fm) => {
    const when = stampText(fm.applied);
    return when ? `Applied ${when}.` : "Applied.";
  },
  rejected: (fm) => {
    const reason = typeof fm.reason === "string" ? fm.reason.trim() : "";
    return reason ? `Rejected: ${reason}` : "Rejected.";
  },
  superseded: () => "A later change replaced this one.",
  undone: () => "Undone. The documents are back as they were before it."
};
function changeCard(fm, busy = null, progress = "") {
  if (!fm || fm.type !== "change") {
    return { state: "none", label: "Change", line: "This block shows a change. This note is not a change document.", counts: "", kind: "", id: "", buttons: [] };
  }
  const id = typeof fm.id === "string" ? fm.id.trim() : "";
  const status2 = typeof fm.status === "string" ? fm.status.trim() : "";
  const counts = countsLine(fm.counts);
  const card = { id, counts, kind: workKind(fm) };
  if (busy === "apply") return { ...card, state: "busy", label: "Applying", line: "Atlas applies this change.", buttons: [] };
  if (busy === "reject") return { ...card, state: "busy", label: "Cancelling", line: "Atlas rejects this change.", buttons: [] };
  if (status2 === "proposed") {
    if (!id) return { ...card, state: "proposed", label: "Proposed", line: "This change has no id, so it cannot be applied from here.", buttons: [] };
    return {
      ...card,
      state: "proposed",
      label: "Proposed",
      line: "Review the writes below. Approve applies them in one commit. Cancel rejects the change, and this document stays as the record.",
      buttons: ["approve", "cancel"]
    };
  }
  if (status2 === "running") {
    const line = progress.trim() || "The agent starts. Its steps appear here and under Progress.";
    return { ...card, state: "running", label: "Running", line, buttons: id ? ["cancel"] : [] };
  }
  const result = RESULT[status2];
  if (result) return { ...card, state: status2, label: capital(status2), line: result(fm), buttons: [] };
  return { ...card, state: "other", label: status2 ? capital(status2) : "Change", line: status2 ? `Status: ${status2}.` : "This change has no status.", buttons: [] };
}
function workKind(fm) {
  const kind = typeof fm.kind === "string" ? fm.kind.trim() : "";
  if (!kind) return "";
  const files = Array.isArray(fm.files) ? fm.files.length : 0;
  return files > 0 ? `${kind} \xB7 ${files} ${files === 1 ? "file" : "files"}` : kind;
}
function rejectReason(input) {
  const line = input.replace(/\s+/g, " ").trim();
  return line || "cancelled in Obsidian";
}
function stampText(value) {
  const m = /^(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2})/.exec(String(value ?? ""));
  return m ? `${m[1]} ${m[2]}` : "";
}
function capital(s) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

// src/change.ts
var ChangeRunner = class {
  constructor(plugin) {
    this.plugin = plugin;
  }
  plugin;
  current = null;
  listeners = /* @__PURE__ */ new Set();
  /** The command that runs for this change, if one does. */
  actionFor(id) {
    return this.current?.id === id ? this.current.action : null;
  }
  get busy() {
    return this.current !== null;
  }
  /** Calls fn when a command starts or ends; returns the call that stops it. */
  listen(fn) {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }
  async apply(id, sourcePath) {
    await this.run(id, "apply", sourcePath, async () => {
      await saveOpen(this.plugin.app, sourcePath);
      const p = await this.plugin.atlas(["change", "apply", id]);
      const counts = countsLine(p.counts);
      const commit = p.commit ? ` Commit ${p.commit.slice(0, 7)}.` : "";
      new import_obsidian.Notice(`Atlas: applied ${p.ref?.title ?? id}${counts ? `: ${counts}` : ""}.${commit}`);
      warn(p.warnings);
    });
  }
  async reject(id, reason, sourcePath) {
    await this.run(id, "reject", sourcePath, async () => {
      const p = await this.plugin.atlas(["change", "reject", id, "--reason", rejectReason(reason)]);
      new import_obsidian.Notice(`Atlas: rejected ${p.ref?.title ?? id}.`);
      warn(p.warnings);
    });
  }
  /**
   * Runs one command. The widget stays busy until Obsidian reads the document's new
   * status, so it never offers Approve again for a change that was just applied.
   */
  async run(id, action, sourcePath, fn) {
    if (this.current) {
      new import_obsidian.Notice("Atlas: a change command runs. Wait for it to finish.");
      return;
    }
    this.current = { id, action };
    this.emit();
    const decided = decision(this.plugin.app, sourcePath, SETTLE_MS);
    try {
      await fn();
      await decided.done;
    } catch (e) {
      new import_obsidian.Notice(`Atlas: ${e.message}`, 1e4);
    } finally {
      decided.stop();
      this.current = null;
      this.emit();
    }
  }
  emit() {
    for (const fn of this.listeners) fn();
  }
};
var SETTLE_MS = 5e3;
function decision(app, path, ms) {
  let stop = () => {
  };
  const done = new Promise((resolve) => {
    const ref = app.metadataCache.on("changed", (file, _data, cache) => {
      const status2 = cache.frontmatter?.status;
      if (file.path === path && !["proposed", "applying", "running"].includes(status2)) finish();
    });
    const timer = window.setTimeout(() => finish(), ms);
    function finish() {
      app.metadataCache.offref(ref);
      window.clearTimeout(timer);
      resolve();
    }
    stop = finish;
  });
  return { done, stop };
}
async function saveOpen(app, path) {
  for (const leaf of app.workspace.getLeavesOfType("markdown")) {
    const view = leaf.view;
    if (view instanceof import_obsidian.MarkdownView && view.file?.path === path) await view.save();
  }
}
function warn(warnings) {
  for (const w of warnings ?? []) new import_obsidian.Notice(`Atlas: ${w}`, 1e4);
}
var ChangeWidget = class extends import_obsidian.MarkdownRenderChild {
  constructor(containerEl, plugin, runner, path) {
    super(containerEl);
    this.plugin = plugin;
    this.runner = runner;
    this.path = path;
  }
  plugin;
  runner;
  path;
  generation = 0;
  onload() {
    const { metadataCache, vault } = this.plugin.app;
    this.registerEvent(
      metadataCache.on("changed", (file) => {
        if (file.path === this.path) void this.render();
      })
    );
    this.registerEvent(
      vault.on("rename", (file, oldPath) => {
        if (oldPath === this.path) this.path = file.path;
      })
    );
    this.register(this.runner.listen(() => void this.render()));
    void this.render();
  }
  frontmatter() {
    const file = this.plugin.app.vault.getFileByPath(this.path);
    return file ? this.plugin.app.metadataCache.getFileCache(file)?.frontmatter : void 0;
  }
  /** The last line of a running change's Progress section, read from the file: the cache holds no body text. */
  async progress(fm) {
    const file = fm?.status === "running" ? this.plugin.app.vault.getFileByPath(this.path) : null;
    return file ? lastProgressLine(await this.plugin.app.vault.read(file)) : "";
  }
  async render() {
    const generation = ++this.generation;
    const fm = this.frontmatter();
    const progress = await this.progress(fm).catch(() => "");
    if (generation !== this.generation) return;
    const card = changeCard(fm, this.runner.actionFor(typeof fm?.id === "string" ? fm.id : ""), progress);
    const el = this.containerEl;
    el.empty();
    el.addClass("atlas-change-card");
    el.dataset.state = card.state;
    const head = el.createDiv({ cls: "atlas-change-head" });
    head.createSpan({ cls: "atlas-change-label", text: card.label });
    if (card.kind) head.createSpan({ cls: "atlas-change-kind", text: card.kind });
    if (card.counts) head.createSpan({ cls: "atlas-change-counts", text: card.counts });
    el.createDiv({ cls: "atlas-change-line", text: card.line });
    if (card.buttons.length === 0) return;
    const buttons = el.createDiv({ cls: "atlas-change-buttons" });
    const running = card.state === "running";
    for (const b of card.buttons) {
      const button = buttons.createEl("button", { cls: b === "approve" ? "mod-cta" : "", text: b === "approve" ? "Approve" : "Cancel" });
      if (this.runner.busy) {
        button.disabled = true;
        button.setAttr("title", "Another change command runs.");
      }
      button.onclick = b === "approve" ? () => void this.runner.apply(card.id, this.path) : () => new CancelModal(this.plugin.app, running, (reason) => void this.runner.reject(card.id, reason, this.path)).open();
    }
  }
};
function changeProcessor(plugin, runner) {
  return (_source, el, ctx) => {
    ctx.addChild(new ChangeWidget(el, plugin, runner, ctx.sourcePath));
  };
}
var CancelModal = class extends import_obsidian.Modal {
  constructor(app, running, done) {
    super(app);
    this.running = running;
    this.done = done;
  }
  running;
  done;
  reason = "";
  onOpen() {
    this.setTitle("Cancel this change");
    this.contentEl.createEl("p", {
      text: this.running ? "Atlas rejects the work. The agent stops when it reports its next step, and this document stays as the record." : "Atlas rejects the change. Nothing it would write changes, and its document stays as the record."
    });
    const submit = () => {
      this.close();
      this.done(this.reason);
    };
    new import_obsidian.Setting(this.contentEl).setName("Reason").setDesc("Optional. One line.").addText((text) => {
      text.setPlaceholder("Why not").onChange((v) => this.reason = v);
      text.inputEl.addClass("atlas-reason-input");
      text.inputEl.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.isComposing) {
          e.preventDefault();
          submit();
        }
      });
      window.setTimeout(() => text.inputEl.focus(), 0);
    });
    new import_obsidian.Setting(this.contentEl).addButton((b) => b.setButtonText("Back").onClick(() => this.close())).addButton((b) => b.setButtonText("Cancel the change").setWarning().onClick(submit));
  }
  onClose() {
    this.contentEl.empty();
  }
};

// src/cli.ts
var import_child_process = require("child_process");
var import_fs = require("fs");
var import_os = require("os");
var import_obsidian2 = require("obsidian");
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
function exec(bin, args, cwd, answers = []) {
  return new Promise((resolve, reject) => {
    (0, import_child_process.execFile)(
      bin,
      args,
      { cwd, env: childEnv(), maxBuffer: 32 * 1024 * 1024, timeout: 5 * 60 * 1e3 },
      (err, stdout, stderr) => {
        if (!err) return resolve(stdout);
        const code = err.code;
        if (typeof code === "number" && answers.includes(code)) return resolve(stdout);
        if (code === "ENOENT") {
          return reject(new AtlasError(`the atlas-obsidian binary was not found at ${bin}`));
        }
        reject(new AtlasError(errorMessage(String(stderr)) || err.message));
      }
    );
  });
}
async function runAtlas(bin, vault, args, answers = []) {
  if (!bin) throw new AtlasError("the atlas-obsidian binary was not found; set its path in the Atlas settings");
  const out = await exec(bin, [...args, "--vault", vault, "--json"], vault, answers);
  let parsed;
  try {
    parsed = JSON.parse(out);
  } catch {
    throw new AtlasError(`atlas-obsidian ${args[0]} did not print JSON`);
  }
  for (const line of movedNotices(parsed)) new import_obsidian2.Notice(`Atlas: ${line}`, 0);
  return parsed;
}
async function binaryVersion(bin) {
  return (await exec(bin, ["version"], void 0)).trim();
}

// src/quiet.ts
var QuietTimer = class {
  constructor(clock, task, quietMs) {
    this.clock = clock;
    this.task = task;
    this.quietMs = quietMs;
  }
  clock;
  task;
  quietMs;
  handle = null;
  running = false;
  /** The quiet period ended while the task ran. */
  again = false;
  /** Starts the quiet period again. */
  touch() {
    if (this.quietMs <= 0) return;
    this.arm();
  }
  /** Sets the quiet period; a waiting timer starts again with it. */
  setQuiet(ms) {
    this.quietMs = ms;
    if (ms <= 0) {
      this.cancel();
      this.again = false;
    } else if (this.handle !== null) {
      this.arm();
    }
  }
  /** Stops the timer; a task that runs ends as it would. */
  stop() {
    this.quietMs = 0;
    this.again = false;
    this.cancel();
  }
  get waiting() {
    return this.handle !== null;
  }
  get busy() {
    return this.running;
  }
  arm() {
    this.cancel();
    this.handle = this.clock.set(() => {
      this.handle = null;
      void this.fire();
    }, this.quietMs);
  }
  cancel() {
    if (this.handle !== null) this.clock.clear(this.handle);
    this.handle = null;
  }
  async fire() {
    if (this.running) {
      this.again = true;
      return;
    }
    this.running = true;
    this.again = false;
    let retry = false;
    try {
      retry = await this.task();
    } catch {
      retry = false;
    } finally {
      this.running = false;
    }
    if ((retry || this.again) && this.quietMs > 0 && this.handle === null) this.arm();
    this.again = false;
  }
};

// src/repo.ts
var import_obsidian3 = require("obsidian");
var RepoPanel = class extends import_obsidian3.MarkdownRenderChild {
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
    if (!isDocumentPath(f.path)) continue;
    const other = cache.getFileCache(f)?.frontmatter;
    if (other?.id === id) return other;
  }
  return void 0;
}

// src/sessions.ts
var import_obsidian4 = require("obsidian");
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
function startCommand(dir, agent, prompt = "") {
  const first = prompt.replace(/[\x00-\x1f\x7f]+/g, " ").trim();
  return `cd ${shellQuote(dir)} && ${agent.trim() || "claude"}${first ? ` ${shellQuote(first)}` : ""}`;
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
    out.push({
      file,
      path: file.path,
      status: String(fm.status ?? ""),
      updated: String(fm.updated ?? ""),
      ended: String(fm.ended ?? ""),
      pid: Number(fm.pid ?? 0) || 0,
      parent: String(fm.parent ?? ""),
      description: typeof fm.description === "string" && fm.description.trim() ? fm.description : file.basename,
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
async function resume(plugin, s) {
  const id = s.harness_id.trim();
  if (!id || s.parent) {
    new import_obsidian4.Notice("Atlas: this session has no conversation of its own to resume.");
    return;
  }
  let target = { harness: s.harness, id, cwd: expandHome(s.cwd, (0, import_os3.homedir)()), configDir: "" };
  if (s.harness !== "codex") {
    const transcript = findTranscript(s.transcript, id);
    if (!transcript) {
      new import_obsidian4.Notice("Atlas: Claude Code has no saved conversation for this session, so it cannot resume.", 8e3);
      return;
    }
    const place = resumePlace(transcript);
    target = { ...target, cwd: place.cwd || target.cwd, configDir: place.configDir };
  }
  await plugin.runInTerminal(resumeCommand(target), "the resume command");
}
var STATE_CLASS = { "needs you": "waiting", working: "working", idle: "idle", ended: "ended", lost: "ended" };
var SessionsView = class extends import_obsidian4.ItemView {
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
    const top = card.createDiv({ cls: "atlas-session-top" });
    top.createSpan({ cls: "atlas-session-status", text: state });
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
    void this.app.vault.cachedRead(s.file).then((text) => {
      const last = lastProgressLine(text);
      if (generation === this.generation && last) card.setAttr("title", plainLinks(last));
    });
  }
};

// src/conversations.ts
function duetApi(app) {
  const plugins = app?.plugins;
  const api = plugins?.getPlugin?.("duet")?.api;
  if (!api || typeof api.version !== "number" || api.version < 1) return void 0;
  if (typeof api.newConversation !== "function" || typeof api.conversationStatus !== "function" || typeof api.onTurnEnd !== "function") return void 0;
  return api;
}
var Conversations = class {
  constructor(changed, ended = () => {
  }) {
    this.changed = changed;
    this.ended = ended;
  }
  changed;
  ended;
  followed = [];
  /**
   * Lists a conversation until its turn ends. It subscribes before it reads the status, so
   * a turn that ends in between is not missed: the status then says the turn is over.
   */
  follow(api, path, label) {
    const item = { conversation: { path, label }, stop: () => {
    } };
    item.stop = api.onTurnEnd(path, (turn) => {
      item.conversation.path = turn.path;
      if (this.drop(item)) this.ended(item.conversation, turn);
    });
    if (status(api, path) !== "working") {
      item.stop();
      return;
    }
    this.followed.push(item);
    this.changed();
  }
  /** Drops each conversation that no longer works: ended, gone, or with Duet off. */
  check(api) {
    for (const item of [...this.followed]) {
      if (!api || status(api, item.conversation.path) !== "working") this.drop(item);
    }
  }
  list() {
    return this.followed.map((f) => ({ ...f.conversation }));
  }
  stop() {
    for (const f of this.followed) f.stop();
    this.followed = [];
  }
  drop(item) {
    const i = this.followed.indexOf(item);
    if (i < 0) return false;
    this.followed.splice(i, 1);
    item.stop();
    this.changed();
    return true;
  }
};
function status(api, path) {
  try {
    return api.conversationStatus(path);
  } catch {
    return "none";
  }
}

// src/journalstate.ts
var JOURNALS = "journals";
var MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];
var SEPARATORS = /[-_\t\n\v\f\r\u0085\p{Zs}\u2028\u2029]+/u;
function journalName(folder) {
  return folder.split(SEPARATORS).filter((w) => w !== "").map((w) => /\p{L}/u.test(w) && /\p{Nd}/u.test(w) ? w.toUpperCase() : capital2(w)).join(" ");
}
function capital2(word) {
  const first = word.codePointAt(0);
  if (first === void 0) return word;
  const head = String.fromCodePoint(first);
  return head.toUpperCase() + word.slice(head.length);
}
function editionTitle(folder, day2) {
  return `User Journal ${journalName(folder)} - ${day2.getDate()} ${MONTHS[day2.getMonth()]} ${day2.getFullYear()} Edition`;
}
function journalVolumes(list) {
  return (list ?? []).filter((v) => typeof v.volume === "string" && v.volume !== "").map((v) => ({
    volume: v.volume,
    name: v.name || journalName(v.volume),
    notes: v.notes ?? 0,
    edition: v.edition ?? "",
    changed: v.changed === true
  }));
}
function toPublish(volumes) {
  return volumes.filter((v) => v.changed).length;
}
function publishBlocked(v) {
  if (v.notes === 0) return "The volume holds no note.";
  if (!v.changed) return v.edition ? `No change since ${v.edition}.` : "No change to publish.";
  return "";
}
function volumeOf(path) {
  const parts = path.split("/");
  return parts.length >= 3 && parts[0] === JOURNALS && parts[1] !== "" && !parts[1].startsWith(".") ? parts[1] : "";
}
function numbered(latest, title) {
  return latest === title || latest.startsWith(`${title} (`) && latest.endsWith(")");
}

// src/palette.ts
var import_obsidian7 = require("obsidian");

// src/checkout.ts
var import_obsidian5 = require("obsidian");

// src/checkoutstate.ts
var CHECKOUT = "checkout";
var READING_LIST = "Reading list";
function checkouts(list) {
  return (list ?? []).filter((c) => typeof c.folder === "string" && c.folder !== "").map((c) => ({
    folder: c.folder,
    request: c.request || folderName(c.folder),
    date: c.date ?? "",
    documents: c.documents ?? 0,
    edited: c.edited ?? 0,
    returned: c.returned ?? ""
  })).sort((a, b) => a.folder < b.folder ? 1 : a.folder > b.folder ? -1 : 0);
}
function toReturn(list) {
  return list.filter((c) => returnBlocked(c) === "").length;
}
function returnBlocked(c) {
  if (c.returned) return `Returned ${day(c.returned)}.`;
  if (c.edited === 0) return "No copy is edited.";
  return "";
}
function day(stamp) {
  return stamp.slice(0, 10);
}
function readingListPath(c) {
  return `${c.folder}/${READING_LIST}.md`;
}
function folderName(folder) {
  return folder.slice(folder.lastIndexOf("/") + 1);
}
function skippedLine(skipped) {
  const list = skipped ?? [];
  if (list.length === 0) return "";
  return `the return left out ${list.length === 1 ? "1 copy" : `${list.length} copies`}: ${list.join("; ")}.`;
}
function oneLine(request) {
  return request.trim().replace(/\s+/g, " ");
}
var TITLE_MAX = 60;
function checkoutTitle(request) {
  let text = oneLine(request);
  if (text.length > TITLE_MAX) {
    const cut = text.slice(0, TITLE_MAX);
    const space = cut.lastIndexOf(" ");
    text = `${(space > TITLE_MAX / 2 ? cut.slice(0, space) : cut).trimEnd()}\u2026`;
  }
  return `Agent \xB7 Check out ${text}`;
}

// src/messages.ts
var MAX_NAMED = 10;
function report(doc, what) {
  return `Your work document is [[${doc.title}]] (${doc.id}): report each step with change progress, and propose ${what} with change propose and id ${doc.id}.`;
}
function ingestMessage(doc) {
  return `/atlas-obsidian:wiki-ingest Ingest the files of ingest/ into the wiki. ${report(doc, "into it")}`;
}
function repairMessage(doc) {
  return `/atlas-obsidian:wiki-review Repair the lint findings that a change repairs. ${report(doc, "the repairs into it")}`;
}
function publishMessage(edition, doc) {
  return `/atlas-obsidian:wiki-sync Absorb the source [[${edition.title}]] (${edition.id}), the user's journal edition. Cite it where its ideas land. ${report(doc, "into it")}`;
}
function checkoutMessage(request) {
  return `/atlas-obsidian:wiki-checkout Check out the material on: ${oneLine(request)}`;
}
function resolveMessage(target, backlinks) {
  const names = backlinks.slice(0, MAX_NAMED).map((b) => `[[${b.title}]]`);
  const more = backlinks.length - names.length;
  if (more > 0) names.push(`${more} more`);
  return `/atlas-obsidian:wiki-edit Remove [[${target.title}]] (${target.path}), which ${names.join(", ")} ${backlinks.length === 1 ? "links" : "link"}: point each backlink elsewhere, or drop it, then propose a remove.`;
}

// src/checkout.ts
function startCheckout(plugin, request) {
  return plugin.runAgent(checkoutMessage(request), checkoutTitle(request), "checkout");
}
async function returnCheckout(plugin, c) {
  for (const leaf of plugin.app.workspace.getLeavesOfType("markdown")) {
    const view = leaf.view;
    if (view instanceof import_obsidian5.MarkdownView && view.file?.path.startsWith(`${c.folder}/`)) await view.save();
  }
  const { returned } = await plugin.atlas([CHECKOUT, "return", c.folder]);
  const line = skippedLine(returned.skipped);
  if (line) new import_obsidian5.Notice(`Atlas: ${line}`, 15e3);
  if (returned.change?.ref) await plugin.openWhenSeen(returned.change.ref.path, true);
}
var CheckoutModal = class extends import_obsidian5.Modal {
  constructor(app, start) {
    super(app);
    this.start = start;
  }
  start;
  request = "";
  onOpen() {
    this.setTitle("Check out material");
    const el = this.contentEl;
    el.addClass("atlas-checkout");
    el.createEl("p", {
      text: `The librarian finds the documents that serve your request and copies them into ${CHECKOUT}/, with a reading list. Edit the copies as you like. Return proposes your edits to the wiki as a change.`
    });
    let go;
    const submit = () => {
      const request = oneLine(this.request);
      if (!request) return;
      this.close();
      this.start(request);
    };
    new import_obsidian5.Setting(el).setName("Request").setDesc("One line: the subject, in your words.").addText((text) => {
      text.setPlaceholder("reinforcement learning").onChange((v) => {
        this.request = v;
        go?.setDisabled(oneLine(v) === "");
      });
      text.inputEl.addClass("atlas-checkout-input");
      text.inputEl.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.isComposing) {
          e.preventDefault();
          submit();
        }
      });
      window.setTimeout(() => text.inputEl.focus(), 0);
    });
    new import_obsidian5.Setting(el).addButton((b) => b.setButtonText("Cancel").onClick(() => this.close())).addButton((b) => {
      go = b.setButtonText("Check out").setCta().setDisabled(true).onClick(submit);
    });
  }
  onClose() {
    this.contentEl.empty();
  }
};

// src/palettestate.ts
function paletteState(status2, liveSessions) {
  const byTitle = (a, b) => a.title.localeCompare(b.title);
  const journals = journalVolumes(status2.journals);
  const list = checkouts(status2.checkouts);
  return {
    proposed: [...status2.changes?.proposed ?? []].sort(byTitle),
    running: [...status2.changes?.running ?? []].sort(byTitle),
    ingest: (status2.ingest ?? []).map((i) => i.name),
    pending: status2.pending?.length ?? 0,
    sessions: liveSessions,
    trash: status2.trash ?? 0,
    journals,
    toPublish: toPublish(journals),
    checkouts: list,
    toReturn: toReturn(list),
    problems: status2.problems ?? 0
  };
}
function plural2(n, one, many) {
  return `${n} ${n === 1 ? one : many}`;
}
function noteTitle(path) {
  const name = path.slice(path.lastIndexOf("/") + 1);
  return name.endsWith(".md") ? name.slice(0, -3) : name;
}
var SEVERITY = ["error", "warning", "info"];
function isRepairable(f) {
  return (f.severity === "error" || f.severity === "warning") && /\bwiki-edit\b/.test(f.fix);
}
function lintSummary(r, max = 8) {
  const findings = r.findings ?? [];
  const rank = (s) => SEVERITY.includes(s) ? SEVERITY.indexOf(s) : SEVERITY.length;
  const sorted = [...findings].sort((a, b) => rank(a.severity) - rank(b.severity));
  const n = (s) => r.counts?.[s] ?? findings.filter((f) => f.severity === s).length;
  const parts = [plural2(n("error"), "error", "errors"), plural2(n("warning"), "warning", "warnings"), `${n("info")} info`].filter((p) => !p.startsWith("0 "));
  const checked = r.checked ?? 0;
  return {
    counts: parts.length > 0 ? parts.join(", ") : `No findings in ${plural2(checked, "document", "documents")}.`,
    first: sorted.slice(0, max),
    more: Math.max(0, sorted.length - max),
    repairable: findings.filter(isRepairable).length
  };
}
function trashOutcome(r) {
  const backlinks = r.backlinks ?? [];
  if (backlinks.length > 0) return { kind: "linked", path: r.path, title: noteTitle(r.path), backlinks };
  if (!r.moved) return { kind: "error", line: `${r.path} did not move, and nothing links it.` };
  const through = r.change?.title ? ` The change ${r.change.title} records it.` : "";
  return { kind: "moved", line: `Moved ${r.path} to ${r.moved}.${through}` };
}

// src/publish.ts
var import_obsidian6 = require("obsidian");
function confirmPublish(plugin, vol) {
  const title = editionTitle(vol.volume, /* @__PURE__ */ new Date());
  new PublishModal(plugin.app, vol, title, numbered(vol.edition, title), () => void publish(plugin, vol.volume)).open();
}
async function confirmPublishOf(plugin, volume) {
  try {
    const { journals } = await plugin.atlas(["journal", "list"]);
    const vol = journalVolumes(journals).find((v) => v.volume === volume);
    if (!vol) {
      new import_obsidian6.Notice(`Atlas: ${JOURNALS}/${volume} is not a journal volume.`);
      return;
    }
    const why = publishBlocked(vol);
    if (why) new import_obsidian6.Notice(`Atlas: ${vol.name}: ${why}`, 8e3);
    else confirmPublish(plugin, vol);
  } catch (e) {
    new import_obsidian6.Notice(`Atlas: ${e.message}`, 1e4);
  }
}
async function publish(plugin, volume) {
  if (plugin.publishing) return;
  plugin.setPublishing(volume);
  let edition;
  try {
    const { published } = await plugin.atlas(["journal", "publish", `${JOURNALS}/${volume}`]);
    edition = published.source;
    new import_obsidian6.Notice(`Atlas: published ${edition.title}.`);
    const { ref: doc } = await plugin.atlas(["change", "start", "--kind", "ingest", "--title", `Ingest ${edition.title}`]);
    await plugin.openWhenSeen(doc.path, true);
    await plugin.runAgent(publishMessage(edition, doc), `Agent \xB7 ${doc.title}`, "publish");
  } catch (e) {
    const message = e.message;
    new import_obsidian6.Notice(
      edition ? `Atlas: the agent for ${edition.title} did not start: ${message}. The edition waits as a pending source; wiki-sync absorbs it.` : `Atlas: ${message}`,
      1e4
    );
  } finally {
    plugin.setPublishing("");
  }
}
var PublishModal = class extends import_obsidian6.Modal {
  constructor(app, vol, title, numbered2, publish2) {
    super(app);
    this.vol = vol;
    this.title = title;
    this.numbered = numbered2;
    this.publish = publish2;
  }
  vol;
  title;
  numbered;
  publish;
  onOpen() {
    const { vol } = this;
    this.setTitle(`Publish ${vol.name}`);
    const el = this.contentEl;
    el.addClass("atlas-publish");
    el.createEl("p", { text: `Publish captures the ${plural2(vol.notes, "note", "notes")} of ${JOURNALS}/${vol.volume}/ as one source:` });
    el.createDiv({ cls: "atlas-publish-title", text: this.title });
    if (this.numbered) el.createEl("p", { cls: "atlas-publish-quiet", text: "An edition of this day exists, so this one takes a number." });
    el.createEl("p", {
      text: "Then a work document opens, and an agent absorbs the edition into the wiki and cites it. You approve its changes. No agent edits the volume."
    });
    const buttons = el.createDiv({ cls: "atlas-publish-buttons" });
    buttons.createEl("button", { text: "Cancel" }).onclick = () => this.close();
    const go = buttons.createEl("button", { cls: "mod-cta", text: "Publish" });
    go.onclick = () => {
      this.close();
      this.publish();
    };
  }
  onClose() {
    this.contentEl.empty();
  }
};

// src/palette.ts
var PALETTE_VIEW = "atlas-palette";
var PALETTE_ICON = "map";
var REFRESH_MS = 3e4;
var EVENT_DELAY = 1500;
var PaletteView = class extends import_obsidian7.ItemView {
  constructor(leaf, plugin) {
    super(leaf);
    this.plugin = plugin;
  }
  plugin;
  state = null;
  error = "";
  /** The last progress line of each running work document, by path. */
  progress = /* @__PURE__ */ new Map();
  lint = null;
  busy = null;
  /** The folder of the checkout that Return proposes now. */
  returning = "";
  loading = false;
  again = false;
  soon = (0, import_obsidian7.debounce)(() => void this.refresh(), EVENT_DELAY, true);
  getViewType() {
    return PALETTE_VIEW;
  }
  getDisplayText() {
    return "Atlas";
  }
  getIcon() {
    return PALETTE_ICON;
  }
  async onOpen() {
    const touch = (...paths) => {
      if (paths.some((p) => isSnapshotPath(p, this.app.vault.configDir))) this.soon();
    };
    this.registerEvent(this.app.vault.on("create", (f) => touch(f.path)));
    this.registerEvent(this.app.vault.on("modify", (f) => touch(f.path)));
    this.registerEvent(this.app.vault.on("delete", (f) => touch(f.path)));
    this.registerEvent(this.app.vault.on("rename", (f, old) => touch(f.path, old)));
    this.registerEvent(this.app.workspace.on("file-open", () => this.render()));
    this.registerInterval(window.setInterval(() => void this.refresh(), REFRESH_MS));
    this.render();
    void this.refresh();
  }
  async onClose() {
    this.soon.cancel();
  }
  /** Reads the status again. One read runs at a time; a call during a read runs one more after it. */
  async refresh() {
    if (this.loading) {
      this.again = true;
      return;
    }
    this.loading = true;
    try {
      do {
        this.again = false;
        await this.readStatus();
      } while (this.again);
    } finally {
      this.loading = false;
    }
    this.render();
  }
  async readStatus() {
    try {
      const out = await this.plugin.atlas(["vault"]);
      const { groups } = await sessionGroups(this.app, this.plugin.staleHours());
      const state = paletteState(out.status, groups.open.length);
      const progress = /* @__PURE__ */ new Map();
      for (const r of state.running) {
        const file = this.app.vault.getFileByPath(r.path);
        if (file) progress.set(r.path, lastProgressLine(await this.app.vault.read(file)));
      }
      this.state = state;
      this.progress = progress;
      this.error = "";
    } catch (e) {
      this.error = e.message;
    }
    this.plugin.checkConversations();
  }
  render() {
    const root = this.contentEl;
    root.empty();
    root.addClass("atlas-palette");
    this.renderStatus(root);
    this.renderRunning(root);
    this.renderJournals(root);
    this.renderCheckouts(root);
    this.renderActions(root);
  }
  section(root, name) {
    const el = root.createDiv({ cls: "atlas-palette-section" });
    el.createDiv({ cls: "atlas-palette-heading", text: name });
    return el;
  }
  row(parent, key, name, value) {
    const row = parent.createDiv({ cls: "atlas-palette-row" });
    row.dataset.row = key;
    row.createSpan({ cls: "atlas-palette-name", text: name });
    row.createSpan({ cls: "atlas-palette-value", text: value });
    return row;
  }
  link(parent, title, path) {
    const a = parent.createEl("a", { cls: "atlas-palette-link", text: title, href: "#" });
    a.setAttr("title", path);
    a.onclick = (evt) => {
      evt.preventDefault();
      void this.openPath(path, evt.metaKey || evt.ctrlKey);
    };
    return a;
  }
  renderStatus(root) {
    const el = this.section(root, "Status");
    if (this.error) el.createDiv({ cls: "atlas-palette-error", text: `Atlas: ${this.error}` });
    const s = this.state;
    if (!s) {
      if (!this.error) el.createDiv({ cls: "atlas-palette-quiet", text: "Reading the vault\u2026" });
      return;
    }
    const list = (refs, line) => {
      if (refs.length === 0) return;
      const ul = el.createEl("ul", { cls: "atlas-palette-list" });
      for (const r of refs) {
        const li = ul.createEl("li");
        this.link(li, r.title, r.path);
        const extra = line?.(r);
        if (extra) li.createDiv({ cls: "atlas-palette-progress", text: extra });
      }
    };
    this.row(el, "proposed", "Proposed changes", String(s.proposed.length));
    list(s.proposed);
    this.row(el, "running", "Running work", String(s.running.length));
    list(s.running, (r) => [r.kind, this.progress.get(r.path) || "no step yet"].filter((x) => x).join(" \xB7 "));
    this.row(el, "ingest", "Ingest", plural2(s.ingest.length, "file", "files"));
    if (s.ingest.length > 0) {
      const ul = el.createEl("ul", { cls: "atlas-palette-list atlas-palette-files" });
      for (const name of s.ingest) ul.createEl("li", { text: name });
    }
    this.row(el, "pending", "Pending sources", String(s.pending));
    const sessions = this.row(el, "sessions", "Live sessions", String(s.sessions));
    const open = sessions.createEl("button", { cls: "atlas-palette-small", text: "Open sessions" });
    open.onclick = () => void this.plugin.openSessions();
    this.row(el, "trash", "Trash", plural2(s.trash, "file", "files"));
    this.row(el, "journals", "Journals", `${s.toPublish} to publish`);
    this.row(el, "checkouts", "Checkouts", `${s.toReturn} to return`);
    this.row(el, "problems", "Lint problems", plural2(s.problems, "error", "errors"));
  }
  renderRunning(root) {
    const running = this.plugin.conversations.list();
    if (running.length === 0) return;
    const el = this.section(root, "Running");
    const ul = el.createEl("ul", { cls: "atlas-palette-list" });
    for (const c of running) {
      const li = ul.createEl("li", { cls: "atlas-palette-agent" });
      li.createSpan({ cls: "atlas-palette-badge", text: c.label });
      this.link(li, c.path.slice(c.path.lastIndexOf("/") + 1).replace(/\.md$/, ""), c.path);
    }
  }
  renderJournals(root) {
    const s = this.state;
    if (!s) return;
    const el = this.section(root, "Journals");
    if (s.journals.length === 0) {
      el.createDiv({ cls: "atlas-palette-quiet", text: `No journal yet: a volume is a folder directly under ${JOURNALS}/.` });
      return;
    }
    for (const vol of s.journals) this.renderVolume(el, vol);
  }
  renderVolume(parent, vol) {
    const el = parent.createDiv({ cls: "atlas-palette-volume" });
    el.dataset.volume = vol.volume;
    el.dataset.changed = String(vol.changed);
    const head = el.createDiv({ cls: "atlas-palette-row" });
    head.createSpan({ cls: "atlas-palette-name atlas-palette-volume-name", text: vol.name }).setAttr("title", `${JOURNALS}/${vol.volume}/`);
    if (vol.changed) head.createSpan({ cls: "atlas-palette-badge atlas-palette-changed", text: "changed" });
    head.createSpan({ cls: "atlas-palette-value", text: plural2(vol.notes, "note", "notes") });
    const edition = el.createDiv({ cls: "atlas-palette-progress atlas-palette-edition" });
    if (vol.edition) {
      const file = this.app.metadataCache.getFirstLinkpathDest(vol.edition, "");
      if (file) this.link(edition, vol.edition, file.path);
      else edition.setText(vol.edition);
    } else {
      edition.setText("never published");
    }
    const why = publishBlocked(vol);
    const publishing = this.plugin.publishing === vol.volume;
    const button = el.createEl("button", { cls: "atlas-palette-small atlas-palette-publish", text: publishing ? "Publish\u2026" : "Publish" });
    if (why || this.busy || this.plugin.publishing) {
      button.disabled = true;
      button.setAttr("title", why || "Another action runs.");
    } else {
      button.addClass("mod-cta");
    }
    button.onclick = () => confirmPublish(this.plugin, vol);
  }
  renderCheckouts(root) {
    const s = this.state;
    if (!s) return;
    const el = this.section(root, "Checkouts");
    if (s.checkouts.length === 0) {
      el.createDiv({ cls: "atlas-palette-quiet", text: `No checkout yet: Checkout asks the librarian for the material on a subject, and copies it into ${CHECKOUT}/.` });
      return;
    }
    for (const c of s.checkouts) this.renderCheckout(el, c);
  }
  renderCheckout(parent, c) {
    const el = parent.createDiv({ cls: "atlas-palette-checkout" });
    el.dataset.folder = c.folder;
    const head = el.createDiv({ cls: "atlas-palette-row" });
    this.link(head.createSpan({ cls: "atlas-palette-name atlas-palette-request" }), c.request, readingListPath(c));
    head.createSpan({ cls: "atlas-palette-value", text: plural2(c.documents, "document", "documents") });
    const line = [c.date, `${c.edited} edited`];
    if (c.returned) line.push(`returned ${day(c.returned)}`);
    el.createDiv({ cls: "atlas-palette-progress atlas-palette-checkout-line", text: line.join(" \xB7 ") });
    const why = returnBlocked(c);
    const returning = this.busy === "return" && this.returning === c.folder;
    const button = el.createEl("button", { cls: "atlas-palette-small atlas-palette-return", text: returning ? "Return\u2026" : "Return" });
    if (why || this.busy || this.plugin.publishing) {
      button.disabled = true;
      button.setAttr("title", why || "Another action runs.");
    } else {
      button.addClass("mod-cta");
    }
    button.onclick = () => {
      this.returning = c.folder;
      void this.act("return", () => returnCheckout(this.plugin, c));
    };
  }
  renderActions(root) {
    const el = this.section(root, "Actions");
    const files = this.state?.ingest.length ?? 0;
    this.action(el, "ingest", files > 0 ? `Ingest ${plural2(files, "file", "files")}` : "Ingest", files > 0 ? "" : "ingest/ holds no file.", () => this.ingest());
    this.actionButton(el, "checkout", "Checkout", "").onclick = () => this.askCheckout();
    this.action(el, "lint", "Wiki lint", "", () => this.runLint());
    if (this.lint) this.renderLint(el, this.lint);
    const file = this.app.workspace.getActiveFile();
    this.action(el, "trash", "Safe delete this file", file ? "" : "Open a file first.", () => this.safeDelete(), file?.path ?? "");
  }
  /** A button of an action. why disables it; a running action disables every one. */
  action(parent, name, text, why, run, note = "") {
    this.actionButton(parent, name, text, why, note).onclick = () => void this.act(name, run);
  }
  /** The button of an action, with no click handler yet. */
  actionButton(parent, name, text, why, note = "") {
    const wrap = parent.createDiv({ cls: "atlas-palette-action" });
    wrap.dataset.action = name;
    const button = wrap.createEl("button", { text: this.busy === name ? `${text}\u2026` : text });
    if (why || this.busy || this.plugin.publishing) {
      button.disabled = true;
      button.setAttr("title", why || "Another action runs.");
    }
    if (why) wrap.createDiv({ cls: "atlas-palette-quiet", text: why });
    else if (note) wrap.createDiv({ cls: "atlas-palette-quiet atlas-palette-path", text: note });
    return button;
  }
  async act(name, run) {
    if (this.busy || this.plugin.publishing) return;
    this.busy = name;
    this.render();
    try {
      await run();
    } catch (e) {
      new import_obsidian7.Notice(`Atlas: ${e.message}`, 1e4);
    } finally {
      this.busy = null;
      this.render();
      void this.refresh();
    }
  }
  renderLint(parent, lint) {
    const el = parent.createDiv({ cls: "atlas-palette-lint" });
    el.createDiv({ cls: "atlas-palette-lint-counts", text: lint.counts });
    if (lint.first.length > 0) {
      const ul = el.createEl("ul", { cls: "atlas-palette-list" });
      for (const f of lint.first) {
        const li = ul.createEl("li", { cls: "atlas-palette-finding" });
        li.dataset.severity = f.severity;
        li.createSpan({ cls: "atlas-palette-badge", text: f.check });
        this.link(li, f.doc.title || f.doc.path, f.doc.path);
        li.createDiv({ cls: "atlas-palette-progress", text: f.message });
      }
    }
    if (lint.more > 0) el.createDiv({ cls: "atlas-palette-quiet", text: `${lint.more} more: run wiki-review for all of them.` });
    if (lint.repairable > 0) {
      this.action(el, "repair", "Repair with an agent", "", () => this.repair(), `${plural2(lint.repairable, "finding", "findings")} that a change repairs`);
    }
  }
  // Actions
  async ingest() {
    const files = this.state?.ingest ?? [];
    if (files.length === 0) return;
    const doc = await this.start(["change", "start", "--kind", "ingest", ...files.map((f) => `--file=${f}`)]);
    await this.plugin.runAgent(ingestMessage(doc), `Agent \xB7 ${doc.title}`, "ingest");
  }
  askCheckout() {
    new CheckoutModal(this.app, (request) => void this.act("checkout", () => startCheckout(this.plugin, request))).open();
  }
  async runLint() {
    this.lint = lintSummary(await this.plugin.atlas(["lint"]));
  }
  async repair() {
    const doc = await this.start(["change", "start", "--kind", "repair", "--title", "Repair the lint findings"]);
    await this.plugin.runAgent(repairMessage(doc), `Agent \xB7 ${doc.title}`, "repair");
  }
  async safeDelete() {
    const file = this.app.workspace.getActiveFile();
    if (!file) return;
    await saveOpen(this.app, file.path);
    const arg = file.path.startsWith("-") ? `./${file.path}` : file.path;
    const out = await this.plugin.atlas(["vault", "trash", arg], [2]);
    const outcome = trashOutcome(out.trash);
    if (outcome.kind === "linked") {
      new BacklinksModal(
        this.app,
        outcome,
        (path) => void this.openPath(path, false),
        () => void this.act("trash", () => this.plugin.runAgent(resolveMessage(outcome, outcome.backlinks), `Agent \xB7 Remove ${outcome.title}`, "remove"))
      ).open();
      return;
    }
    new import_obsidian7.Notice(`Atlas: ${outcome.line}`, outcome.kind === "error" ? 1e4 : 6e3);
  }
  /** Starts a work document and opens it. */
  async start(args) {
    const { ref } = await this.plugin.atlas(args);
    await this.openPath(ref.path, true);
    return ref;
  }
  openPath(path, newTab) {
    return this.plugin.openWhenSeen(path, newTab);
  }
};
var BacklinksModal = class extends import_obsidian7.Modal {
  constructor(app, outcome, show, resolve) {
    super(app);
    this.outcome = outcome;
    this.show = show;
    this.resolve = resolve;
  }
  outcome;
  show;
  resolve;
  onOpen() {
    const { outcome } = this;
    this.setTitle(`${outcome.title} stays`);
    const el = this.contentEl;
    el.addClass("atlas-backlinks");
    el.createEl("p", {
      text: `${plural2(outcome.backlinks.length, "document links", "documents link")} ${outcome.path}, so safe delete moved nothing. Point each link elsewhere, or drop it; then the file can go to trash/.`
    });
    const ul = el.createEl("ul");
    for (const b of outcome.backlinks) {
      const li = ul.createEl("li");
      const a = li.createEl("a", { text: b.title || b.path, href: "#" });
      a.setAttr("title", b.path);
      a.onclick = (evt) => {
        evt.preventDefault();
        this.close();
        this.show(b.path);
      };
      if (b.type) li.createSpan({ cls: "atlas-backlinks-type", text: ` ${b.kind || b.type}` });
    }
    const buttons = el.createDiv({ cls: "atlas-backlinks-buttons" });
    buttons.createEl("button", { text: "Close" }).onclick = () => this.close();
    const go = buttons.createEl("button", { cls: "mod-cta", text: "Resolve with an agent" });
    go.onclick = () => {
      this.close();
      this.resolve();
    };
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
  snapshotQuietSeconds: SNAPSHOT_QUIET_DEFAULT
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
    const status2 = binary.descEl.createDiv({ cls: "atlas-setting-status" });
    const showVersion = async () => {
      const bin = findBinary(this.plugin.settings.binaryPath);
      if (!bin) {
        status2.setText("No binary found.");
        return;
      }
      try {
        status2.setText(`Uses ${bin} (${await binaryVersion(bin)}).`);
      } catch (e) {
        status2.setText(`Cannot run ${bin}: ${e.message}`);
      }
    };
    void showVersion();
    new import_obsidian8.Setting(containerEl).setName("Keep the views fresh").setDesc("Runs atlas-obsidian vault sync --views two seconds after a note changes, so the views, the statuses, and the callouts follow your edits.").addToggle(
      (toggle) => toggle.setValue(this.plugin.settings.syncOnChange).onChange(async (value) => {
        this.plugin.settings.syncOnChange = value;
        await this.plugin.saveSettings();
      })
    );
    new import_obsidian8.Setting(containerEl).setName("Snapshot after a quiet period").setDesc("Seconds with no file change before Atlas commits your edits to the vault's history. 0 turns it off.").addText((text) => {
      text.inputEl.type = "number";
      text.inputEl.min = "0";
      text.setPlaceholder(String(DEFAULT_SETTINGS.snapshotQuietSeconds)).setValue(String(this.plugin.settings.snapshotQuietSeconds));
      text.inputEl.addEventListener("change", async () => {
        const seconds = quietSeconds(text.getValue());
        text.setValue(String(seconds));
        await this.plugin.setSnapshotQuiet(seconds);
      });
    });
    new import_obsidian8.Setting(containerEl).setName("Agents").setHeading();
    const agents = containerEl.createDiv();
    void this.agents(agents);
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
    intro.setText(`Start agent, Resume, and the palette's agents without Duet read these. ${shortHome(config.files.global)} holds them for every vault; .atlas/config.json in this vault overrides them, key by key. atlas-obsidian config shows the result.`);
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
      const same = (key, label) => global ? "" : `Same as all vaults (${label || inherited(config, key)})`;
      new import_obsidian8.Setting(el).setName("Agent").addDropdown((d) => {
        if (!global) d.addOption("", same("agent", AGENT_NAMES[inherited(config, "agent")]));
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
        if (!global) d.addOption("", same("terminal", TERMINAL_NAMES[inherited(config, "terminal")]));
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
var MAX_WITH = 30;
function tagDocs(app) {
  const out = [];
  for (const file of app.vault.getMarkdownFiles()) {
    if (!isDocumentPath(file.path)) continue;
    const fm = app.metadataCache.getFileCache(file)?.frontmatter;
    if (!fm || !isDocumentType(fm.type)) continue;
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
  chosen = [];
  rerender = (0, import_obsidian9.debounce)(() => this.render(), 500, true);
  constructor(leaf) {
    super(leaf);
  }
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
  add(tag) {
    if (!this.chosen.includes(tag)) this.chosen.push(tag);
    this.render();
  }
  drop(tag) {
    this.chosen = this.chosen.filter((t) => t !== tag);
    this.render();
  }
  render() {
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
  async openView(tag) {
    const file = this.app.vault.getAbstractFileByPath(tagViewPath(tag));
    if (file instanceof import_obsidian9.TFile) await this.app.workspace.getLeaf(false).openFile(file);
  }
};

// src/main.ts
var SYNC_DELAY = 2e3;
var LEGACY_KEYS = ["agentCommand", "terminal", "terminalCommand"];
var ECHO_WINDOW = 5e3;
var SEE_MS = 1e4;
var AtlasPlugin = class extends import_obsidian10.Plugin {
  settings = { ...DEFAULT_SETTINGS };
  /** The agent settings of 8.0.2, kept in data.json until they move to the vault's config file. */
  legacy = null;
  syncing = false;
  syncTimer = null;
  pending = /* @__PURE__ */ new Set();
  echoes = /* @__PURE__ */ new Set();
  echoUntil = 0;
  lastAutoError = "";
  /** Commits the user's edits after a quiet period. */
  snapshots = new QuietTimer(
    { set: (fn, ms) => window.setTimeout(fn, ms), clear: (h) => window.clearTimeout(h) },
    () => this.snapshot(),
    0
  );
  lastSnapshotError = "";
  sessionsRibbon = null;
  /** The journal volume that a publish captures now, or "". */
  publishing = "";
  /** The agents the palette started through Duet, while their turn runs. */
  conversations = new Conversations(
    () => this.paletteViews().forEach((v) => v.render()),
    (c, turn) => {
      if (turn.status === "failed") new import_obsidian10.Notice(`Atlas: the ${c.label} agent stopped: ${turn.error ?? "its turn failed"}.`, 1e4);
    }
  );
  async onload() {
    await this.loadSettings();
    this.addSettingTab(new AtlasSettingTab(this.app, this));
    this.registerMarkdownCodeBlockProcessor("atlas-repo", repoProcessor(this));
    this.registerMarkdownCodeBlockProcessor("atlas-change", changeProcessor(this, new ChangeRunner(this)));
    this.addRibbonIcon("refresh-cw", "Atlas: sync the vault", () => void this.sync(true));
    this.addCommand({ id: "sync", name: "Sync the vault", callback: () => void this.sync(true) });
    this.registerView(TAG_NAV_VIEW, (leaf) => new TagNavigator(leaf));
    this.addRibbonIcon(NAV_ICON, "Atlas: open the Atlas navigator", () => void this.openTags());
    this.addCommand({ id: "open-tags", name: "Open the Atlas navigator", callback: () => void this.openTags() });
    this.registerView(PALETTE_VIEW, (leaf) => new PaletteView(leaf, this));
    this.addRibbonIcon(PALETTE_ICON, "Atlas", () => void this.openPalette());
    this.addCommand({ id: "open-palette", name: "Open the Atlas palette", callback: () => void this.openPalette() });
    this.addCommand({ id: "start-agent", name: "Start agent", callback: () => void this.startAgent() });
    this.addCommand({
      id: "publish-journal",
      name: "Publish this journal volume",
      checkCallback: (checking) => {
        const volume = volumeOf(this.app.workspace.getActiveFile()?.path ?? "");
        if (!volume) return false;
        if (!checking) void confirmPublishOf(this, volume);
        return true;
      }
    });
    this.registerView(SESSIONS_VIEW, (leaf) => new SessionsView(leaf, this));
    this.sessionsRibbon = this.addRibbonIcon("bot", "Atlas: open the sessions", () => void this.openSessions());
    this.sessionsRibbon.addClass("atlas-sessions-ribbon");
    this.addCommand({ id: "open-sessions", name: "Open the sessions", callback: () => void this.openSessions() });
    this.addCommand({ id: "migrate", name: `Migrate this vault to the ${layoutName(LAYOUT)} layout`, callback: () => void this.migrate() });
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
      this.watchForSnapshots();
    });
    const first = this.app.metadataCache.on("resolved", () => {
      this.app.metadataCache.offref(first);
      this.refreshSessions();
    });
    this.registerEvent(first);
  }
  onunload() {
    if (this.syncTimer !== null) window.clearTimeout(this.syncTimer);
    this.snapshots.stop();
    this.conversations.stop();
  }
  async loadSettings() {
    const saved = await this.loadData();
    const old = Object.entries(saved ?? {}).filter(([k]) => LEGACY_KEYS.includes(k));
    this.legacy = old.length > 0 ? Object.fromEntries(old) : null;
    this.settings = { ...DEFAULT_SETTINGS };
    for (const key of Object.keys(DEFAULT_SETTINGS)) {
      if (saved && saved[key] !== void 0) this.settings[key] = saved[key];
    }
    this.settings.snapshotQuietSeconds = quietSeconds(this.settings.snapshotQuietSeconds);
  }
  async saveSettings() {
    await this.saveData({ ...this.legacy, ...this.settings });
  }
  /** Runs one atlas command in this vault and returns its JSON; answers are the exit codes that print an answer too. */
  atlas(args, answers = []) {
    const adapter = this.app.vault.adapter;
    if (!(adapter instanceof import_obsidian10.FileSystemAdapter)) {
      return Promise.reject(new AtlasError("this vault is not a folder on disk"));
    }
    return runAtlas(findBinary(this.settings.binaryPath), adapter.getBasePath(), args, answers);
  }
  // Sync
  /** A manual sync runs every step; an automatic one runs the steps that read no git. */
  async sync(manual) {
    if (this.syncing) {
      if (manual) new import_obsidian10.Notice("Atlas: a sync is running.");
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
      if (manual) new import_obsidian10.Notice(`Atlas: ${syncSummary(out.synced)}`);
      for (const line of strayNotices(out.synced)) new import_obsidian10.Notice(`Atlas: ${line}`, 0);
    } catch (e) {
      const message = e.message;
      if (manual || message !== this.lastAutoError) new import_obsidian10.Notice(`Atlas: ${message}`);
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
  // Quiet snapshots
  /** Counts file events from now on: the events of Obsidian's first load are not edits. */
  watchForSnapshots() {
    const touch = (...paths) => {
      if (paths.some((p) => isSnapshotPath(p, this.app.vault.configDir))) this.snapshots.touch();
    };
    this.registerEvent(this.app.vault.on("create", (f) => touch(f.path)));
    this.registerEvent(this.app.vault.on("modify", (f) => touch(f.path)));
    this.registerEvent(this.app.vault.on("delete", (f) => touch(f.path)));
    this.registerEvent(this.app.vault.on("rename", (f, oldPath) => touch(f.path, oldPath)));
    this.snapshots.setQuiet(this.settings.snapshotQuietSeconds * 1e3);
    this.snapshots.touch();
  }
  async setSnapshotQuiet(seconds) {
    this.settings.snapshotQuietSeconds = seconds;
    await this.saveSettings();
    if (this.app.workspace.layoutReady) this.snapshots.setQuiet(seconds * 1e3);
  }
  /**
   * Commits the hand edits as one snapshot. It shows no notice: a held lock tries again
   * after the next quiet period, and any other failure waits for the next edit.
   */
  async snapshot() {
    if (!this.migrated()) return false;
    try {
      await this.atlas(["vault", "snapshot"]);
      this.lastSnapshotError = "";
      return false;
    } catch (e) {
      const message = e.message;
      if (isLockHeld(message)) return true;
      if (message !== this.lastSnapshotError) console.warn(`Atlas: the snapshot failed: ${message}`);
      this.lastSnapshotError = message;
      return false;
    }
  }
  // Layout
  /** The layout the vault document records; this plugin's when there is none to read. */
  layout() {
    const atlas = this.app.vault.getFileByPath("Atlas.md");
    if (!atlas) return LAYOUT;
    return layoutOf(this.app.metadataCache.getFileCache(atlas)?.frontmatter);
  }
  /** Whether the vault has the layout this plugin reads. */
  migrated() {
    return this.layout() >= LAYOUT;
  }
  checkLayout() {
    if (this.migrated()) return;
    const notice = new import_obsidian10.Notice("", 0);
    const el = notice.messageEl;
    const layout = this.layout();
    const needs = `Atlas: this vault has the ${layoutName(layout)} layout. This plugin needs the ${layoutName(LAYOUT)} layout.`;
    if (layout < MIGRATES_FROM) {
      el.createDiv({ text: `${needs} Migrate the vault to 8.x with Atlas 8.1.1 first.` });
      return;
    }
    el.createDiv({ text: needs });
    const button = el.createEl("button", { text: "Show the migration", cls: "mod-cta atlas-notice-button" });
    button.onclick = () => {
      notice.hide();
      void this.migrate();
    };
  }
  async migrate() {
    try {
      const report2 = await this.atlas(["vault", "migrate", "--dry-run"]);
      new MigrationModal(this.app, report2, this.layout(), async () => {
        try {
          const done = await this.atlas(["vault", "migrate"]);
          new import_obsidian10.Notice(`Atlas: ${migrationSummary(done)}`, 1e4);
          for (const line of strayNotices(done)) new import_obsidian10.Notice(`Atlas: ${line}`, 0);
        } catch (e) {
          new import_obsidian10.Notice(`Atlas: ${e.message}`, 1e4);
        }
      }).open();
    } catch (e) {
      new import_obsidian10.Notice(`Atlas: ${e.message}`, 1e4);
    }
  }
  // Views
  async openTags() {
    const { workspace } = this.app;
    let leaf = workspace.getLeavesOfType(TAG_NAV_VIEW)[0];
    if (!leaf) {
      const left = workspace.getLeftLeaf(false);
      if (!left) return;
      await left.setViewState({ type: TAG_NAV_VIEW, active: true });
      leaf = left;
    }
    await workspace.revealLeaf(leaf);
  }
  async openPalette() {
    const { workspace } = this.app;
    let leaf = workspace.getLeavesOfType(PALETTE_VIEW)[0];
    if (!leaf) {
      const right = workspace.getRightLeaf(false);
      if (!right) return;
      await right.setViewState({ type: PALETTE_VIEW, active: true });
      leaf = right;
    }
    await workspace.revealLeaf(leaf);
  }
  /** Marks the volume a publish captures, "" when it ends, in every palette. */
  setPublishing(volume) {
    this.publishing = volume;
    for (const view of this.paletteViews()) {
      view.render();
      if (!volume) void view.refresh();
    }
  }
  /** Opens a file once Obsidian sees it; a document the binary just wrote takes a moment. */
  async openWhenSeen(path, newTab) {
    const deadline = Date.now() + SEE_MS;
    let file = this.app.vault.getFileByPath(path);
    while (!file && Date.now() < deadline) {
      await new Promise((resolve) => window.setTimeout(resolve, 100));
      file = this.app.vault.getFileByPath(path);
    }
    if (!(file instanceof import_obsidian10.TFile)) {
      new import_obsidian10.Notice(`Atlas: Obsidian does not see ${path} yet.`);
      return;
    }
    await this.app.workspace.getLeaf(newTab ? "tab" : false).openFile(file);
  }
  paletteViews() {
    return this.app.workspace.getLeavesOfType(PALETTE_VIEW).map((leaf) => leaf.view).filter((v) => v instanceof PaletteView);
  }
  // Sessions
  sessionViews() {
    return this.app.workspace.getLeavesOfType(SESSIONS_VIEW).map((leaf) => leaf.view).filter((v) => v instanceof SessionsView);
  }
  refreshSessions = (0, import_obsidian10.debounce)(
    () => {
      void sessionGroups(this.app, this.staleHours()).then(({ groups }) => {
        const waiting = groups.open.filter((s) => s.state === "needs you").length;
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
  /** Moves the agent settings of 8.0.2 into the vault's config file, once. */
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
        new import_obsidian10.Notice(`Atlas: cannot open the terminal (${e.message}). The Atlas settings choose it.`, 8e3);
      }
    }
    await navigator.clipboard.writeText(command);
    new import_obsidian10.Notice(`Atlas: copied ${what}. Run it in a terminal.`);
  }
  /** Starts the agent in the vault, in a terminal; a prompt is its first message. */
  async startAgent(prompt = "") {
    const adapter = this.app.vault.adapter;
    if (!(adapter instanceof import_obsidian10.FileSystemAdapter)) return;
    let config;
    try {
      config = await this.agentConfig();
    } catch (e) {
      new import_obsidian10.Notice(`Atlas: cannot read the agent preferences: ${e.message}`, 8e3);
      return;
    }
    await this.runInTerminal(startCommand(adapter.getBasePath(), config.preferences.agent_command, prompt), "the agent command", config);
  }
  /**
   * Starts an agent with a first message: in a Duet conversation, which the palette lists
   * while its turn runs, else in a terminal. title names the conversation note; label says
   * what the agent does.
   */
  async runAgent(message, title, label) {
    const api = duetApi(this.app);
    if (api) {
      try {
        const { path } = await api.newConversation({ message, title, loadUserSetup: true });
        this.conversations.follow(api, path, label);
        return;
      } catch (e) {
        new import_obsidian10.Notice(`Atlas: Duet did not start the agent (${e.message}). Atlas starts it in a terminal.`, 1e4);
      }
    } else {
      new import_obsidian10.Notice("Atlas: Duet runs the agents in Obsidian. Duet 0.3.0 or later is not on, so Atlas starts the agent in a terminal.", 1e4);
    }
    await this.startAgent(message);
  }
  /** Drops the conversations whose turn ended while no event came, such as when Duet turned off. */
  checkConversations() {
    this.conversations.check(duetApi(this.app));
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
var MigrationModal = class extends import_obsidian10.Modal {
  constructor(app, report2, layout, run) {
    super(app);
    this.report = report2;
    this.layout = layout;
    this.run = run;
  }
  report;
  layout;
  run;
  onOpen() {
    const r = this.report;
    const from = r.from || layoutName(this.layout);
    this.setTitle(`Migrate to the ${layoutName(LAYOUT)} layout`);
    const el = this.contentEl;
    el.addClass("atlas-migration");
    el.createEl("p", { text: `The migration moves ${r.vault} from the ${from} layout to the ${layoutName(LAYOUT)} layout in one commit. git revert takes it back. It:` });
    const steps = el.createEl("ul");
    for (const step of migrationSteps(this.layout)) steps.createEl("li", { text: step });
    const list = (items, title, line) => {
      if (!items || items.length === 0) return;
      const d = el.createEl("details");
      d.createEl("summary", { text: `${title} (${items.length})` });
      const ul = d.createEl("ul");
      for (const item of items) ul.createEl("li", { text: line(item) });
    };
    const move = (m) => `${m.from} \u2192 ${m.to}`;
    list(r.moved, "Files to move", move);
    list(r.edited, "Files to edit", (p) => p);
    list(r.strays, "Your notes in views/, to move to ingest/", move);
    list(r.warnings, "Warnings", (w) => w);
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
