const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const now = new Date();
const at = (minutes) => new Date(now.getTime() - minutes * 60e3).toISOString();
const account = {
  id: "codex@biz@example.com", provider: "codex", name: "Codex", who: "biz@example.com",
  kind: "account", agent: "codex", model: "gpt-6-astra", routing: "pace",
  known: true, used: 45, pace: 1, due: new Date(now.getTime() + 3 * 36e5).toISOString(),
};
const route = (id, fields) => {
  const r = {
    id, seq: id, time: at(1), agent: "codex", model: "codex/gpt-6-astra", provider: "codex",
    order: [account], done: true, status: 200, ms: 900, tokens: 1200, ...fields,
  };
  r.tries ??= [{ id: r.order[0].id, model: account.model, start: at(1), done: true, status: 200, ms: 900 }];
  return r;
};
const mid = { id: "mid", provider: "mid", name: "MidCode", kind: "provider", model: account.model };
const ok = { id: mid.id, model: mid.model, start: at(1), done: true, status: 200 };
const foreign = { order: [mid], compaction: "5x#123abc", provider: mid.id, model: "mid/" + mid.model };
const recovered = (fail) => ({ ...foreign, tries: [{ ...ok, status: 400, fail }, ok] });
const routes = [
  route(109, foreign),
  route(108, recovered("retry with local checkpoint excerpt")),
  route(107, recovered("retry with writer's checkpoint summary")),
  route(106, recovered("retry with visible history only")),
  route(105, { ...foreign, status: 400, error: "rejected", tries: [{ ...ok, status: 400 }] }),
  route(104, { ...recovered("foreign"), order: [account, mid] }),
  route(102, { compaction: account.id }),
  route(101, {}),
];

function serve(lang) {
  return async (request) => {
    const url = new URL(request.request().url());
    const json = (data) => request.fulfill({ json: data });
    if (url.pathname === "/boot.js") return request.fulfill({
      contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:true};`,
    });
    if (url.pathname === "/wails/runtime.js") return request.fulfill({
      contentType: "text/javascript", body: "export const Window = {};",
    });
    if (url.pathname === "/api/state") return json({
      agents: [{ id: "codex", name: "Codex", path: "/test/config.toml", fields: [] }],
      profiles: [], settings: { lang, theme: "light" },
    });
    if (url.pathname === "/api/gateway/trace") {
      if (url.searchParams.get("wait")) await new Promise((resolve) => setTimeout(resolve, 20e3));
      return json({ mine: true, now: now.toISOString(), seq: 109, totals: { requests: routes.length }, routes });
    }
    if (url.pathname === "/api/gateway/history") return json({ cut: false, days: [], routes: [] });
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname === "/api/plugins") return json({ plugins: [] });
    if (url.pathname === "/api/groups") return json({ groups: [] });
    if (url.pathname === "/api/providers") return json({ providers: [], excluded: [], gateway: { running: true, window: true } });
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await request.fulfill({ body: await fs.readFile(file), contentType });
  };
}

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of ["en", "zh", "zh-TW", "ja", "de"]) {
    test(`${engine} ${lang}: native checkpoint writer, final reader and recovery`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      t.after(() => browser.close());
      const page = await browser.newPage({ viewport: { width: 1100, height: 1200 }, reducedMotion: "reduce" });
      page.setDefaultTimeout(5000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.route("**/*", serve(lang));
      await page.goto("http://magpie.test/?view=routing");
      await page.locator(".rt-req").nth(routes.length - 1).waitFor();
      const native = { en: /native compaction item/, zh: /原生压缩项/, "zh-TW": /原生壓縮項/, ja: /ネイティブ圧縮項目/, de: /Komprimierungselement/ }[lang];
      const why = page.locator(".rt-steps li.why").first();
      const detail = async (i) => {
        await page.locator(".rt-req").nth(i).click();
        // The click schedules the story to render on the next animation frame.
        await page.evaluate(() => new Promise(requestAnimationFrame));
        return (await why.textContent()).replace(/\s+/g, " ").trim();
      };
      const direct = await detail(0);
      assert.match(direct, native);
      assert.match(direct, /5x/);
      assert.match(direct, /MidCode/);
      if (lang === "zh") {
        assert.equal(direct, "本轮携带 5x 生成的原生压缩项。 MidCode 直接使用原压缩项接续。");
        if (process.env.ARTIFACT_DIR) {
          await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
          await page.locator(".rt-steps").screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-checkpoint.png`) });
        }
      }
      const modes = new Set([direct]);
      for (let i = 1; i <= 3; i++) {
        const text = await detail(i);
        assert.match(text, /5x/);
        assert.match(text, /MidCode/);
        if (lang === "zh") assert.match(text, [/本地历史摘录/, /原写入者提供的文字摘要/, /仅使用本轮可见消息/][i-1]);
        modes.add(text);
      }
      assert.equal(modes.size, 4);
      const failed = await detail(4);
      assert.match(failed, /5x/);
      assert.doesNotMatch(failed, /MidCode/);
      assert.equal(await detail(5), direct, "removing only reasoning preserves the native checkpoint");
      assert.match(await detail(6), /biz@example\.com/);
      assert.doesNotMatch(await detail(7), native);
      assert.deepEqual(errors, []);
    });
  }
}
