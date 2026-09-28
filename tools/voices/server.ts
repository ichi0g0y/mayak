// The built-in voices' editor, for development only (task dev:voice): a page
// with each pack's lines in a table, saved to its pack.json as they are
// typed; the lines changed since their WAV was made are marked and made
// again from the page (with a progress bar); each line, or a whole pack in a
// row, plays as it is now. It listens on 127.0.0.1 only.
//
// It reloads itself: task dev:voice runs it with bun --watch (a change to its
// code restarts it, and the page reconnects and reads the state again), a
// change to editor.html reloads the page, and a pack.json changed from
// outside (by hand, task voices) shows at once. The page saves only the lines
// it changed, so what changed outside is not written over.
import { createHash } from 'node:crypto'
import { readdirSync, readFileSync, statSync, watch } from 'node:fs'
import { join, normalize, relative } from 'node:path'
import { decodeWave, pcmWave } from './adpcm'
import {
  kinds,
  kokoroVoices,
  lineFile,
  lineHash,
  loadManifest,
  loadPack,
  type Pack,
  packFile,
  packIDs,
  root,
  savePack,
  voicevoxReady,
  voicevoxStyles,
  writeLine,
} from './voices'

const port = Number(process.env.VOICE_EDITOR_PORT ?? 5178)
const samplesDir = join(import.meta.dir, 'samples')
const editorFile = join(import.meta.dir, 'editor.html')

// The generation under way: one at a time, its progress sent to every page.
type Job = { running: boolean; done: number; total: number; current: string; errors: string[] }
let job: Job = { running: false, done: 0, total: 0, current: '', errors: [] }
// What the pages are told: the job, the page itself changed (reload), the
// packs changed on disk, and on connecting which page they should be.
type Message = { type: 'job'; job: Job } | { type: 'reload' } | { type: 'packs' } | { type: 'hello'; page: string }
const listeners = new Set<(message: Message) => void>()
const broadcast = (message: Message) => listeners.forEach((send) => send(message))
const announce = () => broadcast({ type: 'job', job })
const pageVersion = () => createHash('sha1').update(readFileSync(editorFile)).digest('hex').slice(0, 12)

// A pack written from the page is not news to it; one written from outside is.
const ownWrites = new Map<string, number>()
let packsTimer: ReturnType<typeof setTimeout> | undefined
watch(root, { recursive: true }, (_event, name) => {
  if (!name || !String(name).endsWith('pack.json')) return
  if (Date.now() - (ownWrites.get(join(root, String(name))) ?? 0) < 1500) return
  clearTimeout(packsTimer)
  packsTimer = setTimeout(() => broadcast({ type: 'packs' }), 300)
})
let pageTimer: ReturnType<typeof setTimeout> | undefined
watch(editorFile, () => {
  clearTimeout(pageTimer)
  pageTimer = setTimeout(() => broadcast({ type: 'reload' }), 200)
})

async function generate(items: { id: string; kind: string }[]) {
  job = { running: true, done: 0, total: items.length, current: '', errors: [] }
  announce()
  const packs = new Map<string, Pack>()
  for (const { id, kind } of items) {
    job.current = `${id}/${kind}`
    announce()
    try {
      if (!packs.has(id)) packs.set(id, await loadPack(id))
      await writeLine(id, packs.get(id)!, kind)
    } catch (e) {
      job.errors.push(`${id}/${kind}: ${e instanceof Error ? e.message : String(e)}`)
    }
    job.done++
    announce()
  }
  job.running = false
  job.current = ''
  announce()
}

async function state() {
  const manifest = await loadManifest()
  const ready = await voicevoxReady()
  const packs = []
  for (const id of packIDs()) {
    const pack = await loadPack(id)
    const stale = kinds.filter((kind) => manifest[id]?.[kind] !== lineHash(pack, kind))
    const styles = pack.voicevox && ready ? await voicevoxStyles(pack.voicevox.speaker).catch(() => []) : []
    packs.push({ id, pack, stale, styles })
  }
  packs.sort((a, b) =>
    a.pack.language !== b.pack.language ? (a.pack.language === 'ja' ? -1 : 1) : a.pack.order - b.pack.order,
  )
  return { kinds, packs, voicevox: ready, kokoroVoices, job }
}

function samples(): string[] {
  const out: string[] = []
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const path = join(dir, entry.name)
      if (entry.isDirectory()) walk(path)
      else if (entry.name.endsWith('.wav')) out.push(relative(samplesDir, path).replaceAll('\\', '/'))
    }
  }
  try {
    walk(samplesDir)
  } catch {}
  return out.sort()
}

// A WAV as PCM (ADPCM decoded), which the browser plays.
async function playable(path: string) {
  const { rate, samples } = decodeWave(await Bun.file(path).arrayBuffer())
  return new Response(pcmWave(rate, samples), { headers: { 'content-type': 'audio/wav', 'cache-control': 'no-store' } })
}

const json = (data: unknown, status = 200) =>
  new Response(JSON.stringify(data), { status, headers: { 'content-type': 'application/json' } })

Bun.serve({
  hostname: '127.0.0.1',
  port,
  idleTimeout: 0,
  async fetch(req) {
    const url = new URL(req.url)
    const path = url.pathname
    try {
      if (path === '/') return new Response(Bun.file(editorFile), { headers: { 'cache-control': 'no-store' } })
      if (path === '/api/state') return json(await state())
      if (path === '/api/samples') return json(samples())
      // The lines and engine settings the page changed, put into the pack as
      // it is on disk now.
      const packMatch = path.match(/^\/api\/packs\/([a-z0-9-]+)$/)
      if (packMatch && req.method === 'PATCH') {
        const id = packMatch[1]
        const current = await loadPack(id)
        const change = (await req.json()) as Partial<Pick<Pack, 'lines' | 'voicevox' | 'kokoro'>>
        const saved: Pack = { ...current, lines: { ...current.lines } }
        for (const [kind, line] of Object.entries(change.lines ?? {}))
          if ((kinds as readonly string[]).includes(kind)) saved.lines[kind] = line
        if (current.voicevox && change.voicevox) saved.voicevox = change.voicevox
        if (current.kokoro && change.kokoro) saved.kokoro = change.kokoro
        ownWrites.set(packFile(id), Date.now())
        await savePack(id, saved)
        return json(await state())
      }
      if (path === '/api/generate' && req.method === 'POST') {
        if (job.running) return json({ error: 'a generation is under way' }, 409)
        const items = ((await req.json()) as { id: string; kind: string }[]).filter(
          (item) => packIDs().includes(item.id) && (kinds as readonly string[]).includes(item.kind),
        )
        void generate(items)
        return json(job)
      }
      if (path === '/api/events') {
        let send: (message: Message) => void = () => {}
        const stream = new ReadableStream({
          start(controller) {
            send = (message) => controller.enqueue(`data: ${JSON.stringify(message)}\n\n`)
            listeners.add(send)
            // After a restart the page reconnects within half a second.
            controller.enqueue('retry: 500\n\n')
            send({ type: 'hello', page: pageVersion() })
            send({ type: 'job', job })
          },
          cancel() {
            listeners.delete(send)
          },
        })
        return new Response(stream, {
          headers: { 'content-type': 'text/event-stream', 'cache-control': 'no-store', connection: 'keep-alive' },
        })
      }
      const audioMatch = path.match(/^\/audio\/([a-z0-9-]+)\/([a-zA-Z]+)\.wav$/)
      if (audioMatch && packIDs().includes(audioMatch[1]) && (kinds as readonly string[]).includes(audioMatch[2]))
        return await playable(lineFile(audioMatch[1], audioMatch[2]))
      if (path.startsWith('/samples/')) {
        const file = normalize(join(samplesDir, decodeURIComponent(path.slice('/samples/'.length))))
        if (!file.startsWith(samplesDir) || !statSync(file).isFile()) return new Response('not found', { status: 404 })
        return await playable(file)
      }
      return new Response('not found', { status: 404 })
    } catch (e) {
      return json({ error: e instanceof Error ? e.message : String(e) }, 500)
    }
  },
})

const address = `http://127.0.0.1:${port}/`
console.log(`Voice editor: ${address} (Ctrl+C to stop)`)
// The browser opens once: after a restart, the page already open reconnects.
setTimeout(() => {
  if (process.platform === 'win32' && !process.env.VOICE_EDITOR_NO_OPEN && listeners.size === 0)
    Bun.spawn(['cmd', '/c', 'start', '', address])
}, 2500)
