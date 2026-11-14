const base64_chunk = 1 << 20;

export function is_standalone() {
  return document.getElementById("hr_summary") !== null;
}

function decode_base64(text) {
  const clean = text.replace(/\s+/g, "");
  const parts = [];
  let total = 0;
  for (let i = 0; i < clean.length; i += base64_chunk) {
    const binary = atob(clean.slice(i, i + base64_chunk));
    const bytes = new Uint8Array(binary.length);
    for (let j = 0; j < binary.length; j++) {
      bytes[j] = binary.charCodeAt(j);
    }
    parts.push(bytes);
    total += bytes.length;
  }
  const out = new Uint8Array(total);
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

async function gunzip(bytes) {
  const stream = new Blob([bytes]).stream().pipeThrough(new DecompressionStream("gzip"));
  return new Response(stream).arrayBuffer();
}

export async function inline_binary(id) {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error("missing embedded data: " + id);
  }
  return gunzip(decode_base64(element.textContent));
}

export async function inline_json(id) {
  const buffer = await inline_binary(id);
  return JSON.parse(new TextDecoder().decode(buffer));
}

export function inline_text(id) {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error("missing embedded text: " + id);
  }
  return element.textContent;
}

export function summary_inline() {
  return JSON.parse(inline_text("hr_summary"));
}

async function checked_fetch(url) {
  const response = await fetch(url, { cache: "no-store", credentials: "same-origin" });
  if (!response.ok) {
    throw new Error(url + ": " + response.status);
  }
  return response;
}

export async function fetch_json(url) {
  return (await checked_fetch(url)).json();
}

export async function fetch_buffer(url) {
  return (await checked_fetch(url)).arrayBuffer();
}

export async function fetch_text(url) {
  return (await checked_fetch(url)).text();
}

export async function load_shader(name) {
  if (is_standalone()) {
    return inline_text("hr_shader_" + name.replace(/\./g, "_"));
  }
  return fetch_text("/shaders/" + name + ".glsl");
}
