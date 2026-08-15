import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { OneTimeCredentialDialog } from './one-time-credential-dialog'

describe('credencial exibida uma única vez', () => {
  it('mostra a credencial em um diálogo e exige conclusão explícita', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(
      <OneTimeCredentialDialog
        credential="mw_client_secret-value"
        onClose={onClose}
      />,
    )

    expect(
      screen.getByRole('dialog', { name: 'Nova credencial' }),
    ).toBeVisible()
    expect(screen.getByLabelText('Credencial gerada')).toHaveValue(
      'mw_client_secret-value',
    )
    expect(screen.getByText(/não será exibida novamente/i)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Concluir' }))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('mostra e copia separadamente as credenciais de um projeto OAuth', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    render(
      <OneTimeCredentialDialog
        credentials={[
          { label: 'MW_OAUTH_PROJECT_KEY', value: 'project-key' },
          { label: 'MW_OAUTH_CALLBACK_SECRET', value: 'callback-secret' },
        ]}
        onClose={() => undefined}
        title="Credenciais do projeto"
      />,
    )

    expect(
      screen.getByRole('dialog', { name: 'Credenciais do projeto' }),
    ).toBeVisible()
    expect(screen.getByLabelText('MW_OAUTH_PROJECT_KEY')).toHaveValue(
      'project-key',
    )
    expect(screen.getByLabelText('MW_OAUTH_CALLBACK_SECRET')).toHaveValue(
      'callback-secret',
    )

    await user.click(
      screen.getByRole('button', {
        name: 'Copiar MW_OAUTH_CALLBACK_SECRET',
      }),
    )
    expect(writeText).toHaveBeenCalledWith('callback-secret')
  })
})
