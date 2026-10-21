// The judge language registry lives on the server (`GET /api/v1/languages`).
// Pages keep the built-in list as a fallback for the first paint.
import { getLanguages } from '../api.ts'
import { languageOptions } from '../components/autocomplete/LanguageSelectAutoComplete.tsx'

export type LanguageItem = { _id: string; name: string }

let cache: LanguageItem[] | null = null

export async function loadLanguages(): Promise<LanguageItem[]> {
  if (cache) return cache
  try {
    const items = await getLanguages()
    if (items.length) {
      cache = items.map((item) => ({ _id: item.id, name: item.display }))
    }
  } catch {
    // keep the built-in list
  }
  return cache ?? languageOptions
}
