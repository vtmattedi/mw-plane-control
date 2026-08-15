import { describe, expect, it } from 'vitest'
import { splitFormValues, trimPastedValue } from './form-values'

describe('pasted form values', () => {
  it('removes zero-width and BOM characters from URI edges', () => {
    expect(
      splitFormValues(
        '\u200Bhttps://woi.mattediworks.com/callback\uFEFF, https://woi.mattediworks.com/',
      ),
    ).toEqual([
      'https://woi.mattediworks.com/callback',
      'https://woi.mattediworks.com/',
    ])
    expect(trimPastedValue('\u200Bhttps://example.com/lifecycle\uFEFF')).toBe(
      'https://example.com/lifecycle',
    )
  })

  it('does not alter characters inside an exact URI', () => {
    expect(trimPastedValue('https://example.com/a\u200Bb')).toBe(
      'https://example.com/a\u200Bb',
    )
  })
})
