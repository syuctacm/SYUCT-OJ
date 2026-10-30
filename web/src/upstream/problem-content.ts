// Hydro stores multilingual problem statements as a JSON object whose keys are
// locale identifiers. Plain Markdown and pre-rendered HTML remain strings.
type LocalizedContent = Record<string, unknown>

function parseLocalized(value: unknown): LocalizedContent | null {
  if (typeof value !== 'string') return null
  const trimmed = value.trim()
  if (!trimmed.startsWith('{')) return null
  try {
    const parsed: unknown = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null
    return parsed as LocalizedContent
  } catch {
    return null
  }
}

export function contentLanguages(value: unknown): string[] {
  return Object.keys(parseLocalized(value) ?? {})
}

export function selectLocalizedContent(value: unknown, language = '', fallback = 'zh'): string {
  const localized = parseLocalized(value)
  if (!localized) return String(value ?? '')
  const keys = Object.keys(localized)
  if (!keys.length) return ''
  const preferred = [language, fallback].filter(Boolean)
  let selected = preferred.map((key) => keys.find((candidate) => candidate === key)).find(Boolean)
  if (!selected && language) selected = keys.find((key) => key.startsWith(language))
  if (!selected) selected = keys[0]
  return String(localized[selected] ?? '')
}
