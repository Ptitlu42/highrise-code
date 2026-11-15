import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

const readme_path = resolve(process.env.README_PATH || "README.md");
const stats_path = resolve(process.env.STATS_PATH || "showcase_out/stats.json");
const commit = (process.env.GITHUB_SHA || "local").slice(0, 7);
const start_marker = "<!-- stats:start -->";
const end_marker = "<!-- stats:end -->";
const bar_width = 20;
const top_languages = 8;
const top_files = 5;

function format_int(value) {
  return new Intl.NumberFormat("en-US").format(value);
}

function badge(label, value, color) {
  const encode = (text) => encodeURIComponent(String(text).replace(/-/g, "--").replace(/_/g, "__"));
  return "![" + label + "](https://img.shields.io/badge/" + encode(label) + "-" + encode(value) + "-" + color + "?style=flat-square)";
}

function bar(percent) {
  const filled = Math.max(percent > 0 ? 1 : 0, Math.round((percent / 100) * bar_width));
  return "`" + "\u2588".repeat(filled) + "\u2591".repeat(bar_width - filled) + "`";
}

function build_block(report) {
  const languages = report.languages.slice(0, top_languages);
  const files = report.largest_files_by_lines.slice(0, top_files);
  const date = new Date().toISOString().slice(0, 16).replace("T", " ") + " UTC";
  const lines = [];
  lines.push(start_marker);
  lines.push("");
  lines.push(
    [
      badge("files", format_int(report.files), "2ea44f"),
      badge("directories", format_int(report.dirs), "1f6feb"),
      badge("lines", format_int(report.lines.total), "8957e5"),
      badge("code", report.lines.code.percent + "%", "d29922"),
      badge("size", report.bytes_iec, "cf222e"),
      badge("languages", report.languages.length, "0e8a16"),
    ].join(" "),
  );
  lines.push("");
  lines.push("| Language | Files | Lines | Share |");
  lines.push("| --- | ---: | ---: | --- |");
  for (const language of languages) {
    lines.push(
      "| " + language.name + " | " + format_int(language.files) + " | " + format_int(language.lines) + " | " +
        bar(language.percent_lines) + " " + language.percent_lines.toFixed(1) + "% |",
    );
  }
  lines.push("");
  lines.push("| Largest files | Language | Lines |");
  lines.push("| --- | --- | ---: |");
  for (const file of files) {
    lines.push("| `" + file.path + "` | " + file.language + " | " + format_int(file.lines) + " |");
  }
  lines.push("");
  lines.push("<sub>Scanned by itself at commit <code>" + commit + "</code> on " + date + ".</sub>");
  lines.push("");
  lines.push(end_marker);
  return lines.join("\n");
}

function main() {
  const report = JSON.parse(readFileSync(stats_path, "utf8")).report;
  const readme = readFileSync(readme_path, "utf8");
  const start = readme.indexOf(start_marker);
  const end = readme.indexOf(end_marker);
  if (start < 0 || end < start) {
    throw new Error("stats markers not found in README");
  }
  const next = readme.slice(0, start) + build_block(report) + readme.slice(end + end_marker.length);
  writeFileSync(readme_path, next);
}

main();
