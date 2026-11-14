const int_format = new Intl.NumberFormat("en-US");
const float_format = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2, minimumFractionDigits: 0 });
const iec_units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];

export function format_int(value) {
  return int_format.format(value);
}

export function format_float(value) {
  return float_format.format(value);
}

export function format_percent(value) {
  return float_format.format(value) + "%";
}

export function format_bytes(value) {
  let unit = 0;
  let scaled = value;
  while (scaled >= 1024 && unit < iec_units.length - 1) {
    scaled /= 1024;
    unit++;
  }
  if (unit === 0) {
    return int_format.format(value) + " B";
  }
  return scaled.toFixed(2) + " " + iec_units[unit];
}

export function format_duration(seconds) {
  if (seconds < 1) {
    return Math.round(seconds * 1000) + " ms";
  }
  return seconds.toFixed(2) + " s";
}
