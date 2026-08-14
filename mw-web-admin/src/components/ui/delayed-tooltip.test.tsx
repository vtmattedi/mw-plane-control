import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { DelayedTooltip } from './delayed-tooltip'

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

test('renderiza a camada no body e a exibe após o atraso configurado', () => {
  render(
    <button type="button">
      Pagamento
      <DelayedTooltip>Pagamento: Pago</DelayedTooltip>
    </button>,
  )

  const trigger = screen.getByRole('button', { name: 'Pagamento' })
  const tooltip = screen.getByRole('tooltip')
  vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue({
    bottom: 120,
    height: 20,
    left: 80,
    right: 100,
    top: 100,
    width: 20,
    x: 80,
    y: 100,
    toJSON: () => undefined,
  })
  vi.spyOn(tooltip, 'getBoundingClientRect').mockReturnValue({
    bottom: 0,
    height: 24,
    left: 0,
    right: 120,
    top: 0,
    width: 120,
    x: 0,
    y: 0,
    toJSON: () => undefined,
  })

  expect(tooltip.parentElement).toBe(document.body)
  expect(trigger).toHaveAttribute('aria-describedby', tooltip.id)
  expect(tooltip).not.toHaveAttribute('data-open')

  fireEvent.pointerEnter(trigger)
  act(() => vi.advanceTimersByTime(649))
  expect(tooltip).not.toHaveAttribute('data-open')

  act(() => vi.advanceTimersByTime(1))
  expect(tooltip).toHaveAttribute('data-open')
  expect(tooltip).toHaveAttribute('data-placement', 'top')
})

test('funciona por foco de teclado e fecha com Escape', () => {
  render(
    <button type="button">
      Presença
      <DelayedTooltip delayMs={10}>
        Sessão encerrada sem presença informada
      </DelayedTooltip>
    </button>,
  )

  const trigger = screen.getByRole('button', { name: 'Presença' })
  const tooltip = screen.getByRole('tooltip')

  fireEvent.focusIn(trigger)
  act(() => vi.advanceTimersByTime(10))
  expect(tooltip).toHaveAttribute('data-open')

  fireEvent.keyDown(trigger, { key: 'Escape' })
  expect(tooltip).not.toHaveAttribute('data-open')
})
