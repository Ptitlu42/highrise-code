export function create_context(canvas) {
  const gl = canvas.getContext("webgl2", {
    alpha: false,
    antialias: true,
    depth: true,
    stencil: false,
    powerPreference: "high-performance",
    preserveDrawingBuffer: false,
  });
  if (!gl) {
    throw new Error("WebGL2 is not available in this browser");
  }
  return gl;
}

function compile_shader(gl, type, source, label) {
  const shader = gl.createShader(type);
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const log = gl.getShaderInfoLog(shader);
    gl.deleteShader(shader);
    throw new Error("shader " + label + ": " + log);
  }
  return shader;
}

export function create_program(gl, vertex_source, fragment_source, label) {
  const vs = compile_shader(gl, gl.VERTEX_SHADER, vertex_source, label + ".vert");
  const fs = compile_shader(gl, gl.FRAGMENT_SHADER, fragment_source, label + ".frag");
  const program = gl.createProgram();
  gl.attachShader(program, vs);
  gl.attachShader(program, fs);
  gl.linkProgram(program);
  gl.deleteShader(vs);
  gl.deleteShader(fs);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    const log = gl.getProgramInfoLog(program);
    gl.deleteProgram(program);
    throw new Error("program " + label + ": " + log);
  }
  return program;
}

export function uniform_locations(gl, program, names) {
  const out = {};
  for (const name of names) {
    out[name] = gl.getUniformLocation(program, name);
  }
  return out;
}

const cube_faces = [
  { n: [0, 1, 0], v: [[0, 1, 0], [0, 1, 1], [1, 1, 1], [1, 1, 0]] },
  { n: [0, 0, 1], v: [[0, 0, 1], [1, 0, 1], [1, 1, 1], [0, 1, 1]] },
  { n: [0, 0, -1], v: [[1, 0, 0], [0, 0, 0], [0, 1, 0], [1, 1, 0]] },
  { n: [1, 0, 0], v: [[1, 0, 1], [1, 0, 0], [1, 1, 0], [1, 1, 1]] },
  { n: [-1, 0, 0], v: [[0, 0, 0], [0, 0, 1], [0, 1, 1], [0, 1, 0]] },
];

export function build_cube_data() {
  const data = new Float32Array(cube_faces.length * 6 * 6);
  let offset = 0;
  const order = [0, 1, 2, 0, 2, 3];
  for (const face of cube_faces) {
    for (const index of order) {
      const v = face.v[index];
      data[offset++] = v[0];
      data[offset++] = v[1];
      data[offset++] = v[2];
      data[offset++] = face.n[0];
      data[offset++] = face.n[1];
      data[offset++] = face.n[2];
    }
  }
  return data;
}

export const cube_vertex_count = cube_faces.length * 6;

export function create_pick_target(gl, width, height) {
  const texture = gl.createTexture();
  gl.bindTexture(gl.TEXTURE_2D, texture);
  gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
  const depth = gl.createRenderbuffer();
  gl.bindRenderbuffer(gl.RENDERBUFFER, depth);
  gl.renderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, width, height);
  const framebuffer = gl.createFramebuffer();
  gl.bindFramebuffer(gl.FRAMEBUFFER, framebuffer);
  gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, texture, 0);
  gl.framebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, depth);
  const complete = gl.checkFramebufferStatus(gl.FRAMEBUFFER) === gl.FRAMEBUFFER_COMPLETE;
  gl.bindFramebuffer(gl.FRAMEBUFFER, null);
  gl.bindTexture(gl.TEXTURE_2D, null);
  return { texture, depth, framebuffer, width, height, complete };
}

export function destroy_pick_target(gl, target) {
  if (!target) {
    return;
  }
  gl.deleteFramebuffer(target.framebuffer);
  gl.deleteTexture(target.texture);
  gl.deleteRenderbuffer(target.depth);
}
