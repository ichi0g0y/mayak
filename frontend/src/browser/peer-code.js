// A pairing's invitation: the Host makes a key for the link (transport.js)
// and hands it to the other PC, as a long code or, through the pairing relay,
// under an 8-digit code. A code is good for ten minutes; once paired, the
// two PCs keep the key and meet again without one.
const PREFIX = 'MAYAK1.'
const MAX_AGE = 10 * 60 * 1000
// The pairing relay (site/worker/index.js) that parks an invitation under an
// 8-digit code for ten minutes so the two PCs need not copy the long code.
const PAIR_RELAY = 'https://mayak.ich.sh/api/pair'
function validate(value, now = Date.now()) {
  if (
    !value ||
    value.version !== 2 ||
    value.type !== 'link' ||
    !/^[-a-f0-9]{36}$/i.test(value.id) ||
    !Number.isFinite(value.createdAt) ||
    now - value.createdAt > MAX_AGE ||
    value.createdAt - now > 60000 ||
    typeof value.key !== 'string' ||
    !/^[A-Za-z0-9_-]{43}$/.test(value.key)
  )
    throw new Error('invalid-or-expired-code')
  return value
}
function encode(value) {
  validate(value)
  return PREFIX + btoa(JSON.stringify(value)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}
function decode(code, now = Date.now()) {
  if (typeof code !== 'string' || code.length > 4096) throw new Error('invalid-or-expired-code')
  code = code.trim()
  if (!code.startsWith(PREFIX) || !/^[A-Za-z0-9_-]+$/.test(code.slice(PREFIX.length)))
    throw new Error('invalid-or-expired-code')
  let value
  try {
    value = JSON.parse(atob(code.slice(PREFIX.length).replace(/-/g, '+').replace(/_/g, '/')))
  } catch {
    throw new Error('invalid-or-expired-code')
  }
  return validate(value, now)
}
export { encode, decode, MAX_AGE, PAIR_RELAY }
