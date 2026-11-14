import { clear, el } from "hr/dom";
import {
  format_bytes,
  format_duration,
  format_float,
  format_int,
  format_percent,
} from "hr/format";

const max_depth_rows = 24;
const max_warning_rows = 1000;

function row(label, value, title) {
  const node = el("div", "kv_row");
  const key = el("span", "kv_key", label);
  const val = el("span", "kv_value", value);
  if (title) {
    val.title = title;
  }
  node.append(key, val);
  return node;
}

function section(title, open) {
  const details = el("details", "section");
  details.open = open;
  details.append(el("summary", "section_title", title));
  const body = el("div", "section_body");
  details.append(body);
  return { details, body };
}

function stacked_bar(parts) {
  const bar = el("div", "stacked_bar");
  for (const part of parts) {
    const segment = el("span", "stacked_segment " + part.class_name);
    segment.style.width = Math.max(part.percent, part.percent > 0 ? 0.5 : 0) + "%";
    segment.title = part.label + ": " + format_percent(part.percent);
    bar.append(segment);
  }
  return bar;
}

function legend_row(parts) {
  const node = el("div", "bar_legend");
  for (const part of parts) {
    const item = el("span", "bar_legend_item");
    item.append(el("i", "dot " + part.class_name));
    item.append(document.createTextNode(part.label + " " + format_int(part.count) + " (" + format_percent(part.percent) + ")"));
    node.append(item);
  }
  return node;
}

function share_parts(label_a, a, label_b, b, label_c, c) {
  return [
    { label: label_a, class_name: "seg_code", count: a.count, percent: a.percent },
    { label: label_b, class_name: "seg_comment", count: b.count, percent: b.percent },
    { label: label_c, class_name: "seg_blank", count: c.count, percent: c.percent },
  ];
}

function distribution(buckets) {
  const wrap = el("div", "histogram");
  const peak = Math.max(1, ...buckets.map((b) => b.files));
  for (const bucket of buckets) {
    const line = el("div", "hist_row");
    line.append(el("span", "hist_label", bucket.label));
    const track = el("span", "hist_track");
    const fill = el("span", "hist_fill");
    fill.style.width = (bucket.files / peak) * 100 + "%";
    track.append(fill);
    line.append(track);
    line.append(el("span", "hist_count", format_int(bucket.files) + " \u00b7 " + format_percent(bucket.percent)));
    wrap.append(line);
  }
  return wrap;
}

function summary_table(entries) {
  const table = el("table", "table compact");
  const head = el("thead");
  const head_row = el("tr");
  for (const label of ["", "mean", "median", "p90", "p99", "max"]) {
    head_row.append(el("th", "", label));
  }
  head.append(head_row);
  table.append(head);
  const body = el("tbody");
  for (const entry of entries) {
    const tr = el("tr");
    tr.append(el("th", "", entry.label));
    const s = entry.value;
    const fmt = entry.format || format_float;
    tr.append(el("td", "", fmt(s.mean)));
    tr.append(el("td", "", fmt(s.median)));
    if (s.p90 === undefined) {
      tr.append(el("td", "", "\u2013"), el("td", "", "\u2013"), el("td", "", "\u2013"));
    } else {
      tr.append(el("td", "", fmt(s.p90)), el("td", "", fmt(s.p99)), el("td", "", fmt(s.max)));
    }
    body.append(tr);
  }
  table.append(body);
  return table;
}

export function build_language_table(report, palette, on_pick) {
  const wrap = el("div", "table_scroll");
  const table = el("table", "table languages");
  const columns = [
    ["", null],
    ["Language", "name"],
    ["Files", "files"],
    ["Lines", "lines"],
    ["Code", "code"],
    ["Comment", "comment"],
    ["Blank", "blank"],
    ["Chars", "chars"],
    ["Bytes", "bytes"],
    ["% lines", "percent_lines"],
    ["% chars", "percent_chars"],
    ["% code", "percent_code"],
    ["Avg lines/file", "average_lines_per_file"],
    ["Avg chars/file", "average_chars_per_file"],
  ];
  const head = el("thead");
  const head_row = el("tr");
  for (const [label] of columns) {
    head_row.append(el("th", "", label));
  }
  head.append(head_row);
  table.append(head);
  const body = el("tbody");
  const rows = new Map();
  for (const lang of report.languages) {
    const tr = el("tr", "lang_row");
    tr.tabIndex = 0;
    const swatch = el("td", "swatch_cell");
    const dot = el("i", "swatch");
    dot.style.backgroundColor = palette.css[lang.index];
    swatch.append(dot);
    tr.append(swatch);
    tr.append(el("td", "lang_name", lang.name));
    tr.append(el("td", "num", format_int(lang.files)));
    tr.append(el("td", "num", format_int(lang.lines)));
    tr.append(el("td", "num", format_int(lang.code)));
    tr.append(el("td", "num", format_int(lang.comment)));
    tr.append(el("td", "num", format_int(lang.blank)));
    tr.append(el("td", "num", format_int(lang.chars)));
    tr.append(el("td", "num", format_bytes(lang.bytes)));
    tr.append(el("td", "num", format_percent(lang.percent_lines)));
    tr.append(el("td", "num", format_percent(lang.percent_chars)));
    tr.append(el("td", "num", format_percent(lang.percent_code)));
    tr.append(el("td", "num", format_float(lang.average_lines_per_file)));
    tr.append(el("td", "num", format_float(lang.average_chars_per_file)));
    tr.title = lang.category;
    const activate = () => on_pick(lang.index);
    tr.addEventListener("click", activate);
    tr.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        activate();
      }
    });
    rows.set(lang.index, tr);
    body.append(tr);
  }
  table.append(body);
  wrap.append(table);
  return {
    node: wrap,
    mark(index) {
      for (const [i, tr] of rows) {
        tr.classList.toggle("active", i === index);
      }
    },
  };
}

export function build_panel(container, summary, palette, on_language) {
  clear(container);
  const report = summary.report;
  const selection = el("div", "selection_card hidden");
  container.append(selection);

  const overview = section("Overview", true);
  overview.body.append(
    row("Root", summary.root),
    row("Files", format_int(report.files)),
    row("Directories", format_int(report.dirs)),
    row("Size", report.bytes_iec, format_int(report.bytes) + " bytes"),
    row("Lines", format_int(report.lines.total)),
    row("Characters", format_int(report.characters.total)),
    row("Non-whitespace chars", format_int(report.characters.non_whitespace) + " (" + format_percent(report.characters.non_whitespace_percent) + ")"),
    row("Empty files", format_int(report.empty_files)),
    row("Scan time", format_duration(report.scan_seconds)),
  );
  if (report.truncated) {
    overview.body.append(el("div", "notice", "Scan truncated by --max-files."));
  }
  container.append(overview.details);

  const lines = section("Lines", true);
  const line_parts = share_parts("Code", report.lines.code, "Comments", report.lines.comment, "Blank", report.lines.blank);
  lines.body.append(stacked_bar(line_parts), legend_row(line_parts));
  lines.body.append(row("Comment / code ratio", format_float(report.lines.comment_code_ratio)));
  container.append(lines.details);

  const chars = section("Characters", true);
  const char_parts = share_parts("Code", report.characters.code, "Comments", report.characters.comment, "Blank", report.characters.blank);
  chars.body.append(stacked_bar(char_parts), legend_row(char_parts));
  chars.body.append(row("Average per line", format_float(report.characters.average_per_line)));
  container.append(chars.details);

  const per_file = section("Per-file statistics", false);
  per_file.body.append(
    summary_table([
      { label: "Lines", value: report.lines_per_file },
      { label: "Chars", value: report.chars_per_file },
      { label: "Bytes", value: report.size_per_file, format: format_bytes },
    ]),
  );
  container.append(per_file.details);

  const lengths = section("File length distribution", false);
  lengths.body.append(el("div", "subtitle", "Lines per file"), distribution(report.lines_distribution));
  lengths.body.append(el("div", "subtitle", "Characters per file"), distribution(report.chars_distribution));
  container.append(lengths.details);

  const depth = section("Depth", false);
  depth.body.append(row("Maximum", format_int(report.depth.max)), row("Average", format_float(report.depth.mean)));
  const levels = report.depth.files_by_level.slice(1, 1 + max_depth_rows).map((count, i) => ({
    label: "level " + (i + 1),
    files: count,
    percent: report.files ? Math.round((count * 10000) / report.files) / 100 : 0,
  }));
  depth.body.append(distribution(levels));
  if (report.depth.files_by_level.length - 1 > max_depth_rows) {
    depth.body.append(el("div", "notice", "Deeper levels omitted."));
  }
  container.append(depth.details);

  const endings = section("Line endings", false);
  endings.body.append(
    row("LF only", format_int(report.line_endings.lf_only)),
    row("CRLF only", format_int(report.line_endings.crlf_only)),
    row("Mixed", format_int(report.line_endings.mixed)),
    row("No final newline", format_int(report.line_endings.no_final_newline)),
  );
  container.append(endings.details);

  const languages = section("Languages", true);
  const table = build_language_table(report, palette, on_language);
  languages.body.append(table.node);
  container.append(languages.details);

  const categories = section("Categories", false);
  for (const category of report.categories) {
    categories.body.append(
      row(category.name, format_int(category.files) + " files \u00b7 " + format_int(category.lines) + " lines \u00b7 " + format_percent(category.percent_lines)),
    );
  }
  container.append(categories.details);

  const largest = section("Largest files", false);
  largest.body.append(el("div", "subtitle", "By lines"));
  for (const file of report.largest_files_by_lines) {
    largest.body.append(row(file.path, format_int(file.lines) + " lines", file.language));
  }
  largest.body.append(el("div", "subtitle", "By size"));
  for (const file of report.largest_files_by_bytes) {
    largest.body.append(row(file.path, format_bytes(file.bytes), file.language));
  }
  container.append(largest.details);

  const skipped = section("Skipped", false);
  const s = report.skipped;
  skipped.body.append(
    row("Symlinks", format_int(s.symlinks)),
    row("Special files", format_int(s.special_files)),
    row("Binary files", format_int(s.binary_files)),
    row("Too large", format_int(s.too_large)),
    row("Ignored by rules", format_int(s.ignored)),
    row("Hidden", format_int(s.hidden)),
    row("Unreadable", format_int(s.unreadable)),
    row("Depth limit", format_int(s.depth_limit)),
  );
  container.append(skipped.details);

  const warnings = section("Warnings (" + format_int(summary.warnings_total) + ")", false);
  const shown = summary.warnings.slice(0, max_warning_rows);
  for (const warning of shown) {
    const item = el("div", "warning_row");
    item.append(el("span", "warning_path", warning.path || "."), el("span", "warning_message", warning.message));
    warnings.body.append(item);
  }
  if (summary.warnings_total > shown.length) {
    warnings.body.append(el("div", "notice", format_int(summary.warnings_total - shown.length) + " more warnings not retained."));
  }
  if (summary.warnings_total === 0) {
    warnings.body.append(el("div", "notice", "No warnings."));
  }
  container.append(warnings.details);

  return { selection, language_table: table };
}

export function render_selection(card, detail, palette) {
  clear(card);
  if (!detail) {
    card.classList.add("hidden");
    return;
  }
  card.classList.remove("hidden");
  const title = el("div", "selection_title");
  const swatch = el("i", "swatch");
  if (detail.language !== undefined && detail.kind !== "dir") {
    swatch.style.backgroundColor = palette.css[detail.lang_index] || "transparent";
  }
  title.append(swatch, el("span", "selection_name", detail.name));
  card.append(title, el("div", "selection_path", detail.path || "."));
  const rows = [
    row("Kind", detail.kind),
    row("Depth", format_int(detail.depth)),
  ];
  if (detail.kind === "file" || detail.kind === "aggregate" || detail.language) {
    rows.push(row(detail.kind === "dir" ? "Main language" : "Language", detail.language + " (" + detail.category + ")"));
  }
  if (detail.kind !== "file") {
    rows.push(row("Files", format_int(detail.files)), row("Subdirectories", format_int(detail.dirs)));
  }
  rows.push(
    row("Size", format_bytes(detail.bytes), format_int(detail.bytes) + " bytes"),
    row("Lines", format_int(detail.lines)),
    row("Code", format_int(detail.code)),
    row("Comments", format_int(detail.comment)),
    row("Blank", format_int(detail.blank)),
    row("Characters", format_int(detail.chars)),
  );
  if (detail.chars_code !== undefined) {
    rows.push(
      row("Code chars", format_int(detail.chars_code)),
      row("Comment chars", format_int(detail.chars_comment)),
      row("Blank chars", format_int(detail.chars_blank)),
    );
  }
  rows.push(
    row("Non-whitespace chars", format_int(detail.chars_non_whitespace)),
    row("Avg chars/line", format_float(detail.average_chars_per_line)),
  );
  card.append(...rows);
  if (detail.languages && detail.languages.length) {
    card.append(el("div", "subtitle", "Languages inside"));
    for (const entry of detail.languages) {
      card.append(row(entry.name, format_int(entry.lines) + " lines \u00b7 " + format_int(entry.files) + " files"));
    }
  }
}
