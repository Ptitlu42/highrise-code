import { spawnSync } from "node:child_process";
import { mkdirSync, readdirSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { chromium } from "playwright";

const bin = resolve(process.env.HR_BIN || "bin/highrise-code");
const target = resolve(process.env.HR_TARGET || ".");
const out_dir = resolve(process.env.OUT_DIR || "showcase_out");
const work_dir = join(out_dir, ".work");
const page_file = join(work_dir, "city.html");
const viewport = { width: 1280, height: 720 };
const gif_width = 880;
const gif_fps = 12;
const stage_top = 46;
const canvas_center = { x: viewport.width / 2, y: stage_top + (viewport.height - stage_top) / 2 };

const sleep = (ms) => new Promise((done) => setTimeout(done, ms));

function run(command, args) {
  const result = spawnSync(command, args, { stdio: ["ignore", "inherit", "inherit"] });
  if (result.status !== 0) {
    throw new Error(command + " failed with status " + result.status);
  }
}

async function move_to(page, x, y, steps = 12, delay = 16) {
  const start = page.__pointer || { x, y };
  for (let i = 1; i <= steps; i++) {
    const t = i / steps;
    await page.mouse.move(start.x + (x - start.x) * t, start.y + (y - start.y) * t);
    await sleep(delay);
  }
  page.__pointer = { x, y };
}

async function drag(page, from, to, steps = 40, delay = 16, button = "left") {
  await move_to(page, from.x, from.y, 4, 8);
  await page.mouse.down({ button });
  await move_to(page, to.x, to.y, steps, delay);
  await page.mouse.up({ button });
}

async function zoom(page, at, amount, steps = 14, delay = 40) {
  await move_to(page, at.x, at.y, 6, 10);
  for (let i = 0; i < steps; i++) {
    await page.mouse.wheel(0, amount / steps);
    await sleep(delay);
  }
}

async function set_mode(page, mode) {
  await page.click('[data-mode="' + mode + '"]');
  await sleep(500);
}

async function hide_panel(page) {
  await page.click("#toggle_panel");
  await sleep(500);
}

async function scene_3d(page) {
  await hide_panel(page);
  await set_mode(page, "3d");
  await sleep(700);
  await drag(page, { x: 420, y: 520 }, { x: 760, y: 470 }, 60, 16);
  await sleep(300);
  await drag(page, { x: 760, y: 470 }, { x: 560, y: 560 }, 50, 16);
  await sleep(300);
  await zoom(page, { x: canvas_center.x - 80, y: canvas_center.y + 20 }, -900);
  await sleep(900);
  await drag(page, { x: 600, y: 500 }, { x: 760, y: 500 }, 40, 16);
  await sleep(800);
}

async function scene_iso(page) {
  await hide_panel(page);
  await set_mode(page, "iso");
  await sleep(700);
  await drag(page, { x: 700, y: 400 }, { x: 560, y: 340 }, 40, 16);
  await sleep(300);
  await zoom(page, { x: canvas_center.x - 120, y: canvas_center.y - 40 }, -1100);
  await sleep(900);
  await drag(page, { x: 600, y: 420 }, { x: 760, y: 360 }, 40, 16);
  await sleep(700);
  await zoom(page, { x: canvas_center.x, y: canvas_center.y }, 1400, 18, 40);
  await sleep(800);
}

async function scene_2d(page) {
  await hide_panel(page);
  await set_mode(page, "2d");
  await sleep(700);
  await zoom(page, { x: canvas_center.x - 150, y: canvas_center.y - 110 }, -1000);
  await sleep(1000);
  await drag(page, { x: 800, y: 300 }, { x: 560, y: 460 }, 40, 16);
  await sleep(900);
  await zoom(page, { x: canvas_center.x, y: canvas_center.y }, -700);
  await sleep(1000);
  await zoom(page, { x: canvas_center.x, y: canvas_center.y }, 1900, 20, 40);
  await sleep(800);
}

async function scene_colors(page) {
  await hide_panel(page);
  await set_mode(page, "3d");
  await drag(page, { x: 500, y: 520 }, { x: 640, y: 500 }, 30, 16);
  await sleep(700);
  for (const value of ["depth", "size", "language"]) {
    await page.selectOption("#color_by", value);
    await sleep(900);
  }
  for (const value of ["code", "size", "flat", "lines"]) {
    await page.selectOption("#height_by", value);
    await sleep(900);
  }
  const legend_item = page.locator("#legend button, #legend .legend_item").nth(1);
  if ((await legend_item.count()) > 0) {
    await legend_item.click();
    await sleep(1100);
    await legend_item.click();
    await sleep(700);
  }
}

async function scene_search(page) {
  await set_mode(page, "iso");
  await sleep(600);
  await page.click("#search");
  await page.keyboard.type("renderer", { delay: 110 });
  await sleep(1000);
  await page.locator(".search_item").first().click();
  await sleep(1800);
  await page.click("#reset_view");
  await sleep(900);
  await page.click("#search", { clickCount: 3 });
  await page.keyboard.type("scan", { delay: 110 });
  await sleep(900);
  await page.locator(".search_item").first().click();
  await sleep(1800);
  await page.click("#reset_view");
  await sleep(1200);
}

const scenes = [
  { name: "city_3d", run: scene_3d },
  { name: "city_iso", run: scene_iso },
  { name: "city_2d", run: scene_2d },
  { name: "colors", run: scene_colors },
  { name: "search", run: scene_search },
];

async function open_page(browser, record_dir) {
  const context = await browser.newContext({
    viewport,
    deviceScaleFactor: 1,
    recordVideo: record_dir ? { dir: record_dir, size: viewport } : undefined,
  });
  const created = Date.now();
  const page = await context.newPage();
  await page.goto("file://" + page_file);
  await page.waitForFunction(() => {
    const status = document.getElementById("status");
    return status && /items/.test(status.textContent);
  });
  await page.mouse.move(canvas_center.x, canvas_center.y);
  page.__pointer = { x: canvas_center.x, y: canvas_center.y };
  await sleep(600);
  return { context, page, offset: (Date.now() - created) / 1000 };
}

function to_gif(video, offset, gif) {
  const filter =
    "fps=" + gif_fps + ",scale=" + gif_width + ":-1:flags=lanczos,split[a][b];" +
    "[a]palettegen=max_colors=96:stats_mode=diff[p];" +
    "[b][p]paletteuse=dither=none:diff_mode=rectangle";
  run("ffmpeg", ["-y", "-v", "error", "-ss", offset.toFixed(2), "-i", video, "-vf", filter, "-loop", "0", gif]);
}

async function record_scene(browser, scene) {
  const record_dir = join(work_dir, scene.name);
  rmSync(record_dir, { recursive: true, force: true });
  mkdirSync(record_dir, { recursive: true });
  const { context, page, offset } = await open_page(browser, record_dir);
  await scene.run(page);
  await page.close();
  await context.close();
  const video = join(record_dir, readdirSync(record_dir).find((name) => name.endsWith(".webm")));
  to_gif(video, offset, join(out_dir, scene.name + ".gif"));
  console.log("recorded " + scene.name);
}

async function capture_stats(browser) {
  const { context, page } = await open_page(browser, null);
  await set_mode(page, "iso");
  await page.click("#search");
  await page.keyboard.type("main.js", { delay: 40 });
  await sleep(700);
  await page.locator(".search_item").first().click();
  await sleep(1500);
  await page.click("#reset_view");
  await sleep(900);
  await page.screenshot({ path: join(out_dir, "stats.png") });
  await context.close();
  console.log("captured stats");
}

async function main() {
  rmSync(out_dir, { recursive: true, force: true });
  mkdirSync(work_dir, { recursive: true });
  run(bin, ["-q", "--out", page_file, "-i", "node_modules", target]);
  const browser = await chromium.launch({
    args: ["--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist"],
  });
  for (const scene of scenes) {
    await record_scene(browser, scene);
  }
  await capture_stats(browser);
  await browser.close();
  rmSync(work_dir, { recursive: true, force: true });
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
