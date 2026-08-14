import { fireEvent, render, screen } from '@testing-library/react'
import { SplitInput } from './split-input'

describe('SplitInput', () => {
  it('mantém a distribuição compacta e sincroniza slider e percentual', () => {
    const onChange = vi.fn()
    render(<SplitInput value={0.7} onChange={onChange} />)

    expect(screen.getByText('70% clínica · 30% profissional')).toBeVisible()
    expect(
      screen.getByRole('slider', {
        name: 'Percentual da clínica: controle deslizante',
      }),
    ).toHaveValue('70')

    fireEvent.change(screen.getByRole('spinbutton'), {
      target: { value: '65' },
    })
    expect(onChange).toHaveBeenCalledWith(0.65)
  })
})
