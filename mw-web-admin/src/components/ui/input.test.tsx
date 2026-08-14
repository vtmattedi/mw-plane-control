import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { Clock3 } from 'lucide-react'
import { MemoryRouter } from 'react-router-dom'
import { Input } from './input'

describe('Input', () => {
  it('forwards text changes as values instead of DOM events', () => {
    const onChange = vi.fn()
    render(
      <Input
        aria-label="Nome"
        placeholder="Nome completo"
        value="Ana"
        onChange={onChange}
      />,
    )

    fireEvent.change(screen.getByLabelText('Nome'), {
      target: { value: 'Ana Maria' },
    })

    expect(onChange).toHaveBeenCalledWith('Ana Maria')
    expect(screen.getByPlaceholderText('Nome completo')).toHaveValue('Ana')
  })

  it('supports numeric attributes and a custom indicator', () => {
    const onChange = vi.fn()
    const { container } = render(
      <Input
        aria-label="Duração"
        indicator={<Clock3 data-testid="clock" />}
        indicatorPosition="left"
        indicatorSeparator
        max={240}
        min={15}
        type="number"
        value={45}
        onChange={onChange}
      />,
    )

    const input = screen.getByLabelText('Duração')
    const root = container.firstElementChild
    const indicator = screen.getByTestId('clock').parentElement

    expect(input).toHaveAttribute('type', 'number')
    expect(input).toHaveAttribute('min', '15')
    expect(root).toHaveAttribute('data-indicator-position', 'left')
    expect(indicator).toHaveAttribute('data-separator', 'true')
  })

  it('provides the search variant with its icon', () => {
    const { container } = render(
      <Input
        aria-label="Buscar"
        placeholder="Buscar paciente"
        value=""
        variant="search"
        onChange={vi.fn()}
      />,
    )

    expect(
      screen.getByRole('searchbox', { name: 'Buscar' }),
    ).toBeInTheDocument()
    expect(container.querySelector('svg')).toBeInTheDocument()
  })

  it('formats money in pt-BR with R$ on the left and emits a canonical decimal value', () => {
    const onChange = vi.fn()
    const { container } = render(
      <Input
        aria-label="Preço"
        type="money"
        value="1234.50"
        onChange={onChange}
      />,
    )

    const input = screen.getByLabelText('Preço')
    expect(input).toHaveValue('1.234,50')
    expect(input.parentElement).toHaveTextContent('R$')
    expect(container.firstElementChild).toHaveAttribute(
      'data-indicator-position',
      'left',
    )

    fireEvent.change(input, { target: { value: '9.876,54' } })
    expect(onChange).toHaveBeenCalledWith('9876.54')
  })

  it('formats CPF and limits it to eleven digits', () => {
    const onChange = vi.fn()
    render(
      <Input
        aria-label="CPF"
        type="cpf"
        value="52998224725"
        onChange={onChange}
      />,
    )

    const input = screen.getByLabelText('CPF')
    expect(input).toHaveValue('529.982.247-25')
    expect(input).toHaveAttribute('maxlength', '14')

    fireEvent.change(input, { target: { value: '1234567890199' } })
    expect(onChange).toHaveBeenCalledWith('123.456.789-01')
  })

  it('formats CRP and limits the professional registration', () => {
    const onChange = vi.fn()
    render(
      <Input
        aria-label="CRP"
        type="crp"
        value="CRP 06-12345"
        onChange={onChange}
      />,
    )

    const input = screen.getByLabelText('CRP')
    expect(input).toHaveValue('06/12345')
    expect(input).toHaveAttribute('maxlength', '9')

    fireEvent.change(input, { target: { value: '06123456789' } })
    expect(onChange).toHaveBeenCalledWith('06/123456')

    fireEvent.change(input, { target: { value: '6/' } })
    expect(onChange).toHaveBeenCalledWith('6/')
  })

  it('adds the LGPD indicator, details link and hover tooltip', async () => {
    render(
      <MemoryRouter>
        <Input aria-label="Documento" lgpd value="" onChange={vi.fn()} />
      </MemoryRouter>,
    )

    const link = screen.getByRole('link', {
      name: 'Dados protegidos pela LGPD. Ver detalhes',
    })
    const tooltip = screen.getByRole('tooltip')

    expect(link).toHaveAttribute('href', '/integracoes?tab=lgpd')
    expect(tooltip).toHaveTextContent(
      /Estes dados são protegidos conforme a LGPD/,
    )

    fireEvent.pointerEnter(link)
    await waitFor(() => expect(tooltip).toHaveAttribute('data-open'))
    fireEvent.pointerLeave(link)
    expect(tooltip).not.toHaveAttribute('data-open')
  })
})
