#version 300 es
precision highp float;
precision highp int;

layout(location = 0) in vec3 a_position;
layout(location = 1) in vec3 a_normal;
layout(location = 2) in vec4 a_rect;
layout(location = 3) in vec4 a_values;
layout(location = 4) in vec4 a_class;

uniform mat4 u_view_proj;
uniform float u_slab;
uniform float u_height_max;
uniform int u_height_mode;
uniform int u_color_mode;
uniform vec4 u_max_values;
uniform float u_selected;
uniform float u_hovered;
uniform float u_focus_lang;
uniform sampler2D u_palette;

out vec3 v_color;
out vec3 v_normal;
flat out vec4 v_id;

vec3 ramp(float t) {
  t = clamp(t, 0.0, 1.0);
  vec3 c0 = vec3(0.18, 0.28, 0.62);
  vec3 c1 = vec3(0.16, 0.70, 0.62);
  vec3 c2 = vec3(0.96, 0.82, 0.28);
  vec3 c3 = vec3(0.92, 0.36, 0.24);
  if (t < 0.333) {
    return mix(c0, c1, t * 3.0);
  }
  if (t < 0.667) {
    return mix(c1, c2, (t - 0.333) * 3.0);
  }
  return mix(c2, c3, (t - 0.667) * 3.0);
}

void main() {
  float kind = a_class.y;
  float depth = a_values.w;
  float base = depth * u_slab;
  float height = u_slab;
  if (kind > 0.5) {
    float value = a_values.x;
    float vmax = u_max_values.x;
    if (u_height_mode == 1) {
      value = a_values.y;
      vmax = u_max_values.y;
    } else if (u_height_mode == 2) {
      value = a_values.z;
      vmax = u_max_values.z;
    }
    float t = 0.1;
    if (u_height_mode != 3) {
      t = pow(clamp(value / max(vmax, 1.0), 0.0, 1.0), 0.65);
    }
    height = u_slab * 0.5 + t * u_height_max;
  }

  vec3 world = vec3(
    a_rect.x + a_position.x * a_rect.z,
    base + a_position.y * height,
    a_rect.y + a_position.z * a_rect.w
  );
  gl_Position = u_view_proj * vec4(world, 1.0);
  v_normal = a_normal;

  vec3 color;
  if (kind < 0.5) {
    float alt = mod(depth, 2.0);
    color = mix(vec3(0.085, 0.105, 0.145), vec3(0.135, 0.165, 0.215), alt);
    if (u_color_mode == 1) {
      color = mix(color, ramp(depth / max(u_max_values.w, 1.0)), 0.18);
    }
  } else {
    if (u_color_mode == 0) {
      color = texelFetch(u_palette, ivec2(int(a_class.x), 0), 0).rgb;
    } else if (u_color_mode == 1) {
      color = ramp(depth / max(u_max_values.w, 1.0));
    } else {
      color = ramp(log2(1.0 + a_values.z) / log2(1.0 + max(u_max_values.z, 2.0)));
    }
    if (kind > 1.5) {
      color = mix(color, vec3(dot(color, vec3(0.333))), 0.35);
    }
  }
  if (u_focus_lang >= 0.0 && abs(a_class.x - u_focus_lang) > 0.5 && kind > 0.5) {
    color = mix(color, vec3(0.16, 0.17, 0.2), 0.88);
  }
  float id = float(gl_InstanceID);
  if (id == u_selected) {
    color = mix(color, vec3(1.0), 0.55);
  } else if (id == u_hovered) {
    color = mix(color, vec3(1.0), 0.3);
  }
  v_color = color;

  uint n = uint(gl_InstanceID) + 1u;
  v_id = vec4(float(n & 255u), float((n >> 8) & 255u), float((n >> 16) & 255u), 255.0) / 255.0;
}
