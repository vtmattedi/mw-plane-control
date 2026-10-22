import { describe, expect, it } from 'vitest'
import {
  formatBytes,
  formatCount,
  formatDuration,
  formatRate,
} from './mqtt-format'

describe('mqtt formatters', () => {
  it('shows a dash for values the broker has not reported', () => {
    expect(formatCount(undefined, 'en-US')).toBe('—')
    expect(formatBytes(undefined, 'en-US')).toBe('—')
    expect(formatRate(undefined, 'en-US')).toBe('—')
    expect(formatDuration(undefined)).toBe('—')
  })

  it('formats bytes with binary units', () => {
    expect(formatBytes(0, 'en-US')).toBe('0 B')
    expect(formatBytes(1023, 'en-US')).toBe('1,023 B')
    expect(formatBytes(1536, 'en-US')).toBe('1.5 KB')
    expect(formatBytes(858_876, 'en-US')).toBe('839 KB')
    expect(formatBytes(5 * 1024 ** 3, 'en-US')).toBe('5 GB')
  })

  it('formats counts and rates per locale', () => {
    expect(formatCount(1234567, 'en-US')).toBe('1,234,567')
    expect(formatCount(1234567, 'pt-BR')).toBe('1.234.567')
    expect(formatRate(36.114, 'en-US')).toBe('36.1')
    expect(formatRate(36.114, 'pt-BR')).toBe('36,1')
  })

  it('formats uptime compactly', () => {
    expect(formatDuration(39)).toBe('39s')
    expect(formatDuration(125)).toBe('2m')
    expect(formatDuration(3 * 3600 + 20 * 60)).toBe('3h 20m')
    expect(formatDuration(2 * 86_400 + 5 * 3600)).toBe('2d 5h')
    expect(formatDuration(-1)).toBe('—')
  })
})
