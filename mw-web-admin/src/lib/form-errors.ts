import { ApiError } from '@/lib/api'

export interface FormErrorState {
  fields: Record<string, string[]>
  summary: string
}

export const emptyFormErrors: FormErrorState = {
  fields: {},
  summary: '',
}

const validationMessages: Record<string, string> = {
  'endsAt:validation.after': 'O término não pode ser anterior ao início.',
}

function localizedMessage(field: string, message: string) {
  return validationMessages[`${field}:${message}`] ?? message
}

export function formErrorsFrom(
  error: unknown,
  fallbackMessage: string,
): FormErrorState {
  if (!(error instanceof ApiError)) {
    return { fields: {}, summary: fallbackMessage }
  }

  return {
    fields: Object.fromEntries(
      Object.entries(error.errors).map(([field, messages]) => [
        field,
        messages.map((message) => localizedMessage(field, message)),
      ]),
    ),
    summary: error.message,
  }
}

export function firstFieldError(
  errors: FormErrorState,
  field: string,
): string | undefined {
  return errors.fields[field]?.[0]
}

const fieldLabels: Record<string, string> = {
  administrativeComment: 'Comentário administrativo',
  administrativeNotes: 'Observações administrativas',
  cpf: 'CPF',
  crp: 'CRP',
  currentPassword: 'Senha atual',
  email: 'E-mail',
  endsAt: 'Término',
  insuranceMemberId: 'Identificação do beneficiário',
  insuranceProviderId: 'Convênio',
  linkedPsychologistId: 'Psicólogo vinculado',
  linkedSublocatorId: 'Sublocador vinculado',
  name: 'Nome',
  newPassword: 'Nova senha',
  newPasswordConfirmation: 'Confirmação da nova senha',
  patientId: 'Paciente',
  paymentMethod: 'Forma de pagamento',
  permissions: 'Permissões',
  psychologistId: 'Psicólogo',
  roomId: 'Sala',
  startsAt: 'Início',
  temporaryPassword: 'Senha temporária',
  temporaryPasswordConfirmation: 'Confirmação da senha temporária',
}

export function apiErrorMessage(error: unknown, fallbackMessage: string) {
  if (!(error instanceof ApiError)) return fallbackMessage

  const fieldError = Object.entries(error.errors).find(
    ([, messages]) => messages.length > 0,
  )
  if (!fieldError) return error.message

  const [field, messages] = fieldError
  const localized = localizedMessage(field, messages[0])
  if (field === 'schedule') return localized
  if (localized !== messages[0]) return localized

  const label = fieldLabels[field] ?? field
  return `${error.message} ${label}: ${localized}`
}
