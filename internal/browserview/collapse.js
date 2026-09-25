// Runs in every tab before the page's own scripts (see filter_windows.go).
// Hiding an ad often leaves its slot behind: wrappers that reserve a height,
// margins, an "Advertisement" label. Starting from the elements MAYAK hid,
// this collapses each ancestor that has nothing visible left in it.
(() => {
  const blocked = new Set();
  const skipTags = new Set(["SCRIPT", "STYLE", "LINK", "META", "TEMPLATE", "NOSCRIPT", "BR", "HEAD", "TITLE", "BASE"]);
  const mediaSelector = "img,video,iframe,canvas,svg,picture,object,embed,input,button,select,textarea";
  const sized = el => { const r = el.getBoundingClientRect(); return r.width > 1 && r.height > 1; };
  const hide = (el, reason) => { el.style.setProperty("display", "none", "important"); el.setAttribute("data-mayak-hidden", reason); };
  const marked = el => getComputedStyle(el).getPropertyValue("--rl").trim() === "1";
  // The element stylesheet sets an inherited --rl on every element it hides;
  // only the outermost one is where an ad was.
  const hiddenByUs = el => el.hasAttribute("data-mayak-hidden") || (marked(el) && !(el.parentElement && marked(el.parentElement)));
  const hasContent = el => {
    if (el.innerText && el.innerText.trim()) return true; // innerText skips hidden text
    for (const m of el.querySelectorAll(mediaSelector)) if (sized(m)) return true;
    if (getComputedStyle(el).backgroundImage !== "none") return true;
    const all = el.querySelectorAll("*");
    if (all.length > 200) return true; // a real layout region, not an ad slot
    for (const d of all) if (sized(d) && getComputedStyle(d).backgroundImage !== "none") return true;
    return false;
  };
  let active = false, timer = 0, lastRun = 0;
  const tidy = () => {
    timer = 0; lastRun = Date.now();
    if (!document.body) return;
    const seeds = [];
    for (const el of document.body.querySelectorAll("*")) {
      if (skipTags.has(el.tagName) || el.getClientRects().length) continue;
      if (hiddenByUs(el)) seeds.push(el);
    }
    for (const seed of seeds) {
      let parent = seed.parentElement;
      for (let depth = 0; parent && parent !== document.body && depth < 8; depth++) {
        if (parent.hasAttribute("data-mayak-hidden") || !parent.getClientRects().length || hasContent(parent)) break;
        hide(parent, "empty");
        parent = parent.parentElement;
      }
    }
  };
  // Ads arrive late; re-check after DOM changes, at most every 1.5 seconds.
  const schedule = () => { if (active && !timer) timer = setTimeout(tidy, Math.max(400, 1500 - (Date.now() - lastRun))); };
  const activate = () => {
    if (active) return;
    active = true;
    new MutationObserver(schedule).observe(document.documentElement, { childList: true, subtree: true });
    schedule();
  };
  const collapseBlocked = urls => {
    for (const u of urls) blocked.add(u);
    for (const el of document.querySelectorAll("img,iframe,frame,embed,object,video,audio,source")) {
      const src = el.currentSrc || el.src || el.data || "";
      if (!blocked.has(src)) continue;
      const target = el.tagName === "SOURCE" ? el.parentElement : el;
      if (target) hide(target, "blocked");
    }
    activate();
  };
  // Pages can call these too, but they only hide the page's own elements.
  Object.defineProperty(window, "__mayakCollapse", { value: collapseBlocked });
  Object.defineProperty(window, "__mayakTidy", { value: activate });
})();
