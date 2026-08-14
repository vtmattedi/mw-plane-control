import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { EntityCreationDialog } from './entity-creation-dialog'
import { AppConfirmProvider } from './confirm-provider'
import { useConfirm } from './confirm-context'

function Harness() {
  const confirm = useConfirm()
  const [result, setResult] = useState('pendente')

  return (
    <>
      <button
        type="button"
        onClick={() =>
          void confirm({
            title: 'Arquivar cadastro?',
            description: 'O histórico será preservado.',
            confirmLabel: 'Arquivar',
          }).then((confirmed) =>
            setResult(confirmed ? 'confirmado' : 'cancelado'),
          )
        }
      >
        Abrir confirmação
      </button>
      <output>{result}</output>
    </>
  )
}

function NestedDialogHarness() {
  const confirm = useConfirm()

  return (
    <EntityCreationDialog
      description="Dados editáveis."
      onOpenChange={() => undefined}
      open
      title="Editar usuário"
    >
      <button
        type="button"
        onClick={() =>
          void confirm({
            title: 'Salvar alterações?',
            description: 'Confirme os dados antes de continuar.',
            confirmLabel: 'Salvar',
          })
        }
      >
        Salvar
      </button>
    </EntityCreationDialog>
  )
}

describe('confirmação global', () => {
  it('só libera a ação destrutiva após confirmação explícita', async () => {
    const user = userEvent.setup()
    render(
      <AppConfirmProvider>
        <Harness />
      </AppConfirmProvider>,
    )

    await user.click(screen.getByRole('button', { name: 'Abrir confirmação' }))
    expect(
      screen.getByRole('alertdialog', { name: 'Arquivar cadastro?' }),
    ).toBeInTheDocument()
    expect(screen.getByText('pendente')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Arquivar' }))
    expect(await screen.findByText('confirmado')).toBeInTheDocument()
  })

  it('abre a confirmação acima de um diálogo de edição sem fechá-lo', async () => {
    const user = userEvent.setup()
    render(
      <AppConfirmProvider>
        <NestedDialogHarness />
      </AppConfirmProvider>,
    )

    const editDialog = screen.getByRole('dialog', { name: 'Editar usuário' })
    await user.click(screen.getByRole('button', { name: 'Salvar' }))

    expect(editDialog).toBeInTheDocument()
    expect(
      screen.getByRole('alertdialog', { name: 'Salvar alterações?' }),
    ).toHaveClass('confirm-dialog')
  })
})
