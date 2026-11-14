import { project } from "hr/camera";

const max_labels = 90;
const min_width_px = 64;
const char_width_px = 6.4;
const font = "11px system-ui, sans-serif";
const corner = new Float32Array(3);
const corner_far = new Float32Array(3);

export function create_labels(canvas) {
  const ctx = canvas.getContext("2d");
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

  function draw(camera, layout, slab, enabled) {
    clear();
    if (!enabled || !layout || !layout.dir_ids) {
      return;
    }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.font = font;
    ctx.textBaseline = "top";
    ctx.lineWidth = 3;
    ctx.lineJoin = "round";
    ctx.strokeStyle = "rgba(8, 10, 14, 0.85)";
    ctx.fillStyle = "rgba(235, 240, 250, 0.95)";
    const ids = layout.dir_ids;
    const names = layout.dir_names;
    let drawn = 0;
    for (let n = 0; n < ids.length && drawn < max_labels; n++) {
      const id = ids[n];
      if (id === 0 && layout.count > 1) {
        continue;
      }
      const x = layout.x[id];
      const z = layout.y[id];
      const w = layout.w[id];
      const y_top = (layout.depth[id] + 1) * slab;
      if (!project(camera, x, y_top, z, corner)) {
        continue;
      }
      if (!project(camera, x + w, y_top, z, corner_far)) {
        continue;
      }
      const span = Math.abs(corner_far[0] - corner[0]);
      if (span < min_width_px) {
        continue;
      }
      if (corner[0] < -span || corner[0] > width || corner[1] < -20 || corner[1] > height) {
        continue;
      }
      const max_chars = Math.max(3, Math.floor((span - 6) / char_width_px));
      let text = names[n];
      if (text.length > max_chars) {
        text = text.slice(0, max_chars - 1) + "\u2026";
      }
      ctx.strokeText(text, corner[0] + 4, corner[1] + 3);
      ctx.fillText(text, corner[0] + 4, corner[1] + 3);
      drawn++;
    }
  }

  return { resize, clear, draw };
}
