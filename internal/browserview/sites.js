// Tidies pages the built-in browser opens, whatever the ad blocker does. It
// runs before each document's own scripts (view_windows.go, view_darwin.go).
//
// A Fandom wiki (the official EFT wiki is one) fills its right rail with a
// sign-up card ("New to Fandom?") and ads while signed out, which a page
// translated through Google (translate.goog) always is, and the article gets
// a narrow column. The rail is hidden on Fandom's wikis, translated or not,
// signed in or out.
(() => {
  try {
    if (!/(^|\.)fandom\.com$|-fandom-com\.translate\.goog$/.test(location.hostname)) return
    const css = '.page__right-rail{display:none!important}'
    const add = () => {
      if (document.getElementById('mayak-site')) return
      const style = document.createElement('style')
      style.id = 'mayak-site'
      style.textContent = css
      ;(document.head || document.documentElement).appendChild(style)
    }
    if (document.documentElement) add()
    else document.addEventListener('DOMContentLoaded', add)
  } catch {}
})()
