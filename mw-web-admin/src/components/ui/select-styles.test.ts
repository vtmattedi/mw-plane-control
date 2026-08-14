/// <reference types="node" />

import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const sourceRoot = resolve(process.cwd(), 'src')
const globals = readFileSync(resolve(sourceRoot, 'styles/globals.css'), 'utf8')
const selectStyles = readFileSync(
  resolve(sourceRoot, 'components/ui/select.module.scss'),
  'utf8',
)
const searchableSelectStyles = readFileSync(
  resolve(sourceRoot, 'components/ui/searchable-select.module.scss'),
  'utf8',
)

describe('contrato visual dos selects', () => {
  it('reduz centralmente o halo dos controles no tema escuro', () => {
    expect(globals).toContain('--control-focus-halo-strength: 28%;')
    expect(globals).toMatch(
      /:root\[data-theme='dark'\][\s\S]*--control-focus-halo-strength: 14%;/,
    )
  })

  it('mantém Select e SearchableSelect no mesmo token de foco', () => {
    const tokenUsage = 'var(--ring) var(--control-focus-halo-strength)'

    expect(selectStyles).toContain(tokenUsage)
    expect(selectStyles).toContain("&[data-state='open']")
    expect(searchableSelectStyles).toContain(tokenUsage)
  })
})
