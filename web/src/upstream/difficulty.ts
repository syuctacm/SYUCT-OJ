// Difficulty algorithm derived from Hydro `packages/hydrooj/src/lib/difficulty.ts`
// (AGPL-3.0). Kept byte-for-byte equivalent so templates compute the same value.
const CACHE_INFO = {
  s: 0.0,
  y: 0,
  values: [0.0],
}

function LOGP(x: number) {
  const sqrtPi = 2.506628274631
  return (2 * Math.exp(-2.0 * (Math.log(x) ** 2))) / x / sqrtPi
}

function integrateEnsureCache(y: number) {
  const lastY = CACHE_INFO.y
  if (y <= lastY) return CACHE_INFO
  let s = CACHE_INFO.s
  const dx = 0.1
  const dT = 2
  let x0 = (lastY / dT) * dx
  let currentY = lastY
  while (y > currentY) {
    x0 += dx
    s += LOGP(x0) * dx
    for (let i = 1; i <= dT; i++) CACHE_INFO.values.push(s)
    currentY += dT
  }
  CACHE_INFO.y = currentY
  CACHE_INFO.s = s
  return CACHE_INFO
}

function integrate(y: number) {
  integrateEnsureCache(y)
  return CACHE_INFO.values[y]
}

export function difficulty(nSubmit: number, nAccept: number): number | null {
  if (!nSubmit) return null
  const s = integrate(nSubmit)
  const acRate = nAccept / nSubmit
  const ans = Math.round(10 - 13 * s * acRate)
  return Math.max(ans, 1)
}
