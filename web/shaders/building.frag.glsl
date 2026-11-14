#version 300 es
precision highp float;

in vec3 v_color;
in vec3 v_normal;

uniform vec3 u_light;
uniform float u_flat;

out vec4 o_color;

void main() {
  float lambert = max(dot(normalize(v_normal), normalize(u_light)), 0.0);
  float shade = mix(0.42 + 0.58 * lambert, 1.0, u_flat);
  o_color = vec4(v_color * shade, 1.0);
}
