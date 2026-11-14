import * as mat4 from "hr/mat4";

const fov = Math.PI / 4;
const tan_half_fov = Math.tan(fov / 2);
const max_pitch = 1.5707;
const min_pitch_orbit = 0.08;

const presets = {
  "3d": { yaw: 0.65, pitch: 0.72 },
  iso: { yaw: Math.PI / 4, pitch: Math.atan(1 / Math.SQRT2) },
  "2d": { yaw: 0, pitch: max_pitch },
};

const ray_near = new Float32Array(4);
const ray_far = new Float32Array(4);
const clip = new Float32Array(4);

export function create_camera() {
  return {
    mode: "3d",
    yaw: presets["3d"].yaw,
    pitch: presets["3d"].pitch,
    dist: 1000,
    tx: 0,
    tz: 0,
    world: 1000,
    width: 1,
    height: 1,
    eye: new Float32Array(3),
    view: mat4.create(),
    proj: mat4.create(),
    view_proj: mat4.create(),
    inv_view_proj: mat4.create(),
    inverse_ok: false,
    animation: null,
  };
}

export function is_orthographic(camera) {
  return camera.mode !== "3d";
}

function fit_distance(camera, world, mode) {
  const aspect = camera.width / camera.height;
  if (mode === "3d") {
    return world * Math.max(1.45, 1.7 / aspect);
  }
  const half_extent = mode === "iso" ? 0.75 : 0.56;
  const half_h = Math.max(half_extent / aspect, mode === "iso" ? 0.55 : 0.56) * world;
  return half_h / tan_half_fov;
}

export function reset(camera, world, mode) {
  camera.mode = mode;
  camera.world = world;
  camera.yaw = presets[mode].yaw;
  camera.pitch = presets[mode].pitch;
  camera.tx = world / 2;
  camera.tz = world / 2;
  camera.animation = null;
  camera.dist = fit_distance(camera, world, mode);
  update(camera);
}

export function set_mode(camera, mode) {
  const preset = presets[mode];
  camera.mode = mode;
  camera.yaw = preset.yaw;
  camera.pitch = preset.pitch;
  camera.animation = null;
  update(camera);
}

export function resize(camera, width, height) {
  camera.width = Math.max(1, width);
  camera.height = Math.max(1, height);
  update(camera);
}

export function clamp_distance(camera) {
  const min = camera.world * 0.0015;
  const max = camera.world * 4;
  camera.dist = Math.min(max, Math.max(min, camera.dist));
}

export function update(camera) {
  clamp_distance(camera);
  camera.pitch = Math.min(max_pitch, Math.max(min_pitch_orbit, camera.pitch));
  const cp = Math.cos(camera.pitch);
  const ex = camera.tx + camera.dist * cp * Math.sin(camera.yaw);
  const ey = camera.dist * Math.sin(camera.pitch);
  const ez = camera.tz + camera.dist * cp * Math.cos(camera.yaw);
  camera.eye[0] = ex;
  camera.eye[1] = ey;
  camera.eye[2] = ez;
  mat4.look_at(camera.view, ex, ey, ez, camera.tx, 0, camera.tz, 0, 1, 0);
  const aspect = camera.width / camera.height;
  if (is_orthographic(camera)) {
    const half_h = camera.dist * tan_half_fov;
    const half_w = half_h * aspect;
    mat4.ortho(camera.proj, -half_w, half_w, -half_h, half_h, -camera.world * 2, camera.dist + camera.world * 2);
  } else {
    const near = Math.max(camera.dist * 0.01, camera.world * 0.0002);
    mat4.perspective(camera.proj, fov, aspect, near, camera.dist + camera.world * 4);
  }
  mat4.multiply(camera.view_proj, camera.proj, camera.view);
  camera.inverse_ok = mat4.invert(camera.inv_view_proj, camera.view_proj);
}

export function ground_point(camera, sx, sy, out) {
  if (!camera.inverse_ok) {
    return false;
  }
  const nx = (sx / camera.width) * 2 - 1;
  const ny = 1 - (sy / camera.height) * 2;
  mat4.transform(ray_near, camera.inv_view_proj, nx, ny, -1, 1);
  mat4.transform(ray_far, camera.inv_view_proj, nx, ny, 1, 1);
  const nw = 1 / ray_near[3];
  const fw = 1 / ray_far[3];
  const ax = ray_near[0] * nw;
  const ay = ray_near[1] * nw;
  const az = ray_near[2] * nw;
  const bx = ray_far[0] * fw;
  const by = ray_far[1] * fw;
  const bz = ray_far[2] * fw;
  const dy = by - ay;
  if (Math.abs(dy) < 1e-9) {
    return false;
  }
  const t = -ay / dy;
  out[0] = ax + (bx - ax) * t;
  out[1] = 0;
  out[2] = az + (bz - az) * t;
  return true;
}

export function orbit(camera, dx, dy) {
  camera.yaw -= dx * 0.006;
  camera.pitch += dy * 0.006;
  update(camera);
}

export function rotate_yaw(camera, delta) {
  camera.yaw += delta;
  update(camera);
}

const scratch_point = new Float32Array(3);

export function pan_grab(camera, grab, sx, sy) {
  if (!ground_point(camera, sx, sy, scratch_point)) {
    return;
  }
  camera.tx += grab[0] - scratch_point[0];
  camera.tz += grab[2] - scratch_point[2];
  update(camera);
}

export function zoom_at(camera, sx, sy, factor) {
  const had_point = ground_point(camera, sx, sy, scratch_point);
  const before = camera.dist;
  camera.dist *= factor;
  clamp_distance(camera);
  const applied = camera.dist / before;
  if (had_point) {
    camera.tx = scratch_point[0] + (camera.tx - scratch_point[0]) * applied;
    camera.tz = scratch_point[2] + (camera.tz - scratch_point[2]) * applied;
  }
  update(camera);
}

export function pan_screen(camera, dx_px, dy_px) {
  const world_per_px = (2 * camera.dist * tan_half_fov) / camera.height;
  const right_x = Math.cos(camera.yaw);
  const right_z = -Math.sin(camera.yaw);
  const forward_x = -Math.sin(camera.yaw);
  const forward_z = -Math.cos(camera.yaw);
  camera.tx += (right_x * dx_px - forward_x * dy_px) * world_per_px;
  camera.tz += (right_z * dx_px - forward_z * dy_px) * world_per_px;
  update(camera);
}

export function focus_distance(camera, size) {
  const base = size * 1.15;
  if (is_orthographic(camera)) {
    return base / (2 * tan_half_fov) * 1.35;
  }
  return base * 1.6;
}

export function animate_to(camera, tx, tz, dist, now) {
  camera.animation = {
    start: now,
    duration: 320,
    from_tx: camera.tx,
    from_tz: camera.tz,
    from_dist: camera.dist,
    to_tx: tx,
    to_tz: tz,
    to_dist: dist,
  };
}

export function step_animation(camera, now) {
  const a = camera.animation;
  if (!a) {
    return false;
  }
  const raw = Math.min(1, (now - a.start) / a.duration);
  const t = raw * raw * (3 - 2 * raw);
  camera.tx = a.from_tx + (a.to_tx - a.from_tx) * t;
  camera.tz = a.from_tz + (a.to_tz - a.from_tz) * t;
  camera.dist = a.from_dist + (a.to_dist - a.from_dist) * t;
  update(camera);
  if (raw >= 1) {
    camera.animation = null;
    return false;
  }
  return true;
}

export function project(camera, x, y, z, out) {
  mat4.transform(clip, camera.view_proj, x, y, z, 1);
  if (clip[3] <= 0) {
    out[2] = 0;
    return false;
  }
  const inv = 1 / clip[3];
  out[0] = (clip[0] * inv * 0.5 + 0.5) * camera.width;
  out[1] = (1 - (clip[1] * inv * 0.5 + 0.5)) * camera.height;
  out[2] = 1;
  return true;
}
