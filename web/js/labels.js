import { pixels_per_unit, project } from "hr/camera";
import { node_top } from "hr/renderer";

const kind_dir = 0;
const max_labels = 700;
const min_face_px = 26;
const min_chars = 4;
const pad_px = 3;
const gap_px = 2;
const base_font_px = 11;
const min_font_px = 8;
const max_font_px = 16;
const measure_font_px = 10;
const line_ratio = 1.25;
const axis_aligned_fill = 0.97;
const depth_margin = 0.4;
const font_family = "system-ui, sans-serif";

const corner_a = new Float32Array(3);
const corner_b = new Float32Array(3);
const corner_c = new Float32Array(3);
const corner_d = new Float32Array(3);

const caches = new WeakMap();

function build_cache(layout) {
  const first = layout.count > 1 ? 1 : 0;
  const order = new Int32Array(layout.count - first);
  for (let i = 0; i < order.length; i++) {
    order[i] = i + first;
  }
  const area = new Float32Array(layout.count);
  for (let i = 0; i < layout.count; i++) {
    area[i] = layout.w[i] * layout.h[i];
  }
  order.sort((a, b) => area[b] - area[a]);
  const widths = new Float32Array(layout.count);
  widths.fill(-1);
  return { order, widths };
}

function cache_for(layout) {
  let cache = caches.get(layout);
  if (!cache) {
    cache = build_cache(layout);
    caches.set(layout, cache);
  }
  return cache;
}

function overlaps(placed, count, x, y, w, h) {
  for (let i = 0; i < count; i++) {
    const o = i * 4;
    if (x < placed[o + 2] && x + w > placed[o] && y < placed[o + 3] && y + h > placed[o + 1]) {
      return true;
    }
  }
  return false;
}

export function create_labels(canvas) {
  const ctx = canvas.getContext("2d");
  const measure = document.createElement("canvas").getContext("2d");
  measure.font = measure_font_px + "px " + font_family;
  const placed = new Float32Array(max_labels * 4);
  let width = 1;
  let height = 1;
  let dpr = 1;

  function resize(css_width, css_height, ratio) {
    width = css_width;
    height = css_height;
    dpr = ratio;
    canvas.width = Math.round(css_width * ratio);
    canvas.height = Math.round(css_height * ratio);
  }

  function clear() {
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, canvas.width, canvas.height);
  }

  function unit_width(cache, names, id) {
    let unit = cache.widths[id];
    if (unit < 0) {
      unit = measure.measureText(names[id]).width / measure_font_px;
      cache.widths[id] = unit;
    }
    return unit;
  }

  function draw(view) {
    clear();
    const layout = view.layout;
    if (!view.show_labels || !layout || !layout.names) {
      return;
    }
    const camera = view.camera;
    const names = layout.names;
    const cache = cache_for(layout);
    const order = cache.order;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.textBaseline = "top";
    ctx.lineWidth = 3;
    ctx.lineJoin = "round";
    ctx.strokeStyle = "rgba(8, 10, 14, 0.85)";
    ctx.fillStyle = "rgba(235, 240, 250, 0.95)";
    let drawn = 0;
    for (let n = 0; n < order.length && drawn < max_labels; n++) {
      const id = order[n];
      const x = layout.x[id];
      const z = layout.y[id];
      const w = layout.w[id];
      const h = layout.h[id];
      const top = node_top(layout, id, camera.mode, view.height_by);
      if (!project(camera, x, top, z, corner_a)) {
        continue;
      }
      const reach = w + h;
      const scale = pixels_per_unit(camera, Math.max(corner_a[2] - reach, corner_a[2] * depth_margin));
      const estimate = reach * scale;
      if (estimate < min_face_px) {
        continue;
      }
      if (
        corner_a[0] + estimate < 0 ||
        corner_a[0] - estimate > width ||
        corner_a[1] + estimate < 0 ||
        corner_a[1] - estimate > height
      ) {
        continue;
      }
      if (
        !project(camera, x + w, top, z, corner_b) ||
        !project(camera, x + w, top, z + h, corner_c) ||
        !project(camera, x, top, z + h, corner_d)
      ) {
        continue;
      }
      const min_x = Math.min(corner_a[0], corner_b[0], corner_c[0], corner_d[0]);
      const max_x = Math.max(corner_a[0], corner_b[0], corner_c[0], corner_d[0]);
      const min_y = Math.min(corner_a[1], corner_b[1], corner_c[1], corner_d[1]);
      const max_y = Math.max(corner_a[1], corner_b[1], corner_c[1], corner_d[1]);
      const span_w = max_x - min_x;
      const span_h = max_y - min_y;
      if (span_w < 1 || span_h < 1 || max_x < 0 || min_x > width || max_y < 0 || min_y > height) {
        continue;
      }
      const polygon_area =
        Math.abs(
          corner_a[0] * corner_b[1] - corner_b[0] * corner_a[1] +
            corner_b[0] * corner_c[1] - corner_c[0] * corner_b[1] +
            corner_c[0] * corner_d[1] - corner_d[0] * corner_c[1] +
            corner_d[0] * corner_a[1] - corner_a[0] * corner_d[1],
        ) / 2;
      const fill = Math.sqrt(Math.min(1, polygon_area / (span_w * span_h)));
      const usable_w = span_w * fill - 2 * pad_px;
      if (usable_w < min_face_px) {
        continue;
      }
      const height_cap = (span_h - 2 * pad_px) / line_ratio;
      if (height_cap < min_font_px) {
        continue;
      }
      const unit = unit_width(cache, names, id);
      if (unit <= 0) {
        continue;
      }
      let text = names[id];
      let size = Math.min(max_font_px, usable_w / unit, height_cap);
      let text_w = unit * size;
      if (size < min_font_px) {
        size = Math.min(base_font_px, height_cap);
        const char_w = (unit * size) / text.length;
        const max_chars = Math.floor(usable_w / char_w);
        if (max_chars < min_chars) {
          continue;
        }
        if (max_chars < text.length) {
          text = text.slice(0, max_chars - 1) + "\u2026";
        }
        text_w = char_w * text.length;
      }
      const line_h = size * line_ratio;
      const anchored_left = layout.kind[id] === kind_dir && fill >= axis_aligned_fill;
      let label_x;
      let label_y;
      if (anchored_left) {
        label_x = min_x + pad_px;
        label_y = min_y + pad_px;
      } else if (layout.kind[id] === kind_dir) {
        label_x = (min_x + max_x) / 2 - text_w / 2;
        label_y = min_y + span_h * 0.12;
      } else {
        label_x = (min_x + max_x) / 2 - text_w / 2;
        label_y = (min_y + max_y) / 2 - line_h / 2;
      }
      if (label_x + text_w < 0 || label_x > width || label_y + line_h < 0 || label_y > height) {
        continue;
      }
      if (overlaps(placed, drawn, label_x - gap_px, label_y - gap_px, text_w + 2 * gap_px, line_h + 2 * gap_px)) {
        continue;
      }
      const slot = drawn * 4;
      placed[slot] = label_x - gap_px;
      placed[slot + 1] = label_y - gap_px;
      placed[slot + 2] = label_x + text_w + gap_px;
      placed[slot + 3] = label_y + line_h + gap_px;
      ctx.font = size + "px " + font_family;
      ctx.textAlign = "left";
      ctx.strokeText(text, label_x, label_y);
      ctx.fillText(text, label_x, label_y);
      drawn++;
    }
  }

  return { resize, clear, draw };
}
