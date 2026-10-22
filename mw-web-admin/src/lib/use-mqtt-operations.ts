import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect } from 'react'
import { toast } from 'sonner'
import { beginStepUp, ControlApiError } from '@/lib/control-api'
import {
  runMqttOperation,
  validateOperation,
  type MqttCredential,
  type MqttOperation,
} from '@/lib/mqtt-api'
import { usePreferences } from '@/preferences/preferences-context'

type PendingOperation = { operation: MqttOperation; returnTo: string }

export const pendingOperationKey = 'mw-mqtt-pending-operation'

/** Reads a saved operation back, accepting only one that validates. */
function parsePending(raw: string): PendingOperation | null {
  try {
    const value = JSON.parse(raw) as { operation?: unknown; returnTo?: unknown }
    const operation = validateOperation(value.operation)
    if (!operation || typeof value.returnTo !== 'string') return null
    return { operation, returnTo: value.returnTo }
  } catch {
    return null
  }
}

/**
 * Runs MQTT console operations (clients, roles, rules) with the step-up round trip handled
 * once, for every page.
 *
 * When the service answers `step_up_required`, the operation is saved in session storage
 * (it never contains a secret), the browser goes through the identity provider's step-up,
 * and the operation runs once when it returns to the same page.
 */
export function useMqttOperations(
  options: {
    /** A generated password, to show once. */
    onCredential?: (credential: MqttCredential) => void
    /** Called after any operation succeeds. */
    onDone?: () => void
  } = {},
) {
  const { tr } = usePreferences()
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: (pending: PendingOperation) =>
      runMqttOperation(pending.operation),
    onSuccess: (credential) => {
      sessionStorage.removeItem(pendingOperationKey)
      if (credential) options.onCredential?.(credential)
      toast.success(tr('Operação concluída com sucesso.'))
      options.onDone?.()
      void queryClient.invalidateQueries({ queryKey: ['mqtt'] })
    },
    onError: (error, pending) => {
      if (
        error instanceof ControlApiError &&
        error.code === 'step_up_required'
      ) {
        sessionStorage.setItem(pendingOperationKey, JSON.stringify(pending))
        beginStepUp(pending.returnTo)
        return
      }
      toast.error(
        error instanceof Error
          ? tr(error.message)
          : tr('Não foi possível concluir a operação.'),
      )
    },
  })

  const { mutate } = mutation

  // Resume the operation that triggered a step-up, once.
  useEffect(() => {
    const raw = sessionStorage.getItem(pendingOperationKey)
    if (!raw) return
    const pending = parsePending(raw)
    // Claim before executing so StrictMode cannot replay a credential rotation.
    sessionStorage.removeItem(pendingOperationKey)
    if (pending && pending.returnTo === window.location.pathname) {
      mutate(pending)
    }
  }, [mutate])

  const run = useCallback(
    (operation: MqttOperation) =>
      mutate({ operation, returnTo: window.location.pathname }),
    [mutate],
  )

  return { run, pending: mutation.isPending }
}
