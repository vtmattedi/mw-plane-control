import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, expect, test, vi } from 'vitest'
import { DateSelector } from './date-selector'
import { TimeSelector } from './time-selector'

beforeAll(() => {
  Object.defineProperties(Element.prototype, {
    hasPointerCapture: {
      configurable: true,
      value: () => false,
    },
    releasePointerCapture: {
      configurable: true,
      value: () => undefined,
    },
    setPointerCapture: {
      configurable: true,
      value: () => undefined,
    },
    scrollIntoView: {
      configurable: true,
      value: () => undefined,
    },
  })
})

test('seleciona uma data sem usar o seletor nativo do navegador', async () => {
  const change = vi.fn()
  const user = userEvent.setup()
  render(
    <div data-testid="modal-scroll-container">
      <DateSelector value="2026-08-14" onChange={change} />
    </div>,
  )

  await user.click(screen.getByRole('button', { name: '14 de agosto de 2026' }))
  expect(
    screen
      .getByTestId('modal-scroll-container')
      .contains(screen.getByRole('dialog', { name: 'Escolher data' })),
  ).toBe(false)
  await user.click(
    screen.getByRole('button', {
      name: new Date(2026, 7, 15).toLocaleDateString('pt-BR', {
        dateStyle: 'full',
      }),
    }),
  )

  expect(change).toHaveBeenCalledWith('2026-08-15')
})

test('oferece uma apresentação compacta DD/Mon mantendo o nome acessível', () => {
  render(<DateSelector compact value="2026-08-14" onChange={() => undefined} />)

  expect(
    screen.getByRole('button', { name: '14 de agosto de 2026' }),
  ).toHaveTextContent('14/Ago')
})

test('seleciona somente hora e minuto', async () => {
  const change = vi.fn()
  const user = userEvent.setup()
  render(
    <div data-testid="modal-scroll-container">
      <TimeSelector value="09:30" onChange={change} />
    </div>,
  )

  await user.click(screen.getByRole('button', { name: '09:30' }))
  expect(
    screen
      .getByTestId('modal-scroll-container')
      .contains(screen.getByRole('dialog', { name: 'Escolher horário' })),
  ).toBe(false)
  await user.click(screen.getByRole('combobox', { name: 'Hora' }))
  await user.click(await screen.findByRole('option', { name: '10' }))
  await user.click(screen.getByRole('combobox', { name: 'Minuto' }))
  await user.click(await screen.findByRole('option', { name: '45' }))

  expect(change).toHaveBeenLastCalledWith('10:45')
})
