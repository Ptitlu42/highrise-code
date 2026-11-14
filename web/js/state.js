import { create_camera } from "hr/camera";

export const state = {
  summary: null,
  languages: [],
  palette: null,
  standalone: false,
  mode: "3d",
  color_by: "language",
  height_by: "lines",
  max_depth: 256,
  layout: null,
  camera: create_camera(),
  renderer: null,
  labels: null,
  selected: -1,
  hovered: -1,
  focus_lang: -1,
  show_labels: true,
  dpr: 1,
  css_width: 1,
  css_height: 1,
};

let frame_pending = false;
let frame_callback = null;

export function set_frame_callback(callback) {
  frame_callback = callback;
}

function run_frame(now) {
  frame_pending = false;
  if (frame_callback) {
    frame_callback(now);
  }
}

export function request_render() {
  if (!frame_pending) {
    frame_pending = true;
    requestAnimationFrame(run_frame);
  }
}
