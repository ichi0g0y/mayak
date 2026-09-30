// The menu window (app_menu.go): a menu the shell opens over a page, which
// the shell itself cannot cover. It draws the items it is sent, reports its
// height so the window fits it, and passes the choice back.
import './style.css'
import { Events } from '@wailsio/runtime'
import {
  BrowserMenuReady,
  BrowserMenuChoose,
  BrowserMenuCurrent,
} from '../../bindings/github.com/local/mayak/internal/app/app'
import { icon, esc } from './icons.js'

let current = null
function draw(menu) {
  current = menu
  if (menu.theme) document.documentElement.dataset.theme = menu.theme
  document.documentElement.lang = menu.language || 'ja'
  const rows = (menu.items || [])
    .map((item) => {
      if (item.kind === 'label') return `<div class="snap-menu-label">${esc(item.title)}</div>`
      if (item.kind === 'card') {
        const facts = (item.facts || [])
          .map((f) => `<div><dt>${esc(f.label)}</dt><dd>${esc(f.value)}</dd></div>`)
          .join('')
        const mark = /^#[0-9a-f]{6}$/i.test(item.color || '')
          ? `<span class="menu-card-dot" style="--c:${item.color}"></span>`
          : ''
        return `<div class="menu-card">${item.thumb ? `<img class="menu-card-picture" src="${esc(item.thumb)}" alt="">` : ''}<div class="menu-card-body"><div class="menu-card-head">${mark}<strong>${esc(item.title)}</strong>${item.hint ? `<small>${esc(item.hint)}</small>` : ''}</div>${facts ? `<dl>${facts}</dl>` : ''}</div></div>`
      }
      const picture = item.thumb
        ? `<span class="snap-menu-thumb"><img src="${esc(item.thumb)}" alt=""></span>`
        : item.icon
          ? icon(item.icon)
          : ''
      return `<button class="snap-menu-item" role="menuitem" data-id="${esc(item.id)}">${picture}<span><strong>${esc(item.title)}</strong>${item.hint ? `<small>${esc(item.hint)}</small>` : ''}</span></button>`
    })
    .join('')
  document.querySelector('#app').innerHTML = `<div class="snap-menu menu-window" role="menu">${rows}</div>`
  requestAnimationFrame(() => {
    const box = document.querySelector('.menu-window').getBoundingClientRect()
    void BrowserMenuReady(Math.ceil(box.height))
    document.querySelector('.snap-menu-item')?.focus()
  })
}
Events.On('menu:page', (event) => draw(event.data))
// The window's first menu was sent before this page loaded: ask for it.
BrowserMenuCurrent()
  .then((menu) => {
    if (menu && !current) draw(menu)
  })
  .catch(() => {})
document.addEventListener('click', (event) => {
  const row = event.target.closest('[data-id]')
  if (row && current) void BrowserMenuChoose(row.dataset.id)
})
document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape') {
    event.preventDefault()
    void BrowserMenuChoose('')
    return
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const rows = [...document.querySelectorAll('.snap-menu-item')]
    const at = rows.indexOf(document.activeElement)
    rows[(at + (event.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length]?.focus()
  }
})
