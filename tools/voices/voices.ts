// The built-in voices' packs (internal/sound/voices/<pack>/pack.json) and
// their synthesis, shared by the command line (generate.ts) and the editor
// (server.ts): Japanese with the VOICEVOX engine running locally (the
// VOICEVOX app starts it on 127.0.0.1:50021), English with Kokoro-82M
// (kokoro-js, which downloads the model on first use).
//
// A VOICEVOX line is its text, or {text, style} for another of the speaker's
// styles, {kana} to give its reading and accents in AquesTalk-like notation
// (as the engine's /audio_query returns them), and {query} for the synthesis
// settings of that line (speedScale, pitchScale, intonationScale…). A pack's
// voicevox.query sets them for all of its lines. A Kokoro line is its text,
// or {text, speed}; kokoro.speed sets the pack's. The WAVs are IMA ADPCM
// (adpcm.ts), a quarter of 16-bit PCM.
import { createHash } from 'node:crypto'
import { existsSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { adpcmWave, pcmSamples } from './adpcm'

export const engine = process.env.VOICEVOX_URL ?? 'http://127.0.0.1:50021'
export const root = join(import.meta.dir, '../../internal/sound/voices')
// The notifications (internal/sound/sound.go Kinds).
export const kinds = [
  'quest',
  'taskNotMatched',
  'item',
  'itemNotMatched',
  'error',
  'remoteError',
  'matchFound',
  'raidStart',
  'runThrough',
  'questItems',
  'restartTasks',
] as const

export type LineObject = { text: string; style?: number; kana?: string; query?: Record<string, number>; speed?: number }
export type Line = string | LineObject
export type Pack = {
  name: Record<string, string>
  language: string
  order: number
  credit: string
  terms: string
  termsChecked: string
  voicevox?: { speaker: number; query?: Record<string, number> }
  kokoro?: { voice: string; speed?: number }
  lines: Record<string, Line>
}

export const lineObject = (line: Line | undefined): LineObject =>
  typeof line === 'string' ? { text: line } : (line ?? { text: '' })

export function packIDs(): string[] {
  return readdirSync(root, { withFileTypes: true })
    .filter((e) => e.isDirectory())
    .map((e) => e.name)
}

export const packFile = (id: string) => join(root, id, 'pack.json')
export const lineFile = (id: string, kind: string) => join(root, id, `${kind}.wav`)

// generated.json keeps, a line, a hash of what its WAV was made from (the
// line and the pack's engine settings), so the editor tells the lines
// changed since from those up to date.
const manifestFile = join(import.meta.dir, 'generated.json')
export type Manifest = Record<string, Record<string, string>>

export function lineHash(pack: Pack, kind: string): string {
  const line = lineObject(pack.lines[kind])
  const spec = pack.voicevox
    ? { voicevox: { speaker: pack.voicevox.speaker, query: pack.voicevox.query ?? null }, line }
    : { kokoro: { voice: pack.kokoro?.voice, speed: pack.kokoro?.speed ?? null }, line }
  return createHash('sha1').update(JSON.stringify(spec)).digest('hex').slice(0, 12)
}

export async function loadManifest(): Promise<Manifest> {
  return existsSync(manifestFile) ? Bun.file(manifestFile).json() : {}
}

async function stamp(id: string, kind: string, hash: string) {
  const manifest = await loadManifest()
  manifest[id] = { ...manifest[id], [kind]: hash }
  const sorted = Object.fromEntries(Object.keys(manifest).sort().map((k) => [k, manifest[k]]))
  await Bun.write(manifestFile, JSON.stringify(sorted, null, 2) + '\n')
}

export async function loadPack(id: string): Promise<Pack> {
  if (!packIDs().includes(id)) throw new Error(`no pack ${id}`)
  return Bun.file(packFile(id)).json()
}

export async function savePack(id: string, pack: Pack) {
  await Bun.write(packFile(id), JSON.stringify(pack, null, 2) + '\n')
}

async function post(path: string, body?: unknown) {
  const res = await fetch(engine + path, {
    method: 'POST',
    headers: body === undefined ? undefined : { 'content-type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`${path}: ${res.status} ${await res.text()}`)
  return res
}

// voicevoxReady tells whether the engine answers.
export async function voicevoxReady(): Promise<boolean> {
  try {
    return (await fetch(engine + '/version')).ok
  } catch {
    return false
  }
}

// voicevoxStyles lists a speaker's styles (name and id) for the speaker of a
// style id.
export async function voicevoxStyles(style: number): Promise<{ name: string; id: number }[]> {
  const speakers: { name: string; styles: { name: string; id: number }[] }[] = await (
    await fetch(engine + '/speakers')
  ).json()
  return speakers.find((s) => s.styles.some((st) => st.id === style))?.styles ?? []
}

async function voicevox(pack: NonNullable<Pack['voicevox']>, line: LineObject) {
  const speaker = line.style ?? pack.speaker
  const query = await (await post(`/audio_query?speaker=${speaker}&text=${encodeURIComponent(line.text)}`)).json()
  if (line.kana) {
    query.accent_phrases = await (
      await post(`/accent_phrases?speaker=${speaker}&is_kana=true&text=${encodeURIComponent(line.kana)}`)
    ).json()
  }
  // Short silences around the line: a notification starts at once.
  Object.assign(query, { prePhonemeLength: 0.05, postPhonemeLength: 0.1 }, pack.query, line.query)
  const wav = await (await post(`/synthesis?speaker=${speaker}`, query)).arrayBuffer()
  return { ...pcmSamples(wav), kana: query.kana as string }
}

// The Kokoro voices kokoro-js has, for the editor's choice.
export const kokoroVoices = [
  'af_heart', 'af_alloy', 'af_aoede', 'af_bella', 'af_jessica', 'af_kore', 'af_nicole', 'af_nova', 'af_river',
  'af_sarah', 'af_sky', 'am_adam', 'am_echo', 'am_eric', 'am_fenrir', 'am_liam', 'am_michael', 'am_onyx', 'am_puck',
  'am_santa', 'bf_alice', 'bf_emma', 'bf_isabella', 'bf_lily', 'bm_daniel', 'bm_fable', 'bm_george', 'bm_lewis',
]

let kokoroModel: any
async function kokoro(pack: NonNullable<Pack['kokoro']>, line: LineObject) {
  if (!kokoroModel) {
    // The phonemizer kokoro-js uses needs DecompressionStream, which Bun lacks.
    if (!(globalThis as any).DecompressionStream) {
      const zlib = await import('node:zlib')
      const { Duplex } = await import('node:stream')
      ;(globalThis as any).DecompressionStream = class {
        readable: ReadableStream
        writable: WritableStream
        constructor(format: string) {
          const z =
            format === 'gzip' ? zlib.createGunzip() : format === 'deflate' ? zlib.createInflate() : zlib.createInflateRaw()
          const web = Duplex.toWeb(z) as unknown as { readable: ReadableStream; writable: WritableStream }
          this.readable = web.readable
          this.writable = web.writable
        }
      }
    }
    const { KokoroTTS } = await import('kokoro-js')
    kokoroModel = await KokoroTTS.from_pretrained('onnx-community/Kokoro-82M-v1.0-ONNX', { dtype: 'fp32', device: 'cpu' })
  }
  const audio = await kokoroModel.generate(line.text, { voice: pack.voice, speed: line.speed ?? pack.speed ?? 1 })
  const samples = Int16Array.from(audio.audio as Float32Array, (v) => Math.max(-32768, Math.min(32767, Math.round(v * 32767))))
  return { rate: audio.sampling_rate as number, samples, kana: '' }
}

// writeLine synthesizes a pack's line for a notification and writes its WAV.
export async function writeLine(id: string, pack: Pack, kind: string) {
  const line = lineObject(pack.lines[kind])
  if (!line.text) throw new Error(`${id}: no line for ${kind}`)
  const { rate, samples, kana } = pack.voicevox
    ? await voicevox(pack.voicevox, line)
    : pack.kokoro
      ? await kokoro(pack.kokoro, line)
      : (() => {
          throw new Error(`${id}: no voicevox or kokoro`)
        })()
  await Bun.write(lineFile(id, kind), adpcmWave(rate, samples))
  await stamp(id, kind, lineHash(pack, kind))
  return kana
}
