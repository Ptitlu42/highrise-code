import {
  fetch_buffer,
  fetch_json,
  inline_binary,
  inline_json,
  is_standalone,
  summary_inline,
} from "hr/assets";

const header_bytes = 16;
const magic = [0x48, 0x52, 0x4c, 0x31];

export const kind_dir = 0;
export const kind_file = 1;
export const kind_aggregate = 2;

const kind_names = ["dir", "file", "aggregate"];

let standalone_names = null;
let standalone_layout = null;

export async function load_summary() {
  if (is_standalone()) {
    return summary_inline();
  }
  return fetch_json("/api/summary");
}

export function parse_layout(buffer) {
  const bytes = new Uint8Array(buffer, 0, header_bytes);
  for (let i = 0; i < magic.length; i++) {
    if (bytes[i] !== magic[i]) {
      throw new Error("invalid layout data");
    }
  }
  const view = new DataView(buffer);
  const count = view.getUint32(4, true);
  const world = view.getFloat32(8, true);
  const depth_reached = view.getUint32(12, true);
  let offset = header_bytes;
  const take_f32 = () => {
    const array = new Float32Array(buffer, offset, count);
    offset += count * 4;
    return array;
  };
  const take_u32 = () => {
    const array = new Uint32Array(buffer, offset, count);
    offset += count * 4;
    return array;
  };
  const x = take_f32();
  const y = take_f32();
  const w = take_f32();
  const h = take_f32();
  const lines = take_u32();
  const code = take_u32();
  const comment = take_u32();
  const bytes_col = take_u32();
  const chars = take_u32();
  const nonws = take_u32();
  const parent = new Int32Array(buffer, offset, count);
  offset += count * 4;
  const lang = new Uint16Array(buffer, offset, count);
  offset += count * 2;
  const depth = new Uint16Array(buffer, offset, count);
  offset += count * 2;
  const kind = new Uint8Array(buffer, offset, count);

  const rect = new Float32Array(count * 4);
  const values = new Float32Array(count * 4);
  const cls = new Float32Array(count * 4);
  let max_lines = 1;
  let max_code = 1;
  let max_bytes = 1;
  let dir_count = 0;
  for (let i = 0; i < count; i++) {
    const j = i * 4;
    rect[j] = x[i];
    rect[j + 1] = y[i];
    rect[j + 2] = w[i];
    rect[j + 3] = h[i];
    values[j] = lines[i];
    values[j + 1] = code[i];
    values[j + 2] = bytes_col[i];
    values[j + 3] = depth[i];
    cls[j] = lang[i];
    cls[j + 1] = kind[i];
    if (kind[i] !== kind_dir) {
      if (lines[i] > max_lines) max_lines = lines[i];
      if (code[i] > max_code) max_code = code[i];
      if (bytes_col[i] > max_bytes) max_bytes = bytes_col[i];
    }
    if (kind[i] !== kind_file) dir_count++;
  }
  return {
    count,
    world,
    depth_reached,
    x, y, w, h,
    lines, code, comment, bytes: bytes_col, chars, nonws,
    parent, lang, depth, kind,
    rect, values, cls,
    max_lines, max_code, max_bytes,
    dir_count,
    dir_ids: null,
    dir_names: null,
  };
}

export async function load_layout(max_depth) {
  let buffer;
  if (is_standalone()) {
    if (!standalone_layout) {
      standalone_layout = await inline_binary("hr_layout");
    }
    buffer = standalone_layout;
  } else {
    buffer = await fetch_buffer("/api/layout?max_depth=" + max_depth);
  }
  return parse_layout(buffer);
}

async function names_standalone() {
  if (!standalone_names) {
    standalone_names = await inline_json("hr_names");
  }
  return standalone_names;
}

export async function load_dir_names(layout, max_depth) {
  if (is_standalone()) {
    const names = await names_standalone();
    const ids = new Int32Array(layout.dir_count);
    const out = new Array(layout.dir_count);
    let n = 0;
    for (let i = 0; i < layout.count; i++) {
      if (layout.kind[i] !== kind_file) {
        ids[n] = i;
        out[n] = names[i];
        n++;
      }
    }
    layout.dir_ids = ids;
    layout.dir_names = out;
    return;
  }
  const data = await fetch_json("/api/dirs?max_depth=" + max_depth);
  layout.dir_ids = Int32Array.from(data.ids);
  layout.dir_names = data.names;
}

function path_of(layout, names, id) {
  const parts = [];
  for (let i = id; i > 0; i = layout.parent[i]) {
    parts.push(names[i]);
  }
  return parts.reverse().join("/");
}

function subtree_end(layout, id) {
  const d = layout.depth[id];
  let j = id + 1;
  if (layout.kind[id] === kind_file) {
    return j;
  }
  while (j < layout.count && layout.depth[j] > d) {
    j++;
  }
  return j;
}

function describe_standalone(layout, names, languages, id) {
  const kind = kind_names[layout.kind[id]];
  const lines = layout.lines[id];
  const code = layout.code[id];
  const comment = layout.comment[id];
  const detail = {
    id,
    kind,
    name: names[id],
    path: path_of(layout, names, id),
    depth: layout.depth[id],
    lang_index: layout.lang[id],
    language: languages[layout.lang[id]].name,
    category: languages[layout.lang[id]].category,
    bytes: layout.bytes[id],
    lines,
    code,
    comment,
    blank: Math.max(0, lines - code - comment),
    chars: layout.chars[id],
    chars_non_whitespace: layout.nonws[id],
    average_chars_per_line: lines > 0 ? Math.round((layout.chars[id] / lines) * 100) / 100 : 0,
    files: 1,
    dirs: 0,
  };
  if (kind === "file") {
    return detail;
  }
  const end = subtree_end(layout, id);
  const counts = new Map();
  let files = 0;
  let dirs = 0;
  for (let j = id + 1; j < end; j++) {
    if (layout.kind[j] === kind_file) {
      files++;
      const entry = counts.get(layout.lang[j]) || { files: 0, lines: 0 };
      entry.files++;
      entry.lines += layout.lines[j];
      counts.set(layout.lang[j], entry);
    } else {
      dirs++;
    }
  }
  detail.files = files;
  detail.dirs = dirs;
  detail.languages = Array.from(counts, ([lang, entry]) => ({
    name: languages[lang].name,
    files: entry.files,
    lines: entry.lines,
  }))
    .sort((a, b) => b.lines - a.lines || b.files - a.files)
    .slice(0, 16);
  return detail;
}

export async function load_node(layout, languages, max_depth, id) {
  if (is_standalone()) {
    const names = await names_standalone();
    return describe_standalone(layout, names, languages, id);
  }
  return fetch_json("/api/node?max_depth=" + max_depth + "&id=" + id);
}

export async function search_nodes(layout, max_depth, query) {
  if (is_standalone()) {
    const names = await names_standalone();
    const needle = query.toLowerCase();
    const by_path = needle.includes("/");
    const out = [];
    for (let i = 1; i < layout.count && out.length < 50; i++) {
      const hay = by_path ? path_of(layout, names, i) : names[i];
      if (hay.toLowerCase().includes(needle)) {
        out.push({ id: i, path: path_of(layout, names, i), kind: kind_names[layout.kind[i]] });
      }
    }
    return out;
  }
  return fetch_json("/api/search?max_depth=" + max_depth + "&q=" + encodeURIComponent(query));
}
