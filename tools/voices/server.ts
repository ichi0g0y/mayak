// The built-in voices' editor, for development only (task dev:voice): a page
// with each pack's lines in a table, saved to its pack.json as they are
// typed; the lines changed since their WAV was made are marked and made
// again from the page (with a progress bar); each line, or a whole pack in a
// row, plays as it is now. It listens on 127.0.0.1 only.
import { readdirSync, statSync } from 'node:fs'
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
  packIDs,
  savePack,
  voicevoxReady,
  voicevoxStyles,
  writeLine,
} from './voices'

const port = Number(process.env.VOICE_EDITOR_PORT ?? 5178)
const samplesDir = join(import.meta.dir, 'samples')

// The generation under way: one at a time, its progress sent to every page.
type Job = { running: boolean; done: number; total: number; current: string; errors: string[] }
let job: Job = { running: false, done: 0, total: 0, current: '', errors: [] }
const listeners = new Set<(job: Job) => void>()
const announce = () => listeners.forEach((send) => send(job))

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
      if (path === '/') return new Response(Bun.file(join(import.meta.dir, 'editor.html')))
      if (path === '/api/state') return json(await state())
      if (path === '/api/samples') return json(samples())
      // A pack's settings and lines, saved as the page sends them.
      const packMatch = path.match(/^\/api\/packs\/([a-z0-9-]+)$/)
      if (packMatch && req.method === 'PUT') {
        const id = packMatch[1]
        const current = await loadPack(id)
        const next = (await req.json()) as Pack
        // Only what the page edits: the lines and the engine settings.
        const saved: Pack = { ...current, lines: next.lines }
        if (current.voicevox && next.voicevox) saved.voicevox = next.voicevox
        if (current.kokoro && next.kokoro) saved.kokoro = next.kokoro
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
        let send: (job: Job) => void = () => {}
        const stream = new ReadableStream({
          start(controller) {
            send = (j) => controller.enqueue(`data: ${JSON.stringify(j)}\n\n`)
            listeners.add(send)
            send(job)
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
if (process.platform === 'win32' && !process.env.VOICE_EDITOR_NO_OPEN) Bun.spawn(['cmd', '/c', 'start', '', address])
