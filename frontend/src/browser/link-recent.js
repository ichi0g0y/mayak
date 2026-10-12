// What the Host detected last, kept for its Clients (transport.js).
//
// A Client in the background can miss a detection without anyone knowing:
// the relay keeps its socket while macOS holds the page back, or the socket
// is gone after a sleep and nobody has noticed yet. So the Host keeps the
// latest detection of each kind (task, map, position, item) for RECENT_MS,
// sent or not, and says them again whenever a Client says hello. Each has an
// id, and a Client shows an id once (Shown), so one it did get does not open
// again.

export const RECENT_MS = 30 * 60_000

// Recent is the Host's latest detection of each kind.
export class Recent {
  constructor() {
    /** @type {Map<string, {event:string, args:any[], id:string, at:number}>} */
    this.byEvent = new Map()
  }
  // remember keeps message as the latest of its kind and gives it an id.
  remember(message, now = Date.now()) {
    const kept = { event: message.event, args: message.args, id: crypto.randomUUID(), at: now }
    this.byEvent.set(message.event, kept)
    return kept
  }
  // due is what is said again at a hello: those kept less than RECENT_MS,
  // oldest first, so the last detection still decides the tab shown.
  due(now = Date.now()) {
    return [...this.byEvent.values()].filter((k) => now - k.at < RECENT_MS).sort((a, b) => a.at - b.at)
  }
}

// Shown is the id of the detection of each kind a Client showed last.
export class Shown {
  constructor() {
    /** @type {Map<string, string>} */
    this.ids = new Map()
  }
  // fresh tells whether a detection is new here, and marks it shown. One
  // without an id (from a Host before ids) is always new.
  fresh(event, id) {
    if (typeof id !== 'string' || !id || id.length > 64) return true
    if (this.ids.get(event) === id) return false
    this.ids.set(event, id)
    return true
  }
}
