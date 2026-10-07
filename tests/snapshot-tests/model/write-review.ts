/**
 * Write review.html for failed snapshot cases: wowhead | expected | actual.
 *
 *   bun tests/snapshot-tests/model/_write-review.ts
 *   bun tests/snapshot-tests/model/_write-review.ts retail/npc-187590-merithra
 *
 * With no args, uses `review-failures.json` written by the snapshot test.
 * Open tests/snapshot-tests/model/review.html (or serve that folder).
 * The snapshot test prints a blue file:// link after each run.
 */
import { existsSync, readdirSync, readFileSync, writeFileSync } from 'fs';
import path from 'path';
import { pathToFileURL } from 'url';

const root = import.meta.dir;
const sidecar = path.join(root, 'review-failures.json');

interface ReviewInput {
  readonly suite: string;
  readonly slug: string;
  readonly detail?: string;
  readonly ok?: boolean;
}

interface ReviewCard {
  readonly suite: string;
  readonly slug: string;
  readonly name: string;
  readonly actualSrc: string;
  readonly expectedSrc: string;
  readonly ok: boolean;
  readonly wowheadSrc?: string;
  readonly base?: string;
  readonly detail?: string;
}

export function writeReview(listed: readonly ReviewInput[]): number {
  const cases = collectCases(listed);
  const reviewPath = path.join(root, 'review.html');
  writeFileSync(reviewPath, reviewHtml(cases));
  writeFileSync(sidecar, `${JSON.stringify(listed, null, 2)}\n`);
  const href = pathToFileURL(reviewPath).href;
  const failed = cases.filter((item) => !item.ok).length;
  const passed = cases.filter((item) => item.ok).length;
  console.log(`Open \x1b[34m${href}\x1b[0m (${failed} failed, ${passed} passed)`);
  return cases.length;
}

function buildCase(suite: string, slug: string, detail: string | undefined, ok: boolean): ReviewCard | undefined {
  const actualSrc = `${suite}/${slug}/${slug}.actual.png`;
  const expectedSrc = `${suite}/${slug}/${slug}.expected.png`;
  const wowheadSrc = `${suite}/${slug}/${slug}.wowhead.png`;
  if (!existsSync(path.join(root, actualSrc)) || !existsSync(path.join(root, expectedSrc))) return undefined;
  const base = readCaseBase(suite, slug);
  return {
    suite,
    slug,
    name: `${suite}/${slug}`,
    actualSrc,
    expectedSrc,
    ok,
    ...(existsSync(path.join(root, wowheadSrc)) ? { wowheadSrc } : {}),
    ...(base ? { base } : {}),
    ...(detail ? { detail } : {}),
  };
}

function collectCases(listed: readonly ReviewInput[]): ReviewCard[] {
  const byId = new Map<string, ReviewCard>();
  for (const item of listed) {
    const built = buildCase(item.suite, item.slug, item.detail, item.ok === true);
    if (built) byId.set(built.name, built);
  }
  for (const suite of ['retail', 'classic', 'mount']) {
    const dir = path.join(root, suite);
    if (!existsSync(dir)) continue;
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isDirectory()) continue;
      const name = `${suite}/${entry.name}`;
      if (byId.has(name)) continue;
      const built = buildCase(suite, entry.name, undefined, true);
      if (built) byId.set(name, built);
    }
  }
  return [...byId.values()];
}

function readCaseBase(suite: string, slug: string): string {
  const manifestPath = path.join(root, suite, slug, `${slug}.manifest.json`);
  if (!existsSync(manifestPath)) return '';
  try {
    const parsed: unknown = JSON.parse(readFileSync(manifestPath, 'utf8'));
    if (!isRecord(parsed) || !isRecord(parsed.case) || typeof parsed.case.base !== 'string') return '';
    return parsed.case.base;
  } catch {
    return '';
  }
}

function parseId(id: string): ReviewInput {
  const slash = id.indexOf('/');
  if (slash <= 0 || slash === id.length - 1) throw new Error(`expected suite/slug, got ${id}`);
  return { suite: id.slice(0, slash), slug: id.slice(slash + 1) };
}

function loadCliCases(): ReviewInput[] {
  const ids = process.argv.slice(2).filter((arg) => !arg.startsWith('--') && !/_write-review\.(mjs|ts)$/.test(arg));
  if (ids.length > 0) return ids.map(parseId);
  if (!existsSync(sidecar)) return [];
  const parsed: unknown = JSON.parse(readFileSync(sidecar, 'utf8'));
  if (!Array.isArray(parsed)) return [];
  const listed: ReviewInput[] = [];
  for (const item of parsed) {
    if (!isRecord(item) || typeof item.suite !== 'string' || typeof item.slug !== 'string') continue;
    listed.push({
      suite: item.suite,
      slug: item.slug,
      ...(typeof item.detail === 'string' && item.detail !== '' ? { detail: item.detail } : {}),
      ok: item.ok === true,
    });
  }
  return listed;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function reviewHtml(cases: readonly ReviewCard[]): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Snapshot review</title>
<style>
:root { color-scheme: dark; }
* { box-sizing: border-box; }
html, body { margin: 0; background: #111213; color: #e8e8ea; font: 14px/1.4 ui-sans-serif, system-ui, sans-serif; }
header { position: sticky; top: 0; z-index: 5; display: flex; flex-wrap: wrap; gap: 8px 16px; align-items: center; padding: 12px 16px; background: #111213e6; backdrop-filter: blur(8px); border-bottom: 1px solid #2a2b2e; }
h1 { margin: 0; font-size: 16px; font-weight: 650; }
.status { margin: 0; }
.status select { background: #1c1d20; color: #e8e8ea; border: 1px solid #33343a; border-radius: 8px; padding: 6px 10px; font: inherit; font-size: 16px; font-weight: 650; }
.filters, .vote-filters { display: flex; gap: 6px; flex-wrap: wrap; }
header button, .filters button, .vote-filters button { background: #1c1d20; color: #c8c8cc; border: 1px solid #33343a; border-radius: 999px; padding: 4px 12px; cursor: pointer; }
.filters button[aria-pressed="true"], .vote-filters button[aria-pressed="true"] { background: #2e3a52; border-color: #5b8def; color: #fff; }
.grid { display: flex; flex-direction: column; gap: 16px; padding: 16px; }
.card { margin: 0; background: #1a1b1e; border: 1px solid #2a2b2e; border-radius: 8px; overflow: hidden; }
.card.good { border-color: #2f6d3a; }
.card.bad { border-color: #8a3a3a; }
.card > h2 { display: flex; gap: 8px; align-items: center; margin: 0; padding: 10px 12px; font-size: 14px; font-weight: 650; border-bottom: 1px solid #2a2b2e; }
.votes { display: flex; gap: 4px; flex: none; }
.card > h2 .votes { margin-left: auto; }
.votes button { width: 36px; height: 32px; padding: 0; border-radius: 6px; font-size: 16px; background: #1c1d20; color: #c8c8cc; border: 1px solid #33343a; cursor: pointer; }
.votes button[aria-pressed="true"].up { background: #1e4a28; border-color: #3d9a52; color: #8ee0a0; }
.votes button[aria-pressed="true"].down { background: #4a1e1e; border-color: #c45c5c; color: #f0a0a0; }
.comment { margin: 0; padding: 8px 12px; font-size: 13px; font-weight: 400; color: #f0a0a0; background: #241616; border-bottom: 1px solid #2a2b2e; }
.pair { display: grid; grid-template-columns: 1fr 1fr; }
.pair.triple { grid-template-columns: 1fr 1fr 1fr; }
.col + .col { border-left: 1px solid #2a2b2e; }
.col-head { padding: 6px 10px; font-size: 11px; letter-spacing: .04em; text-transform: uppercase; }
.col-head.wowhead { background: #4a3a12; color: #e6c35a; }
.col-head.actual { background: #2e3a52; color: #9ec1ff; }
.col-head.expected { background: #1e4a28; color: #8ee0a0; }
.shot { display: block; width: 100%; padding: 0; border: 0; background: #0b0c0d; cursor: pointer; }
.shot:hover { outline: 2px solid #5b8def; outline-offset: -2px; }
.shot img { display: block; width: 100%; height: auto; }
.badge { flex: none; font-size: 11px; letter-spacing: .04em; text-transform: uppercase; padding: 1px 6px; border-radius: 4px; }
.badge.retail { background: #1d3a6e; color: #9ec1ff; }
.badge.mount { background: #4a3a12; color: #e6c35a; }
.badge.classic { background: #1e4a28; color: #8ee0a0; }
.name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.detail { font-size: 12px; font-weight: 400; color: #f0a0a0; }
.lightbox { display: none; position: fixed; inset: 0; z-index: 20; background: #000c; }
.lightbox.open { display: flex; align-items: center; justify-content: center; }
.lb-stack { display: flex; flex-direction: column; align-items: center; }
.lb-frame { display: table; margin: 0; }
.lightbox img { display: block; max-width: calc(100vw - 96px); max-height: calc(100vh - 200px); object-fit: contain; background: #000; }
.nav { position: absolute; top: 50%; transform: translateY(-50%); width: 48px; height: 72px; border: 0; background: #0008; color: #fff; font-size: 36px; cursor: pointer; }
.nav:hover { background: #000c; }
.prev { left: 8px; }
.next { right: 8px; }
.lb-head { display: table-caption; caption-side: top; padding: 0 0 8px; box-sizing: border-box; }
.lb-head-row { display: flex; align-items: center; gap: 10px; min-width: 0; }
.lb-kind { flex: none; font-size: 12px; letter-spacing: .06em; text-transform: uppercase; padding: 2px 8px; border-radius: 4px; font-weight: 700; }
.lb-kind.wowhead { background: #4a3a12; color: #e6c35a; }
.lb-kind.actual { background: #2e3a52; color: #9ec1ff; }
.lb-kind.expected { background: #1e4a28; color: #8ee0a0; }
.lb-name { font-size: 15px; font-weight: 650; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
.lb-count { flex: none; margin-left: auto; color: #8a8a90; font-size: 13px; }
.lb-base { margin-top: 4px; font-size: 12px; color: #8a8a90; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.lb-base a { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #5b8def; text-decoration: none; }
.lb-base a:hover { text-decoration: underline; }
.lb-votes { gap: 24px; margin-top: 16px; }
.lb-votes button { width: 88px; height: 88px; font-size: 44px; border-radius: 18px; border-width: 2px; }
.close { position: absolute; top: 8px; right: 8px; z-index: 1; width: 40px; height: 40px; border: 0; background: #0008; color: #fff; font-size: 24px; cursor: pointer; }
.empty { padding: 48px 16px; text-align: center; color: #8a8a90; }
dialog { border: 1px solid #2a2b2e; border-radius: 8px; background: #1a1b1e; color: #e8e8ea; padding: 16px; min-width: min(420px, 90vw); }
dialog p { margin: 0 0 8px; }
dialog input { width: 100%; margin: 0 0 12px; padding: 8px; background: #111213; color: #e8e8ea; border: 1px solid #33343a; border-radius: 6px; }
dialog menu { display: flex; justify-content: flex-end; gap: 8px; margin: 0; padding: 0; }
dialog button { background: #1c1d20; color: #c8c8cc; border: 1px solid #33343a; border-radius: 6px; padding: 6px 12px; cursor: pointer; }
dialog button[value="ok"] { background: #4a1e1e; border-color: #c45c5c; color: #fff; }
</style>
</head>
<body>
<header>
  <label class="status">
    <select id="status" aria-label="Snapshot outcome"></select>
  </label>
  <div class="filters" id="filters"></div>
  <div class="vote-filters" id="vote-filters"></div>
</header>
<main class="grid" id="grid"></main>
<div class="lightbox" id="lightbox" hidden>
  <button class="close" type="button" aria-label="Close">×</button>
  <button class="nav prev" type="button" aria-label="Previous image">‹</button>
  <div class="lb-stack">
    <div class="lb-frame">
      <div class="lb-head">
        <div class="lb-head-row">
          <span class="lb-kind" id="lb-kind"></span>
          <span class="lb-name" id="lb-name"></span>
          <span class="lb-count" id="count"></span>
        </div>
        <div class="lb-base" id="lb-base" hidden></div>
      </div>
      <img alt="">
    </div>
    <span class="votes lb-votes" id="lb-votes"></span>
  </div>
  <button class="nav next" type="button" aria-label="Next image">›</button>
</div>
<dialog id="comment-dialog">
  <form method="dialog">
    <p id="comment-label">Why is this bad?</p>
    <input id="comment-input" name="comment" autocomplete="off">
    <menu>
      <button type="button" value="cancel" id="comment-cancel">Cancel</button>
      <button value="ok">Save</button>
    </menu>
  </form>
</dialog>
<script>
const CASES = ${JSON.stringify(cases)};
const VOTE_KEY = "snapshot-review-votes";
const filtersEl = document.getElementById("filters");
const voteFiltersEl = document.getElementById("vote-filters");
const statusEl = document.getElementById("status");
const grid = document.getElementById("grid");
const box = document.getElementById("lightbox");
const img = box.querySelector("img");
const lbKind = document.getElementById("lb-kind");
const lbName = document.getElementById("lb-name");
const lbBase = document.getElementById("lb-base");
const lbVotes = document.getElementById("lb-votes");
const count = document.getElementById("count");
const dialog = document.getElementById("comment-dialog");
const commentInput = document.getElementById("comment-input");
const commentLabel = document.getElementById("comment-label");
let filter = "all";
let voteFilter = "all";
let outcome = "failed";
let index = 0;
let pendingBad = "";
const votes = loadVotes();

function loadVotes() {
  try {
    const parsed = JSON.parse(localStorage.getItem(VOTE_KEY) || "{}");
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function saveVotes() {
  localStorage.setItem(VOTE_KEY, JSON.stringify(votes));
}

function voteOf(name) {
  return votes[name] || null;
}

function statusCases() {
  return CASES.filter((item) => (outcome === "failed" ? !item.ok : item.ok));
}

function visibleCases() {
  return statusCases().filter((item) => {
    if (filter !== "all" && item.suite !== filter) return false;
    const vote = voteOf(item.name)?.vote;
    if (voteFilter === "good") return vote === "good";
    if (voteFilter === "bad") return vote === "bad";
    return true;
  });
}

function slides() {
  const out = [];
  for (const item of visibleCases()) {
    if (item.wowheadSrc) out.push({ name: item.name, kind: "wowhead", src: item.wowheadSrc });
    out.push({ name: item.name, kind: "expected", src: item.expectedSrc });
    out.push({ name: item.name, kind: "actual", src: item.actualSrc });
  }
  return out;
}

function counts() {
  let good = 0, bad = 0;
  for (const item of statusCases()) {
    const vote = voteOf(item.name)?.vote;
    if (vote === "good") good += 1;
    if (vote === "bad") bad += 1;
  }
  return { good, bad };
}

function chip(label, pressed, onClick) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.textContent = label;
  btn.setAttribute("aria-pressed", String(pressed));
  btn.addEventListener("click", onClick);
  return btn;
}

function renderFilters() {
  const failed = CASES.filter((item) => !item.ok).length;
  const passed = CASES.filter((item) => item.ok).length;
  const failedOpt = document.createElement("option");
  failedOpt.value = "failed";
  failedOpt.textContent = "Failed snapshots (" + failed + ")";
  const passedOpt = document.createElement("option");
  passedOpt.value = "successful";
  passedOpt.textContent = "Successful snapshots (" + passed + ")";
  statusEl.replaceChildren(failedOpt, passedOpt);
  statusEl.value = outcome;
  const pool = statusCases();
  const suites = ["all", ...new Set(pool.map((item) => item.suite))];
  if (filter !== "all" && !suites.includes(filter)) filter = "all";
  filtersEl.replaceChildren(...suites.map((suite) => {
    const n = suite === "all" ? pool.length : pool.filter((item) => item.suite === suite).length;
    return chip(suite === "all" ? "All (" + n + ")" : suite + " (" + n + ")", suite === filter, () => {
      filter = suite;
      render();
    });
  }));
  const n = counts();
  voteFiltersEl.replaceChildren(
    chip("Good (" + n.good + ")", voteFilter === "good", () => { voteFilter = voteFilter === "good" ? "all" : "good"; render(); }),
    chip("Bad (" + n.bad + ")", voteFilter === "bad", () => { voteFilter = voteFilter === "bad" ? "all" : "bad"; render(); }),
  );
}

function voteButtons(name) {
  const wrap = document.createElement("span");
  wrap.className = "votes";
  const vote = voteOf(name);
  const up = document.createElement("button");
  up.type = "button";
  up.className = "up";
  up.textContent = "👍";
  up.title = "Good";
  up.setAttribute("aria-pressed", String(vote?.vote === "good"));
  up.addEventListener("click", (e) => { e.stopPropagation(); markGood(name); });
  const down = document.createElement("button");
  down.type = "button";
  down.className = "down";
  down.textContent = "👎";
  down.title = "Bad, with comment";
  down.setAttribute("aria-pressed", String(vote?.vote === "bad"));
  down.addEventListener("click", (e) => { e.stopPropagation(); markBad(name); });
  wrap.append(up, copyButton(), down);
  return wrap;
}

function copyButton() {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "copy";
  btn.textContent = "📋";
  btn.title = "Copy results";
  btn.setAttribute("aria-label", "Copy results");
  btn.addEventListener("click", (e) => { e.stopPropagation(); copyResults(); });
  return btn;
}

function shotButton(src, kind, name, onClick) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "shot";
  const image = document.createElement("img");
  image.src = src;
  image.alt = name + " " + kind;
  image.loading = "lazy";
  btn.append(image);
  btn.addEventListener("click", onClick);
  return btn;
}

function column(item, kind, src, list) {
  const col = document.createElement("div");
  col.className = "col";
  const head = document.createElement("div");
  head.className = "col-head " + kind;
  head.textContent = kind;
  col.append(head, shotButton(src, kind, item.name, () => {
    openAt(list.findIndex((slide) => slide.name === item.name && slide.kind === kind));
  }));
  return col;
}

function render() {
  renderFilters();
  const items = visibleCases();
  const list = slides();
  if (items.length === 0) {
    const empty = document.createElement("p");
    empty.className = "empty";
    empty.textContent = CASES.length === 0
      ? "No cases with actual and expected images."
      : statusCases().length === 0
        ? (outcome === "failed" ? "No failed cases." : "No successful cases.")
        : "No cases in this filter.";
    grid.replaceChildren(empty);
    if (box.classList.contains("open")) syncLightbox();
    return;
  }
  grid.replaceChildren(...items.map((item) => {
    const vote = voteOf(item.name);
    const card = document.createElement("figure");
    card.className = "card" + (vote?.vote === "good" ? " good" : vote?.vote === "bad" ? " bad" : "");
    const title = document.createElement("h2");
    const badge = document.createElement("span");
    badge.className = "badge " + item.suite;
    badge.textContent = item.suite;
    const name = document.createElement("span");
    name.className = "name";
    name.textContent = item.name;
    title.append(badge, name);
    if (item.detail) {
      const detail = document.createElement("span");
      detail.className = "detail";
      detail.textContent = item.detail;
      title.append(detail);
    }
    title.append(voteButtons(item.name));
    const pair = document.createElement("div");
    pair.className = "pair" + (item.wowheadSrc ? " triple" : "");
    if (item.wowheadSrc) pair.append(column(item, "wowhead", item.wowheadSrc, list));
    pair.append(column(item, "expected", item.expectedSrc, list), column(item, "actual", item.actualSrc, list));
    card.append(title);
    if (vote?.vote === "bad" && vote.comment) {
      const note = document.createElement("p");
      note.className = "comment";
      note.textContent = vote.comment;
      card.append(note);
    }
    card.append(pair);
    return card;
  }));
  if (box.classList.contains("open")) syncLightbox();
}

function markGood(name) {
  if (voteOf(name)?.vote === "good") delete votes[name];
  else votes[name] = { vote: "good" };
  saveVotes();
  render();
}

function markBad(name) {
  pendingBad = name;
  commentLabel.textContent = "Why is " + name + " bad?";
  commentInput.value = voteOf(name)?.comment || "";
  dialog.showModal();
  commentInput.focus();
  commentInput.select();
}

document.getElementById("comment-cancel").addEventListener("click", () => dialog.close("cancel"));
dialog.addEventListener("close", () => {
  const name = pendingBad;
  pendingBad = "";
  if (dialog.returnValue !== "ok" || name === "") return;
  const comment = commentInput.value.trim();
  if (comment === "") return;
  votes[name] = { vote: "bad", comment };
  saveVotes();
  render();
});

function resultsText() {
  const good = [];
  const bad = [];
  for (const item of CASES) {
    const vote = voteOf(item.name);
    if (vote?.vote === "good") good.push("- " + item.name);
    if (vote?.vote === "bad") bad.push("- " + item.name + (vote.comment ? ": " + vote.comment : ""));
  }
  return "Good:\\n" + (good.join("\\n") || "- none") + "\\n\\nBad:\\n" + (bad.join("\\n") || "- none");
}

async function copyResults() {
  const text = resultsText();
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    commentInput.value = text;
  }
}

function syncLightbox() {
  const items = slides();
  if (items.length === 0) { close(); return; }
  index = ((index % items.length) + items.length) % items.length;
  const item = items[index];
  const same = items.filter((slide) => slide.name === item.name);
  img.src = item.src;
  img.alt = item.kind + " " + item.name;
  lbKind.className = "lb-kind " + item.kind;
  lbKind.textContent = item.kind;
  lbName.textContent = item.name;
  count.textContent = (same.findIndex((slide) => slide.kind === item.kind) + 1) + " / " + same.length;
  fillBase(CASES.find((c) => c.name === item.name)?.base || "");
  lbVotes.replaceChildren(...voteButtons(item.name).children);
  if (img.complete) fitLightboxHead();
  else img.addEventListener("load", fitLightboxHead, { once: true });
}

function fitLightboxHead() {
  const head = box.querySelector(".lb-head");
  const w = img.clientWidth;
  head.style.width = w > 0 ? w + "px" : "";
}

function shiftKind(delta) {
  const items = slides();
  const cur = items[index];
  if (!cur) return;
  const same = [];
  for (let i = 0; i < items.length; i++) {
    if (items[i].name === cur.name) same.push(i);
  }
  const at = same.indexOf(index);
  openAt(same[(at + delta + same.length) % same.length]);
}

function shiftCase(delta) {
  const cases = visibleCases();
  const items = slides();
  const cur = items[index];
  if (!cur || cases.length === 0) return;
  const ci = cases.findIndex((item) => item.name === cur.name);
  const next = cases[(ci + delta + cases.length) % cases.length];
  let i = items.findIndex((slide) => slide.name === next.name && slide.kind === "wowhead");
  if (i < 0) i = items.findIndex((slide) => slide.name === next.name);
  openAt(i);
}

function fillBase(base) {
  lbBase.replaceChildren();
  if (!base) { lbBase.hidden = true; return; }
  lbBase.hidden = false;
  lbBase.title = base;
  if (/^https?:\\/\\//i.test(base)) {
    const a = document.createElement("a");
    a.href = base;
    a.target = "_blank";
    a.rel = "noreferrer";
    a.textContent = base;
    lbBase.append(a);
  } else {
    lbBase.textContent = base;
  }
}

function openAt(i) {
  const items = slides();
  if (items.length === 0) return;
  index = (i + items.length) % items.length;
  box.classList.add("open");
  box.hidden = false;
  syncLightbox();
}

function close() {
  box.classList.remove("open");
  box.hidden = true;
}

box.querySelector(".prev").addEventListener("click", (e) => { e.stopPropagation(); shiftKind(-1); });
box.querySelector(".next").addEventListener("click", (e) => { e.stopPropagation(); shiftKind(1); });
box.querySelector(".close").addEventListener("click", (e) => { e.stopPropagation(); close(); });
box.addEventListener("click", (e) => { if (e.target === box) close(); });
img.addEventListener("click", (e) => e.stopPropagation());
img.addEventListener("load", fitLightboxHead);
window.addEventListener("resize", () => { if (box.classList.contains("open")) fitLightboxHead(); });
box.querySelector(".lb-stack").addEventListener("click", (e) => e.stopPropagation());
document.addEventListener("keydown", (e) => {
  if (dialog.open) return;
  if (!box.classList.contains("open")) return;
  if (e.key === "Escape") close();
  if (e.key === "ArrowLeft") { e.preventDefault(); shiftKind(-1); }
  if (e.key === "ArrowRight") { e.preventDefault(); shiftKind(1); }
  if (e.key === "ArrowUp") { e.preventDefault(); shiftCase(-1); }
  if (e.key === "ArrowDown") { e.preventDefault(); shiftCase(1); }
});
statusEl.addEventListener("change", () => {
  outcome = statusEl.value === "successful" ? "successful" : "failed";
  filter = "all";
  voteFilter = "all";
  render();
});
render();
</script>
</body>
</html>
`;
}

if (import.meta.main) {
  writeReview(loadCliCases());
  process.exit(0);
}
