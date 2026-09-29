// Tidies pages the built-in browser opens, whatever the ad blocker does. It
// runs before each document's own scripts (view_windows.go, view_darwin.go).
//
// A Fandom wiki page translated through Google (translate.goog) is always
// signed out, so Fandom fills its right rail with a sign-up card ("New to
// Fandom?") and the article gets a narrow column. The rail is hidden there;
// the wiki itself keeps it (signed in, it can be collapsed from the page).
(() => {
  try {
    if (!/-fandom-com\.translate\.goog$/.test(location.hostname)) return
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
