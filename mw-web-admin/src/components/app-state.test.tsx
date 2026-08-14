import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import {
  EmptyState,
  ErrorState,
  LoadingState,
  UnauthorizedState,
} from './app-state'
import { BrandLogo } from './brand-logo'
import { MattediWorksBrand } from './mattediworks-brand'

describe('identidade e estados globais', () => {
  it('renderiza o logotipo MattediWorks com nome acessível', () => {
    render(
      <MemoryRouter>
        <BrandLogo linkTo="/" />
      </MemoryRouter>,
    )

    expect(screen.getByRole('img', { name: 'MattediWorks' })).toBeVisible()
  })

  it.each(['only', 'across', 'below', 'right'] as const)(
    'oferece a composição %s clara e escura da atribuição MattediWorks',
    (variant) => {
      const { container } = render(<MattediWorksBrand variant={variant} />)

      expect(screen.getByRole('img', { name: 'MattediWorks' })).toHaveAttribute(
        'data-variant',
        variant,
      )
      const [light, dark] = [...container.querySelectorAll('img')].map(
        (image) => image.getAttribute('src') ?? '',
      )
      expect(light).toContain('data:image/svg+xml')
      expect(dark).toContain('data:image/svg+xml')
      expect(dark).not.toBe(light)
      expect(dark).toContain("fill='%23fff'")
    },
  )

  it('mantém a composição à direita como padrão', () => {
    render(<MattediWorksBrand />)

    expect(screen.getByRole('img', { name: 'MattediWorks' })).toHaveAttribute(
      'data-variant',
      'right',
    )
  })

  it('oferece estados globais em português e sem depender apenas de cor', () => {
    const { rerender } = render(
      <LoadingState
        description="Buscando informações."
        title="Carregando cadastros"
      />,
    )
    expect(
      screen.getByRole('heading', { name: 'Carregando cadastros' }),
    ).toBeInTheDocument()

    rerender(<EmptyState description="A lista está vazia." />)
    expect(
      screen.getByRole('heading', { name: 'Nada por aqui' }),
    ).toBeInTheDocument()

    rerender(<ErrorState description="Tente novamente." />)
    expect(screen.getByRole('alert')).toBeInTheDocument()

    rerender(
      <UnauthorizedState
        action={<button type="button">Voltar</button>}
        description="Permissão necessária."
      />,
    )
    expect(
      screen.getByRole('heading', { name: 'Acesso negado' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Voltar' })).toBeInTheDocument()
  })
})
