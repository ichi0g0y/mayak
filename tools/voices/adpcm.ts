// IMA ADPCM (WAV format 0x11, mono): a quarter of 16-bit PCM's size, which
// the built-in voices are kept in; internal/sound/voice.go decodes it.
const steps = [
  7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 19, 21, 23, 25, 28, 31, 34, 37, 41, 45, 50, 55, 60, 66, 73, 80, 88, 97, 107,
  118, 130, 143, 157, 173, 190, 209, 230, 253, 279, 307, 337, 371, 408, 449, 494, 544, 598, 658, 724, 796, 876, 963,
  1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358, 5894,
  6484, 7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899, 15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794,
  32767,
]
const indexShift = [-1, -1, -1, -1, 2, 4, 6, 8]
const blockAlign = 512
const samplesPerBlock = (blockAlign - 4) * 2 + 1

// pcmSamples reads a mono 16-bit PCM WAV.
export function pcmSamples(wav: ArrayBuffer): { rate: number; samples: Int16Array } {
  const view = new DataView(wav)
  let rate = 0
  for (let at = 12; at + 8 <= view.byteLength; ) {
    const id = String.fromCharCode(...new Uint8Array(wav, at, 4))
    const size = view.getUint32(at + 4, true)
    if (id === 'fmt ') {
      if (view.getUint16(at + 8, true) !== 1 || view.getUint16(at + 10, true) !== 1 || view.getUint16(at + 22, true) !== 16)
        throw new Error('not mono 16-bit PCM')
      rate = view.getUint32(at + 12, true)
    } else if (id === 'data') {
      return { rate, samples: new Int16Array(wav.slice(at + 8, at + 8 + size)) }
    }
    at += 8 + size + (size % 2)
  }
  throw new Error('no data')
}

export function adpcmWave(rate: number, samples: Int16Array): Uint8Array {
  const blocks = Math.max(1, Math.ceil(samples.length / samplesPerBlock))
  const data = new Uint8Array(blocks * blockAlign)
  const at = (i: number) => samples[Math.min(i, samples.length - 1)] ?? 0
  for (let b = 0; b < blocks; b++) {
    const base = b * samplesPerBlock
    const out = b * blockAlign
    let predictor = at(base)
    let index = 0
    // Start each block at a step that suits its first difference.
    const first = Math.abs(at(base + 1) - predictor)
    while (index < 88 && steps[index] < first) index++
    data[out] = predictor & 0xff
    data[out + 1] = (predictor >> 8) & 0xff
    data[out + 2] = index
    for (let i = 1; i < samplesPerBlock; i++) {
      const step = steps[index]
      let diff = at(base + i) - predictor
      let code = 0
      if (diff < 0) {
        code = 8
        diff = -diff
      }
      if (diff >= step) {
        code |= 4
        diff -= step
      }
      if (diff >= step >> 1) {
        code |= 2
        diff -= step >> 1
      }
      if (diff >= step >> 2) code |= 1
      let delta = step >> 3
      if (code & 4) delta += step
      if (code & 2) delta += step >> 1
      if (code & 1) delta += step >> 2
      predictor = Math.max(-32768, Math.min(32767, code & 8 ? predictor - delta : predictor + delta))
      index = Math.max(0, Math.min(88, index + indexShift[code & 7]))
      const pos = out + 4 + ((i - 1) >> 1)
      data[pos] |= (i - 1) % 2 === 0 ? code : code << 4
    }
  }
  const header = new DataView(new ArrayBuffer(60))
  const text = (at: number, s: string) => [...s].forEach((c, i) => header.setUint8(at + i, c.charCodeAt(0)))
  text(0, 'RIFF')
  header.setUint32(4, 52 + data.length, true)
  text(8, 'WAVE')
  text(12, 'fmt ')
  header.setUint32(16, 20, true)
  header.setUint16(20, 0x11, true)
  header.setUint16(22, 1, true)
  header.setUint32(24, rate, true)
  header.setUint32(28, Math.floor((rate * blockAlign) / samplesPerBlock), true)
  header.setUint16(32, blockAlign, true)
  header.setUint16(34, 4, true)
  header.setUint16(36, 2, true)
  header.setUint16(38, samplesPerBlock, true)
  text(40, 'fact')
  header.setUint32(44, 4, true)
  header.setUint32(48, samples.length, true)
  text(52, 'data')
  header.setUint32(56, data.length, true)
  const wave = new Uint8Array(60 + data.length)
  wave.set(new Uint8Array(header.buffer))
  wave.set(data, 60)
  return wave
}
