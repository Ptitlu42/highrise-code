import {
  build_cube_data,
  create_context,
  create_pick_target,
  create_program,
  cube_vertex_count,
  destroy_pick_target,
  uniform_locations,
} from "hr/gl";

const uniform_names = [
  "u_view_proj",
  "u_slab",
  "u_height_max",
  "u_height_mode",
  "u_color_mode",
  "u_max_values",
  "u_selected",
  "u_hovered",
  "u_focus_lang",
  "u_palette",
  "u_light",
  "u_flat",
];

const height_modes = { lines: 0, code: 1, size: 2, flat: 3 };
const color_modes = { language: 0, depth: 1, size: 2 };
const background = [0.035, 0.043, 0.06];
const light = [0.45, 0.85, 0.35];
const slab_ratio = 0.002;
const height_ratio = 0.07;

export function slab_thickness(world, mode) {
  return mode === "2d" ? 0 : world * slab_ratio;
}

export function node_top(layout, id, mode, height_by) {
  const slab = slab_thickness(layout.world, mode);
  const base = layout.depth[id] * slab;
  if (layout.kind[id] === 0) {
    return base + slab;
  }
  if (mode === "2d") {
    return base;
  }
  let t = 0.12;
  if (height_by !== "flat") {
    let value = layout.lines[id];
    let vmax = layout.max_lines;
    if (height_by === "code") {
      value = layout.code[id];
      vmax = layout.max_code;
    } else if (height_by === "size") {
      value = layout.bytes[id];
      vmax = layout.max_bytes;
    }
    t = Math.sqrt(Math.min(1, Math.max(0, value / Math.max(vmax, 1))));
  }
  return base + slab * 0.5 + t * layout.world * height_ratio;
}

export function create_renderer(canvas, sources, palette) {
  const gl = create_context(canvas);
  const building = create_program(gl, sources.vertex, sources.fragment, "building");
  const pick = create_program(gl, sources.vertex, sources.pick, "pick");
  const locations = {
    building: uniform_locations(gl, building, uniform_names),
    pick: uniform_locations(gl, pick, uniform_names),
  };

  const cube_buffer = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, cube_buffer);
  gl.bufferData(gl.ARRAY_BUFFER, build_cube_data(), gl.STATIC_DRAW);
  const rect_buffer = gl.createBuffer();
  const values_buffer = gl.createBuffer();
  const class_buffer = gl.createBuffer();

  const vao = gl.createVertexArray();
  gl.bindVertexArray(vao);
  gl.bindBuffer(gl.ARRAY_BUFFER, cube_buffer);
  gl.enableVertexAttribArray(0);
  gl.vertexAttribPointer(0, 3, gl.FLOAT, false, 24, 0);
  gl.enableVertexAttribArray(1);
  gl.vertexAttribPointer(1, 3, gl.FLOAT, false, 24, 12);
  const instance_attributes = [
    [2, rect_buffer],
    [3, values_buffer],
    [4, class_buffer],
  ];
  for (const [index, buffer] of instance_attributes) {
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
    gl.enableVertexAttribArray(index);
    gl.vertexAttribPointer(index, 4, gl.FLOAT, false, 0, 0);
    gl.vertexAttribDivisor(index, 1);
  }
  gl.bindVertexArray(null);

  const palette_texture = gl.createTexture();
  gl.bindTexture(gl.TEXTURE_2D, palette_texture);
  gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
  gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, palette.count, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, palette.data);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);

  let pick_target = null;
  let instance_count = 0;
  let layout = null;
  const pixel = new Uint8Array(4);

  function set_layout(next) {
    layout = next;
    instance_count = next.count;
    gl.bindBuffer(gl.ARRAY_BUFFER, rect_buffer);
    gl.bufferData(gl.ARRAY_BUFFER, next.rect, gl.STATIC_DRAW);
    gl.bindBuffer(gl.ARRAY_BUFFER, values_buffer);
    gl.bufferData(gl.ARRAY_BUFFER, next.values, gl.STATIC_DRAW);
    gl.bindBuffer(gl.ARRAY_BUFFER, class_buffer);
    gl.bufferData(gl.ARRAY_BUFFER, next.cls, gl.STATIC_DRAW);
  }

  function resize(width, height) {
    if (canvas.width !== width || canvas.height !== height) {
      canvas.width = width;
      canvas.height = height;
    }
    destroy_pick_target(gl, pick_target);
    pick_target = null;
  }

  function apply(program, locs, view, for_pick) {
    const camera = view.camera;
    const flat = camera.mode === "2d";
    const world = layout.world;
    gl.useProgram(program);
    gl.uniformMatrix4fv(locs.u_view_proj, false, camera.view_proj);
    gl.uniform1f(locs.u_slab, slab_thickness(world, camera.mode));
    gl.uniform1f(locs.u_height_max, flat ? 0 : world * height_ratio);
    gl.uniform1i(locs.u_height_mode, height_modes[view.height_by] ?? 0);
    gl.uniform1i(locs.u_color_mode, color_modes[view.color_by] ?? 0);
    gl.uniform4f(locs.u_max_values, layout.max_lines, layout.max_code, layout.max_bytes, layout.depth_reached);
    gl.uniform1f(locs.u_selected, for_pick ? -1 : view.selected);
    gl.uniform1f(locs.u_hovered, for_pick ? -1 : view.hovered);
    gl.uniform1f(locs.u_focus_lang, view.focus_lang);
    gl.uniform3f(locs.u_light, light[0], light[1], light[2]);
    gl.uniform1f(locs.u_flat, flat ? 1 : 0);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, palette_texture);
    gl.uniform1i(locs.u_palette, 0);
    if (flat) {
      gl.disable(gl.DEPTH_TEST);
      gl.disable(gl.CULL_FACE);
    } else {
      gl.enable(gl.DEPTH_TEST);
      gl.depthFunc(gl.LEQUAL);
      gl.enable(gl.CULL_FACE);
      gl.cullFace(gl.BACK);
    }
  }

  function draw(view) {
    if (!layout) {
      gl.viewport(0, 0, canvas.width, canvas.height);
      gl.clearColor(background[0], background[1], background[2], 1);
      gl.clear(gl.COLOR_BUFFER_BIT);
      return;
    }
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    gl.disable(gl.SCISSOR_TEST);
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.clearColor(background[0], background[1], background[2], 1);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    apply(building, locations.building, view, false);
    gl.bindVertexArray(vao);
    gl.drawArraysInstanced(gl.TRIANGLES, 0, cube_vertex_count, instance_count);
    gl.bindVertexArray(null);
  }

  function pick_at(view, x, y) {
    if (!layout || x < 0 || y < 0 || x >= canvas.width || y >= canvas.height) {
      return -1;
    }
    if (!pick_target) {
      pick_target = create_pick_target(gl, canvas.width, canvas.height);
    }
    if (!pick_target.complete) {
      return -1;
    }
    const py = canvas.height - 1 - y;
    gl.bindFramebuffer(gl.FRAMEBUFFER, pick_target.framebuffer);
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.enable(gl.SCISSOR_TEST);
    gl.scissor(x, py, 1, 1);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    apply(pick, locations.pick, view, true);
    gl.bindVertexArray(vao);
    gl.drawArraysInstanced(gl.TRIANGLES, 0, cube_vertex_count, instance_count);
    gl.bindVertexArray(null);
    gl.readPixels(x, py, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, pixel);
    gl.disable(gl.SCISSOR_TEST);
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    const id = pixel[0] | (pixel[1] << 8) | (pixel[2] << 16);
    return id - 1;
  }

  function dispose() {
    destroy_pick_target(gl, pick_target);
    gl.deleteProgram(building);
    gl.deleteProgram(pick);
    gl.deleteBuffer(cube_buffer);
    gl.deleteBuffer(rect_buffer);
    gl.deleteBuffer(values_buffer);
    gl.deleteBuffer(class_buffer);
    gl.deleteVertexArray(vao);
    gl.deleteTexture(palette_texture);
  }

  return { gl, set_layout, resize, draw, pick_at, dispose };
}
