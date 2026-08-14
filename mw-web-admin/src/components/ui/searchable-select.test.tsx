import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'
import { SearchableSelect } from './searchable-select'

const options = [
  { label: 'Ana Ávila', value: 'ana' },
  { label: 'Bruno Lima', value: 'bruno' },
  { label: 'Carla Souza', value: 'carla' },
]

function MultipleHarness() {
  const [value, setValue] = useState<string[]>([])
  return (
    <SearchableSelect
      multiple
      aria-label="Profissionais"
      options={options}
      value={value}
      onValueChange={setValue}
    />
  )
}

test('filtra sem depender de acentos e seleciona um único valor', async () => {
  const user = userEvent.setup()
  const onValueChange = vi.fn()
  render(
    <SearchableSelect
      aria-label="Paciente"
      options={options}
      value=""
      onValueChange={onValueChange}
    />,
  )

  await user.click(screen.getByRole('combobox', { name: 'Paciente' }))
  await user.type(
    screen.getByRole('textbox', { name: 'Pesquisar em Paciente' }),
    'avila',
  )
  await user.click(screen.getByRole('option', { name: 'Ana Ávila' }))

  expect(onValueChange).toHaveBeenCalledWith('ana')
})

test('a variante múltipla alterna mais de um valor', async () => {
  const user = userEvent.setup()
  render(<MultipleHarness />)

  await user.click(screen.getByRole('combobox', { name: 'Profissionais' }))
  await user.click(screen.getByRole('option', { name: 'Ana Ávila' }))
  await user.click(screen.getByRole('option', { name: 'Bruno Lima' }))

  expect(
    screen.getByRole('combobox', { name: 'Profissionais' }),
  ).toHaveTextContent('Ana Ávila, Bruno Lima')
})

test('mantém o hook visual e o estado desabilitado no gatilho fechado', () => {
  render(
    <SearchableSelect
      disabled
      aria-label="Paciente"
      options={options}
      value=""
      onValueChange={vi.fn()}
    />,
  )

  const trigger = screen.getByRole('combobox', { name: 'Paciente' })
  expect(trigger).toBeDisabled()
  expect(trigger.className).not.toBe('')
})
