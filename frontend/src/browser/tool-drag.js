// Reordering the toolbar's icons (translate, snap note, wiki search, external
// browser) by dragging one along the group. A press that does not move is a
// click; a drag moves the icon live and, on release, hands the new order of
// the shown icons to drop. While a drag is on, the shell does not re-render
// (it would put the icons back).
let drag = null,
  suppressClick = false

export const toolDragActive = () => !!drag?.moving

export function installToolDrag({ drop, cancel }) {
  document.addEventListener('pointerdown', (event) => {
    const button = event.target.closest?.('.tool-icons [data-tool]')
    if (!button || event.button !== 0) return
    drag = { button, group: button.parentElement, pointer: event.pointerId, x: event.clientX, moving: false }
  })
  document.addEventListener('pointermove', (event) => {
    if (!drag || event.pointerId !== drag.pointer) return
    if (!drag.moving) {
      if (Math.abs(event.clientX - drag.x) < 5) return
      drag.moving = true
      drag.group.classList.add('reordering')
      drag.button.classList.add('dragging')
      try {
        drag.button.setPointerCapture(event.pointerId)
      } catch {}
    }
    // The icon goes before the first other icon whose middle is right of the pointer.
    const others = [...drag.group.querySelectorAll('[data-tool]')].filter((el) => el !== drag.button)
    const next =
      others.find((el) => {
        const r = el.getBoundingClientRect()
        return event.clientX < r.left + r.width / 2
      }) || null
    if (next !== drag.button.nextElementSibling || (!next && drag.button !== drag.group.lastElementChild))
      drag.group.insertBefore(drag.button, next)
  })
  const end = (event) => {
    if (!drag || event.pointerId !== drag.pointer) return
    const done = drag
    drag = null
    if (!done.moving) return
    done.group.classList.remove('reordering')
    done.button.classList.remove('dragging')
    suppressClick = true
    setTimeout(() => {
      suppressClick = false
    }, 0)
    if (event.type === 'pointercancel') {
      cancel?.()
      return
    }
    drop([...done.group.querySelectorAll('[data-tool]')].map((el) => el.dataset.tool))
  }
  document.addEventListener('pointerup', end)
  document.addEventListener('pointercancel', end)
  // The click that ends a drag is not a click on the icon.
  document.addEventListener(
    'click',
    (event) => {
      if (suppressClick && event.target.closest?.('.tool-icons')) {
        event.stopImmediatePropagation()
        event.preventDefault()
        suppressClick = false
      }
    },
    true,
  )
}
