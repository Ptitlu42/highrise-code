const golden_angle = 137.508;
const hue_offset = 8;
const other_gray = [0.45, 0.47, 0.5];

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
    language_color(rank, languages[i].name === "Other", tmp);
    data[i * 4] = Math.round(tmp[0] * 255);
    data[i * 4 + 1] = Math.round(tmp[1] * 255);
    data[i * 4 + 2] = Math.round(tmp[2] * 255);
    data[i * 4 + 3] = 255;
    css[i] = "rgb(" + data[i * 4] + " " + data[i * 4 + 1] + " " + data[i * 4 + 2] + ")";
  }
  return { count, data, css };
}
