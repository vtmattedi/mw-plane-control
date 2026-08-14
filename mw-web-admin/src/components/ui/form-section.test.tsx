import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'
import { FormSection } from './form-section'

test('combina classes e oferece toggle opcional', async () => {
  const onChange = vi.fn()
  render(
    <FormSection
      className="custom-section"
      title="Agenda"
      toggle={{ state: true, onChange, label: 'Ativar agenda' }}
    >
      <label>
        Campo
        <input />
      </label>
    </FormSection>,
  )

  const section = screen.getByRole('group', { name: 'Agenda' })
  expect(section).toHaveClass('custom-section')
  await userEvent.click(screen.getByRole('switch', { name: 'Ativar agenda' }))
  expect(onChange).toHaveBeenCalledWith(false)
})
