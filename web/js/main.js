import { load_shader, is_standalone } from "hr/assets";
import {
  animate_to,
  focus_distance,
  reset as reset_camera,
  resize as resize_camera,
  set_mode,
  step_animation,
} from "hr/camera";
import {
  kind_file,
  load_dir_names,
  load_layout,
  load_node,
  load_summary,
  search_nodes,
} from "hr/data";
import { by_id, clear, el } from "hr/dom";
import { format_bytes, format_int } from "hr/format";
import { attach_input } from "hr/input";
import { create_labels } from "hr/labels";
import { build_palette } from "hr/palette";
import { build_panel, render_selection } from "hr/panel";
import { create_renderer, slab_thickness } from "hr/renderer";
import { request_render, set_frame_callback, state } from "hr/state";

const max_dpr = 2;
const detail_cache_limit = 400;
const search_delay_ms = 160;
const hover_detail_delay_ms = 70;
const ramp_css = "linear-gradient(90deg, #2e479e, #29b39e, #f5d147, #eb5c3d)";

const canvas = by_id("view");
const labels_canvas = by_id("labels");
const stage = document.querySelector(".stage");
const tooltip = by_id("tooltip");
const message = by_id("message");
const status_line = by_id("status");
const legend = by_id("legend");
const app = by_id("app");
const search_input = by_id("search");
const search_results = by_id("search_results");
const color_select = by_id("color_by");
const height_select = by_id("height_by");
const depth_input = by_id("max_depth");
const panel_toggle = by_id("toggle_panel");
const mode_buttons = Array.from(document.querySelectorAll(".mode_btn"));

let shader_sources = null;
let panel_refs = null;
let detail_cache = new Map();
let detail_token = 0;
let hover_timer = 0;
let search_timer = 0;
let search_token = 0;

function show_message(text) {
  message.textContent = text;
  message.classList.remove("hidden");
}

function hide_message() {
  message.classList.add("hidden");
}

function set_status(text) {
  status_line.textContent = text;
}

function update_status() {
  if (!state.layout) {
    return;
  }
  const depth_note = state.layout.depth_reached < 256 ? " \u00b7 depth " + state.layout.depth_reached : "";
  set_status(format_int(state.layout.count) + " items" + depth_note);
}

function sync_mode_buttons() {
  for (const button of mode_buttons) {
    const active = button.dataset.mode === state.mode;
    button.classList.toggle("active", active);
    button.setAttribute("aria-pressed", active ? "true" : "false");
  }
}

function frame(now) {
  const animating = step_animation(state.camera, now);
  if (state.renderer && state.layout) {
    state.renderer.draw(state);
    if (state.labels) {
      state.labels.draw(state.camera, state.layout, slab_thickness(state.layout.world, state.camera.mode), state.show_labels);
    }
  }
  if (animating) {
    request_render();
  }
}

function handle_resize() {
  const rect = stage.getBoundingClientRect();
  const width = Math.max(1, Math.floor(rect.width));
  const height = Math.max(1, Math.floor(rect.height));
  const dpr = Math.min(window.devicePixelRatio || 1, max_dpr);
  state.dpr = dpr;
  state.css_width = width;
  state.css_height = height;
  if (state.renderer) {
    state.renderer.resize(Math.round(width * dpr), Math.round(height * dpr));
  }
  if (state.labels) {
    state.labels.resize(width, height, dpr);
  }
  resize_camera(state.camera, width, height);
  request_render();
}

function pick(x, y) {
  if (!state.renderer || !state.layout) {
    return -1;
  }
  return state.renderer.pick_at(state, Math.floor(x * state.dpr), Math.floor(y * state.dpr));
}

async function detail_for(id) {
  const cached = detail_cache.get(id);
  if (cached) {
    return cached;
  }
  const detail = await load_node(state.layout, state.languages, state.max_depth, id);
  if (detail_cache.size >= detail_cache_limit) {
    detail_cache.delete(detail_cache.keys().next().value);
  }
  detail_cache.set(id, detail);
  return detail;
}

function basic_tooltip(id) {
  const layout = state.layout;
  const kind = layout.kind[id];
  const lines = [];
  const kind_label = kind === kind_file ? "file" : kind === 0 ? "directory" : "collapsed directory";
  lines.push(el("div", "tip_title", kind_label));
  lines.push(el("div", "tip_line", format_int(layout.lines[id]) + " lines \u00b7 " + format_bytes(layout.bytes[id])));
  lines.push(el("div", "tip_line", state.languages[layout.lang[id]].name));
  return lines;
}

function paint_tooltip(id, detail) {
  clear(tooltip);
  if (detail) {
    tooltip.append(el("div", "tip_title", detail.name));
    tooltip.append(el("div", "tip_path", detail.path || "."));
    const kind = detail.kind === "file" ? detail.language : detail.language + " \u00b7 " + format_int(detail.files) + " files";
    tooltip.append(el("div", "tip_line", kind));
    tooltip.append(el("div", "tip_line", format_int(detail.lines) + " lines (" + format_int(detail.code) + " code, " + format_int(detail.comment) + " comments, " + format_int(detail.blank) + " blank)"));
    tooltip.append(el("div", "tip_line", format_bytes(detail.bytes) + " \u00b7 " + format_int(detail.chars) + " chars"));
    return;
  }
  tooltip.append(...basic_tooltip(id));
}

function place_tooltip(x, y) {
  tooltip.classList.remove("hidden");
  const rect = tooltip.getBoundingClientRect();
  const left = Math.min(x + 16, state.css_width - rect.width - 8);
  const top = Math.min(y + 18, state.css_height - rect.height - 8);
  tooltip.style.left = Math.max(4, left) + "px";
  tooltip.style.top = Math.max(4, top) + "px";
}

function hide_tooltip() {
  tooltip.classList.add("hidden");
  clearTimeout(hover_timer);
}

function on_hover(x, y) {
  const id = pick(x, y);
  if (id !== state.hovered) {
    state.hovered = id;
    request_render();
  }
  if (id < 0) {
    hide_tooltip();
    return;
  }
  paint_tooltip(id, detail_cache.get(id) || null);
  place_tooltip(x, y);
  clearTimeout(hover_timer);
  hover_timer = setTimeout(async () => {
    try {
      const detail = await detail_for(id);
      if (state.hovered === id) {
        paint_tooltip(id, detail);
        place_tooltip(x, y);
      }
    } catch (error) {
      set_status(String(error.message || error));
    }
  }, hover_detail_delay_ms);
}

async function select(id) {
  state.selected = id;
  request_render();
  const token = ++detail_token;
  if (id < 0) {
    render_selection(panel_refs.selection, null, state.palette);
    return;
  }
  try {
    const detail = await detail_for(id);
    if (token === detail_token) {
      render_selection(panel_refs.selection, detail, state.palette);
      if (app.classList.contains("panel_hidden")) {
        toggle_panel();
      }
    }
  } catch (error) {
    show_message(String(error.message || error));
  }
}

function focus_item(id) {
  const layout = state.layout;
  const cx = layout.x[id] + layout.w[id] / 2;
  const cz = layout.y[id] + layout.h[id] / 2;
  const size = Math.max(layout.w[id], layout.h[id]);
  animate_to(state.camera, cx, cz, focus_distance(state.camera, size), performance.now());
  request_render();
}

function on_click(x, y) {
  const id = pick(x, y);
  select(id === state.selected ? -1 : id);
}

function on_dblclick(x, y) {
  const id = pick(x, y);
  if (id >= 0) {
    select(id);
    focus_item(id);
  }
}

function change_mode(mode) {
  state.mode = mode;
  sync_mode_buttons();
  if (state.layout) {
    reset_camera(state.camera, state.layout.world, mode);
  } else {
    set_mode(state.camera, mode);
  }
  request_render();
}

function reset_view() {
  if (state.layout) {
    reset_camera(state.camera, state.layout.world, state.mode);
    request_render();
  }
}

function toggle_panel() {
  const hidden = app.classList.toggle("panel_hidden");
  panel_toggle.setAttribute("aria-expanded", hidden ? "false" : "true");
}

function update_legend() {
  clear(legend);
  if (state.color_by === "language") {
    const top = state.summary.report.languages.slice(0, 10);
    for (const lang of top) {
      const item = el("button", "legend_item", lang.name);
      item.type = "button";
      const dot = el("i", "swatch");
      dot.style.backgroundColor = state.palette.css[lang.index];
      item.prepend(dot);
      item.classList.toggle("active", state.focus_lang === lang.index);
      item.addEventListener("click", () => pick_language(lang.index));
      legend.append(item);
    }
    return;
  }
  const bar = el("div", "ramp_bar");
  bar.style.background = ramp_css;
  const low = state.color_by === "depth" ? "root" : "small";
  const high = state.color_by === "depth" ? "depth " + (state.layout ? state.layout.depth_reached : 0) : "large";
  const labels = el("div", "ramp_labels");
  labels.append(el("span", "", low), el("span", "", high));
  legend.append(bar, labels);
}

function pick_language(index) {
  state.focus_lang = state.focus_lang === index ? -1 : index;
  if (panel_refs) {
    panel_refs.language_table.mark(state.focus_lang);
  }
  update_legend();
  request_render();
}

async function apply_depth(depth, reset_view_after) {
  set_status("Loading layout\u2026");
  try {
    const layout = await load_layout(depth);
    state.max_depth = depth;
    state.layout = layout;
    detail_cache = new Map();
    state.selected = -1;
    state.hovered = -1;
    state.renderer.set_layout(layout);
    await load_dir_names(layout, depth);
    if (reset_view_after) {
      reset_camera(state.camera, layout.world, state.mode);
    }
    if (panel_refs) {
      render_selection(panel_refs.selection, null, state.palette);
    }
    update_status();
    update_legend();
    hide_message();
    request_render();
  } catch (error) {
    show_message(String(error.message || error));
    set_status("");
  }
}

function hide_search() {
  search_results.classList.add("hidden");
  clear(search_results);
}

async function run_search(query) {
  const token = ++search_token;
  try {
    const hits = await search_nodes(state.layout, state.max_depth, query);
    if (token !== search_token) {
      return;
    }
    clear(search_results);
    if (hits.length === 0) {
      search_results.append(el("div", "search_empty", "No match"));
    }
    for (const hit of hits) {
      const item = el("button", "search_item");
      item.type = "button";
      item.append(el("span", "search_kind", hit.kind), el("span", "search_path", hit.path));
      item.addEventListener("click", () => {
        hide_search();
        select(hit.id);
        focus_item(hit.id);
      });
      search_results.append(item);
    }
    search_results.classList.remove("hidden");
  } catch (error) {
    show_message(String(error.message || error));
  }
}

function init_controls() {
  for (const button of mode_buttons) {
    button.addEventListener("click", () => change_mode(button.dataset.mode));
  }
  color_select.value = state.color_by;
  height_select.value = state.height_by;
  depth_input.value = String(state.max_depth);
  depth_input.max = String(state.max_depth);
  depth_input.disabled = state.standalone;
  color_select.addEventListener("change", () => {
    state.color_by = color_select.value;
    update_legend();
    request_render();
  });
  height_select.addEventListener("change", () => {
    state.height_by = height_select.value;
    request_render();
  });
  depth_input.addEventListener("change", () => {
    const limit = state.summary.options.max_depth;
    const value = Math.min(limit, Math.max(1, Math.floor(Number(depth_input.value) || limit)));
    depth_input.value = String(value);
    apply_depth(value, true);
  });
  by_id("reset_view").addEventListener("click", reset_view);
  panel_toggle.addEventListener("click", toggle_panel);
  search_input.addEventListener("input", () => {
    clearTimeout(search_timer);
    const query = search_input.value.trim();
    if (!query) {
      search_token++;
      hide_search();
      return;
    }
    search_timer = setTimeout(() => run_search(query), search_delay_ms);
  });
  document.addEventListener("pointerdown", (event) => {
    if (!search_results.contains(event.target) && event.target !== search_input) {
      hide_search();
    }
  });
  sync_mode_buttons();
}

function init_renderer() {
  state.renderer = create_renderer(canvas, shader_sources, state.palette);
  state.labels = create_labels(labels_canvas);
  if (state.layout) {
    state.renderer.set_layout(state.layout);
  }
  handle_resize();
}

async function start() {
  const summary = await load_summary();
  state.summary = summary;
  state.languages = summary.languages;
  state.palette = build_palette(summary.languages, summary.report.languages.map((l) => l.index));
  state.standalone = is_standalone();
  state.mode = summary.options.mode;
  state.color_by = summary.options.color_by;
  state.height_by = summary.options.height_by;
  state.max_depth = summary.options.max_depth;
  document.title = "highrise-code \u2014 " + summary.root;

  const [vertex, fragment, pick_source] = await Promise.all([
    load_shader("building.vert"),
    load_shader("building.frag"),
    load_shader("pick.frag"),
  ]);
  shader_sources = { vertex, fragment, pick: pick_source };

  panel_refs = build_panel(by_id("panel"), summary, state.palette, pick_language);
  init_controls();
  init_renderer();
  set_frame_callback(frame);

  new ResizeObserver(handle_resize).observe(stage);

  attach_input(canvas, {
    on_hover,
    on_click,
    on_dblclick,
    on_leave: () => {
      if (state.hovered !== -1) {
        state.hovered = -1;
        request_render();
      }
      hide_tooltip();
    },
    on_drag_start: hide_tooltip,
    on_drag_end: () => {},
    on_camera_moved: () => {},
    on_mode: change_mode,
    on_reset: reset_view,
    on_toggle_panel: toggle_panel,
    on_toggle_labels: () => {
      state.show_labels = !state.show_labels;
      request_render();
    },
    on_focus_search: () => search_input.focus(),
    on_escape: () => {
      hide_search();
      search_input.value = "";
      if (state.focus_lang !== -1) {
        pick_language(state.focus_lang);
      }
      select(-1);
    },
  });

  canvas.addEventListener("webglcontextlost", (event) => {
    event.preventDefault();
    show_message("WebGL context lost, waiting for restore\u2026");
  });
  canvas.addEventListener("webglcontextrestored", () => {
    hide_message();
    init_renderer();
    request_render();
  });

  await apply_depth(state.max_depth, true);
}

start().catch((error) => {
  show_message(String(error && error.message ? error.message : error));
});
