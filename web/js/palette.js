const golden_angle = 137.508;
const hue_offset = 8;
const other_gray = [0.45, 0.47, 0.5];
const min_luma = 0.3;

const official_colors = {
  "Go": "#00add8",
  "C": "#555555",
  "C++": "#f34b7d",
  "C#": "#178600",
  "Objective-C": "#438eff",
  "Java": "#b07219",
  "Kotlin": "#a97bff",
  "Scala": "#c22d40",
  "Groovy": "#e69f56",
  "JavaScript": "#f1e05a",
  "TypeScript": "#3178c6",
  "Python": "#3572a5",
  "Cython": "#fedf5b",
  "Ruby": "#701516",
  "PHP": "#4f5d95",
  "Perl": "#0298c3",
  "Lua": "#000080",
  "Rust": "#dea584",
  "Swift": "#f05138",
  "Dart": "#00b4ab",
  "Haskell": "#5e5086",
  "Elm": "#60b5cc",
  "PureScript": "#1d222d",
  "Lean": "#7b68ee",
  "Elixir": "#6e4a7e",
  "Erlang": "#b83998",
  "Clojure": "#db5855",
  "OCaml": "#ef7a08",
  "F#": "#b845fc",
  "Julia": "#a270ba",
  "R": "#198ce7",
  "Zig": "#ec915c",
  "Nim": "#ffc200",
  "Crystal": "#000100",
  "Odin": "#60affe",
  "Gleam": "#ffaff3",
  "D": "#ba595e",
  "Solidity": "#aa6746",
  "Assembly": "#6e4c13",
  "Shell": "#89e051",
  "PowerShell": "#012456",
  "Batch": "#c1f12e",
  "Awk": "#c4a27d",
  "Tcl": "#e4cc98",
  "Makefile": "#427819",
  "CMake": "#da3434",
  "Meson": "#007800",
  "Dockerfile": "#384d54",
  "PKGBUILD": "#1793d1",
  "Starlark": "#76d275",
  "Nix": "#7e7eff",
  "SQL": "#e38c00",
  "GLSL": "#5686a5",
  "HLSL": "#aace60",
  "Vim Script": "#199f4b",
  "Emacs Lisp": "#c065db",
  "Common Lisp": "#3fb68b",
  "Scheme": "#1e4aec",
  "Racket": "#3c5caa",
  "Fortran": "#4d41b1",
  "Pascal": "#e3f171",
  "Ada": "#02f88c",
  "VHDL": "#adb2cb",
  "Verilog": "#b2b7f8",
  "SystemVerilog": "#dae1c2",
  "Visual Basic": "#945db7",
  "WebAssembly": "#654ff0",
  "Protocol Buffers": "#a3c4f3",
  "GraphQL": "#e10098",
  "Terraform": "#844fba",
  "Prolog": "#74283c",
  "HTML": "#e34c26",
  "XML": "#0060ac",
  "Vue": "#41b883",
  "Svelte": "#ff3e00",
  "Handlebars": "#f7931e",
  "CSS": "#663399",
  "SCSS": "#c6538c",
  "Sass": "#a53b70",
  "Less": "#1d365d",
  "LaTeX": "#3d6117",
  "Typst": "#239dad",
  "Markdown": "#083fa1",
  "reStructuredText": "#b0bec5",
  "AsciiDoc": "#73a0c5",
  "Org": "#77aa99",
  "Plain Text": "#9aa0a6",
  "JSON": "#292929",
  "JSON with Comments": "#8a8a8a",
  "YAML": "#cb171e",
  "TOML": "#9c4221",
  "INI": "#d1dbe0",
  "Properties": "#c9a27a",
  "Dotenv": "#faf743",
  "Ignore File": "#7f7f7f",
  "CSV": "#237346",
  "Diff": "#88ddff",
  "Log": "#7f8c8d",
};

function official_color(name, out) {
  const hex = official_colors[name];
  if (!hex) {
    return null;
  }
  const value = parseInt(hex.slice(1), 16);
  out[0] = ((value >> 16) & 255) / 255;
  out[1] = ((value >> 8) & 255) / 255;
  out[2] = (value & 255) / 255;
  const luma = 0.2126 * out[0] + 0.7152 * out[1] + 0.0722 * out[2];
  if (luma < min_luma) {
    const lift = (min_luma - luma) / (1 - luma);
    out[0] += (1 - out[0]) * lift;
    out[1] += (1 - out[1]) * lift;
    out[2] += (1 - out[2]) * lift;
  }
  return out;
}

function hsl_to_rgb(h, s, l, out) {
  const c = (1 - Math.abs(2 * l - 1)) * s;
  const hp = (((h % 360) + 360) % 360) / 60;
  const x = c * (1 - Math.abs((hp % 2) - 1));
  let r = 0;
  let g = 0;
  let b = 0;
  if (hp < 1) {
    r = c;
    g = x;
  } else if (hp < 2) {
    r = x;
    g = c;
  } else if (hp < 3) {
    g = c;
    b = x;
  } else if (hp < 4) {
    g = x;
    b = c;
  } else if (hp < 5) {
    r = x;
    b = c;
  } else {
    r = c;
    b = x;
  }
  const m = l - c / 2;
  out[0] = r + m;
  out[1] = g + m;
  out[2] = b + m;
  return out;
}

export function language_color(index, is_other, out) {
  if (is_other) {
    out[0] = other_gray[0];
    out[1] = other_gray[1];
    out[2] = other_gray[2];
    return out;
  }
  const lightness = 0.5 + 0.08 * ((index % 3) - 1);
  return hsl_to_rgb(hue_offset + index * golden_angle, 0.7, lightness, out);
}

export function build_palette(languages, ranking) {
  const count = languages.length;
  const rank_of = new Array(count).fill(-1);
  ranking.forEach((language_index, rank) => {
    rank_of[language_index] = rank;
  });
  let next_rank = ranking.length;
  const data = new Uint8Array(count * 4);
  const tmp = [0, 0, 0];
  const css = new Array(count);
  for (let i = 0; i < count; i++) {
    const rank = rank_of[i] >= 0 ? rank_of[i] : next_rank++;
    if (!official_color(languages[i].name, tmp)) {
      language_color(rank, languages[i].name === "Other", tmp);
    }
    data[i * 4] = Math.round(tmp[0] * 255);
    data[i * 4 + 1] = Math.round(tmp[1] * 255);
    data[i * 4 + 2] = Math.round(tmp[2] * 255);
    data[i * 4 + 3] = 255;
    css[i] = "rgb(" + data[i * 4] + " " + data[i * 4 + 1] + " " + data[i * 4 + 2] + ")";
  }
  return { count, data, css };
}
