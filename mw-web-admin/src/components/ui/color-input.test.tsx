import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test } from 'vitest'
import { ColorInput } from './color-input'

function ColorInputHarness() {
  const [color, setColor] = useState('#4B7891')

  return <ColorInput label="Cor" value={color} onChange={setColor} />
}

test('oferece campo hexadecimal e seletor visual em popover', async () => {
  const user = userEvent.setup()
  render(<ColorInputHarness />)

  const field = screen.getByRole('textbox', {
    name: 'Cor: valor hexadecimal',
  })
  expect(field).toHaveAttribute('placeholder', 'Cor em hexadecimal')

  await user.click(screen.getByRole('button', { name: 'Escolher cor' }))
  expect(screen.getByRole('slider', { name: 'Color' })).toBeInTheDocument()
  expect(screen.getByRole('slider', { name: 'Hue' })).toBeInTheDocument()

  await user.clear(field)
  await user.type(field, '12abef')
  expect(field).toHaveValue('#12ABEF')
})
