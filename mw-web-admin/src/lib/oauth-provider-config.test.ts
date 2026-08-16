import {
  googleConfigToForm,
  googleFormError,
  googleFormToConfig,
  type GoogleOAuthConfig,
} from './oauth-provider-config'

const config: GoogleOAuthConfig = {
  enabled: true,
  allowed_scopes: ['scope:a', 'scope:b'],
  allowed_callback_urls: ['https://app.example.com/api/oauth/callback'],
  allowed_return_urls: ['https://app.example.com/settings'],
  limits: { per_minute: 10, per_hour: 100, max_pending: 50 },
}

describe('configuração de provedor OAuth', () => {
  it('converte o contrato da API para formulário e de volta', () => {
    const form = googleConfigToForm(config)
    form.allowedScopes += '\nscope:a'

    expect(googleFormToConfig(form)).toEqual(config)
  })

  it('exige políticas completas ao ativar o provedor', () => {
    const form = googleConfigToForm(config)
    form.allowedCallbackURLs = ''

    expect(googleFormError(form)).toMatch(/callback/i)
  })

  it('rejeita URLs não exatas ou HTTP remoto', () => {
    const form = googleConfigToForm(config)
    form.allowedReturnURLs = 'http://app.example.com/settings?from=oauth'

    expect(googleFormError(form)).toMatch(/HTTPS/i)
  })
})
