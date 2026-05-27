import type { LanguageSupport } from '@codemirror/language'

/**
 * Resolve a CodeMirror LanguageSupport for a given filename.
 * Uses @codemirror/language-data's `languages` array (LanguageDescription[]).
 * Returns null if no match or load fails.
 */
export async function resolveLanguage(
  filename: string,
): Promise<LanguageSupport | null> {
  const { languages } = await import('@codemirror/language-data')

  const match = languages.find((desc) => {
    if (desc.filename && desc.filename.test(filename)) return true
    const ext = filename.split('.').pop()
    if (!ext) return false
    return desc.extensions?.includes(ext) ?? false
  })

  if (!match) return null

  try {
    return await match.load()
  } catch {
    return null
  }
}
