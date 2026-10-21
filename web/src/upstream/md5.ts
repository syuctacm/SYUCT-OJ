// MD5 (RFC 1321) used to hash Gravatar e-mails in the browser. Vendored so the
// avatar helper can stay synchronous, like the upstream `avatar()` global.
// Verified against the RFC vectors ("" -> d41d8cd98f00b204e9800998ecf8427e,
// "abc" -> 900150983cd24fb0d6963f7d28e17f72, "The quick brown fox jumps over
// the lazy dog" -> 9e107d9d372bb6826bd81d3542a419d6) and md5sum for UTF-8 text.
const SHIFTS = [
  7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22,
  5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
  4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23,
  6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21,
]

const CONSTANTS = new Uint32Array(64)
for (let index = 0; index < 64; index++) {
  CONSTANTS[index] = Math.floor(Math.abs(Math.sin(index + 1)) * 0x100000000)
}

export function md5(input: string): string {
  const bytes = new TextEncoder().encode(input)
  const bitLength = bytes.length * 8
  const buffer = new Uint8Array((((bytes.length + 8) >> 6) + 1) * 64)
  buffer.set(bytes)
  buffer[bytes.length] = 0x80
  const view = new DataView(buffer.buffer)
  view.setUint32(buffer.length - 8, bitLength >>> 0, true)
  view.setUint32(buffer.length - 4, Math.floor(bitLength / 0x100000000), true)

  let a0 = 0x67452301
  let b0 = 0xefcdab89
  let c0 = 0x98badcfe
  let d0 = 0x10325476

  for (let offset = 0; offset < buffer.length; offset += 64) {
    const chunk = new Uint32Array(16)
    for (let index = 0; index < 16; index++) chunk[index] = view.getUint32(offset + index * 4, true)
    let a = a0
    let b = b0
    let c = c0
    let d = d0
    for (let index = 0; index < 64; index++) {
      let f: number
      let g: number
      if (index < 16) {
        f = (b & c) | (~b & d)
        g = index
      } else if (index < 32) {
        f = (d & b) | (~d & c)
        g = (5 * index + 1) % 16
      } else if (index < 48) {
        f = b ^ c ^ d
        g = (3 * index + 5) % 16
      } else {
        f = c ^ (b | ~d)
        g = (7 * index) % 16
      }
      f = (f + a + CONSTANTS[index] + chunk[g]) >>> 0
      a = d
      d = c
      c = b
      b = (b + ((f << SHIFTS[index]) | (f >>> (32 - SHIFTS[index])))) >>> 0
    }
    a0 = (a0 + a) >>> 0
    b0 = (b0 + b) >>> 0
    c0 = (c0 + c) >>> 0
    d0 = (d0 + d) >>> 0
  }

  const output = new Uint8Array(16)
  const outputView = new DataView(output.buffer)
  outputView.setUint32(0, a0, true)
  outputView.setUint32(4, b0, true)
  outputView.setUint32(8, c0, true)
  outputView.setUint32(12, d0, true)
  return [...output].map((byte) => byte.toString(16).padStart(2, '0')).join('')
}
