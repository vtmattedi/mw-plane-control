import { expect, test } from 'vitest'
import { ApiError } from './api'
import { apiErrorMessage, formErrorsFrom } from './form-errors'

test('traduz a validação de término posterior ao início', () => {
  const error = new ApiError(
    'Verifique os campos informados.',
    422,
    'VALIDATION_FAILED',
    { endsAt: ['validation.after'] },
  )

  expect(apiErrorMessage(error, 'Falha ao salvar.')).toBe(
    'O término não pode ser anterior ao início.',
  )
  expect(formErrorsFrom(error, 'Falha ao salvar.').fields.endsAt).toEqual([
    'O término não pode ser anterior ao início.',
  ])
})

test('exibe conflitos de agenda sem prefixos técnicos', () => {
  const error = new ApiError(
    'Verifique os campos informados.',
    422,
    'VALIDATION_FAILED',
    {
      schedule: ['O psicólogo já possui um agendamento neste horário.'],
    },
  )

  expect(apiErrorMessage(error, 'Falha ao salvar.')).toBe(
    'O psicólogo já possui um agendamento neste horário.',
  )
})
