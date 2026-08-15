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
})
