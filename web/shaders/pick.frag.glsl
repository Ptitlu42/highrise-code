#version 300 es
precision highp float;

flat in vec4 v_id;

out vec4 o_color;

void main() {
  o_color = v_id;
}
