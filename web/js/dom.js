export function el(tag, class_name, text) {
  const node = document.createElement(tag);
  if (class_name) {
    node.className = class_name;
  }
  if (text !== undefined && text !== null) {
    node.textContent = text;
  }
  return node;
}

export function clear(node) {
  while (node.firstChild) {
    node.removeChild(node.firstChild);
  }
}

export function by_id(id) {
  const node = document.getElementById(id);
  if (!node) {
    throw new Error("missing element #" + id);
  }
  return node;
}
