import {
  ground_point,
  orbit,
  pan_grab,
  pan_screen,
  rotate_yaw,
  zoom_at,
} from "hr/camera";
import { request_render, state } from "hr/state";

const click_slop_px = 4;
const hover_interval_ms = 45;
const wheel_line_px = 16;
const wheel_page_px = 400;
const wheel_sensitivity = 0.0016;
const key_zoom_factor = 0.8;
const key_pan_px = 60;
const key_rotate = Math.PI / 12;

export function attach_input(canvas, hooks) {
  let drag = null;
  let hover_timer = 0;
  let hover_last = 0;
  let hover_x = 0;
  let hover_y = 0;
  const grab = new Float32Array(3);

  function local(event) {
    const rect = canvas.getBoundingClientRect();
    return [event.clientX - rect.left, event.clientY - rect.top];
  }

  function schedule_hover() {
    if (hover_timer) {
      return;
    }
    const wait = Math.max(0, hover_interval_ms - (performance.now() - hover_last));
    hover_timer = setTimeout(() => {
      hover_timer = 0;
      hover_last = performance.now();
      hooks.on_hover(hover_x, hover_y);
    }, wait);
  }

  canvas.addEventListener("pointerdown", (event) => {
    if (event.button !== 0 && event.button !== 2) {
      return;
    }
    if (event.isTrusted) {
      canvas.setPointerCapture(event.pointerId);
    }
    canvas.focus({ preventScroll: true });
    const [x, y] = local(event);
    const camera = state.camera;
    camera.animation = null;
    const pan = event.button === 2 || event.shiftKey || camera.mode !== "3d";
    ground_point(camera, x, y, grab);
    drag = {
      id: event.pointerId,
      kind: pan ? "pan" : "orbit",
      button: event.button,
      start_x: x,
      start_y: y,
      last_x: x,
      last_y: y,
      moved: false,
    };
  });

  canvas.addEventListener("pointermove", (event) => {
    const [x, y] = local(event);
    if (!drag || drag.id !== event.pointerId) {
      hover_x = x;
      hover_y = y;
      schedule_hover();
      return;
    }
    if (!drag.moved && Math.hypot(x - drag.start_x, y - drag.start_y) > click_slop_px) {
      drag.moved = true;
      hooks.on_drag_start();
    }
    if (!drag.moved) {
      return;
    }
    if (drag.kind === "orbit") {
      orbit(state.camera, x - drag.last_x, y - drag.last_y);
    } else {
      pan_grab(state.camera, grab, x, y);
    }
    drag.last_x = x;
    drag.last_y = y;
    request_render();
  });

  function end_drag(event) {
    if (!drag || drag.id !== event.pointerId) {
      return;
    }
    const finished = drag;
    drag = null;
    if (event.isTrusted && canvas.hasPointerCapture(event.pointerId)) {
      canvas.releasePointerCapture(event.pointerId);
    }
    if (!finished.moved && finished.button === 0 && event.type === "pointerup") {
      hooks.on_click(finished.start_x, finished.start_y);
    }
    if (finished.moved) {
      hooks.on_drag_end();
    }
  }

  canvas.addEventListener("pointerup", end_drag);
  canvas.addEventListener("pointercancel", end_drag);
  canvas.addEventListener("pointerleave", () => {
    if (!drag) {
      hooks.on_leave();
    }
  });
  canvas.addEventListener("contextmenu", (event) => event.preventDefault());

  canvas.addEventListener(
    "wheel",
    (event) => {
      event.preventDefault();
      const [x, y] = local(event);
      let delta = event.deltaY;
      if (event.deltaMode === 1) {
        delta *= wheel_line_px;
      } else if (event.deltaMode === 2) {
        delta *= wheel_page_px;
      }
      state.camera.animation = null;
      zoom_at(state.camera, x, y, Math.exp(delta * wheel_sensitivity));
      hooks.on_camera_moved();
      request_render();
    },
    { passive: false },
  );

  canvas.addEventListener("dblclick", (event) => {
    const [x, y] = local(event);
    hooks.on_dblclick(x, y);
  });

  window.addEventListener("keydown", (event) => {
    const tag = event.target && event.target.tagName;
    if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") {
      if (event.key === "Escape") {
        event.target.blur();
        hooks.on_escape();
      }
      return;
    }
    if (event.ctrlKey || event.metaKey || event.altKey) {
      return;
    }
    const camera = state.camera;
    switch (event.key) {
      case "1":
        hooks.on_mode("3d");
        break;
      case "2":
        hooks.on_mode("iso");
        break;
      case "3":
        hooks.on_mode("2d");
        break;
      case "r":
      case "R":
        hooks.on_reset();
        break;
      case "p":
      case "P":
        hooks.on_toggle_panel();
        break;
      case "l":
      case "L":
        hooks.on_toggle_labels();
        break;
      case "/":
        event.preventDefault();
        hooks.on_focus_search();
        break;
      case "Escape":
        hooks.on_escape();
        break;
      case "+":
      case "=":
        zoom_at(camera, state.css_width / 2, state.css_height / 2, key_zoom_factor);
        hooks.on_camera_moved();
        break;
      case "-":
      case "_":
        zoom_at(camera, state.css_width / 2, state.css_height / 2, 1 / key_zoom_factor);
        hooks.on_camera_moved();
        break;
      case "ArrowLeft":
        pan_screen(camera, -key_pan_px, 0);
        break;
      case "ArrowRight":
        pan_screen(camera, key_pan_px, 0);
        break;
      case "ArrowUp":
        pan_screen(camera, 0, -key_pan_px);
        break;
      case "ArrowDown":
        pan_screen(camera, 0, key_pan_px);
        break;
      case "q":
      case "Q":
        rotate_yaw(camera, -key_rotate);
        break;
      case "e":
      case "E":
        rotate_yaw(camera, key_rotate);
        break;
      default:
        return;
    }
    event.preventDefault();
    hooks.on_camera_moved();
    request_render();
  });
}
