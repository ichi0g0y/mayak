// Adds the element hiding stylesheet, once per document (see cosmeticCall).
// It runs as the document is created (before the page has any elements) or,
// failing that, once it is parsed. css goes on as it is. generic is
// selectors, one a line, each starting with the class (".ad") or id ("#ad")
// it needs: a rule is added once an element has that class or id, so the
// page is not restyled against thousands of rules that cannot match anything
// on it.
(css, generic) => {
  if (window.__mayakSheet) return;
  let sheet, adopted = false;
  if ("adoptedStyleSheets" in document) {
    sheet = new CSSStyleSheet();
    sheet.replaceSync(css);
    adopted = true;
  } else {
    const style = document.createElement("style");
    style.textContent = css;
    (document.head || document.documentElement).appendChild(style);
    sheet = style.sheet;
  }
  // collapse.js reads the selectors from here.
  Object.defineProperty(window, "__mayakSheet", { value: sheet });
  // A page may set its own list of sheets: put ours back.
  const keep = () => {
    if (adopted && !document.adoptedStyleSheets.includes(sheet)) document.adoptedStyleSheets = [...document.adoptedStyleSheets, sheet];
  };
  keep();
  const tidy = () => { if (window.__mayakTidy) window.__mayakTidy(); };
  if (!generic) return tidy();
  const rules = new Map();
  for (const selector of generic.split("\n")) {
    const key = /^[.#][\w-]+/.exec(selector)[0];
    const list = rules.get(key);
    if (list) list.push(selector);
    else rules.set(key, [selector]);
  }
  const seen = new Set();
  let added = false;
  const check = key => {
    if (seen.has(key)) return;
    seen.add(key);
    for (const selector of rules.get(key) || []) {
      try {
        sheet.insertRule(selector + "{display:none!important;--rl:1}", sheet.cssRules.length);
        added = true;
      } catch {} // a selector the browser does not take
    }
  };
  const look = el => {
    if (el.id) check("#" + el.id);
    for (const name of el.classList) check("." + name);
  };
  const apply = () => {
    keep();
    if (!added) return;
    added = false;
    tidy();
  };
  for (const el of document.querySelectorAll("[id],[class]")) look(el);
  apply();
  // Elements added, or given a class or id, are looked at before they are
  // painted (the observer runs before the next frame).
  new MutationObserver(records => {
    for (const r of records) {
      if (r.type === "attributes") {
        look(r.target);
        continue;
      }
      for (const node of r.addedNodes) {
        if (node.nodeType !== 1) continue;
        look(node);
        if (node.firstElementChild) for (const el of node.querySelectorAll("[id],[class]")) look(el);
      }
    }
    apply();
  }).observe(document, { childList: true, subtree: true, attributes: true, attributeFilter: ["class", "id"] });
}
