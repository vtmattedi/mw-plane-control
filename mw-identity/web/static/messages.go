package static

import "fmt"

// Portal languages. Lang values double as the <html lang> attribute.
const (
	LangPT = "pt-BR"
	LangEN = "en"
)

// Portuguese copy is written for Brazilian readers, not translated word for
// word: terms a Brazilian tech user says in English (login, passkey, app,
// token, scope, audit log) stay in English.
var catalogs = map[string]map[string]string{
	LangPT: {
		"lang.switch_code":  "EN",
		"lang.switch_label": "Ver em inglês",
		"common.theme":      "Alternar tema",
		"common.back":       "Voltar",
		"common.privacy":    "Privacidade",
		"common.security":   "Segurança",
		"common.account":    "Conta",
		"common.copyright":  "© 2026 MattediWorks",

		"account.meta_title":         "Sua conta",
		"account.your_account":       "Sua conta",
		"account.strong_title":       "Falta um passo",
		"account.strong_body":        "O app que você estava usando pede login forte: passkey ou app autenticador. Configure um dos dois aqui embaixo e depois",
		"account.strong_link":        "volte para o app",
		"account.badge_superadmin":   "Superadmin",
		"account.badge_active":       "Conta ativa",
		"account.tabs_label":         "Configurações da conta",
		"account.passkeys_title":     "Passkeys",
		"account.passkeys_lead":      "Entre com biometria, Windows Hello ou uma chave de segurança. Sem senha.",
		"account.passkey_name":       "Nome da passkey",
		"account.passkey_add":        "Adicionar passkey",
		"account.totp_title":         "App autenticador",
		"account.totp_lead":          "Um código de 6 dígitos para quando a passkey não estiver à mão.",
		"account.totp_setup":         "Configurar app autenticador",
		"account.totp_qr_alt":        "QR code para configurar o app autenticador",
		"account.totp_scan":          "Escaneie o QR code",
		"account.totp_apps":          "Funciona com Google Authenticator, Microsoft Authenticator, 1Password ou qualquer app compatível com TOTP.",
		"account.totp_manual":        "Ou digite esta chave na mão:",
		"account.totp_code":          "Código do app",
		"account.totp_confirm":       "Confirmar código",
		"account.totp_disable":       "Desativar app autenticador",
		"account.session_title":      "Sessão atual",
		"account.session_lead":       "Como você entrou no MW Identity desta vez.",
		"account.email":              "Email",
		"account.email_verification": "Verificação do email",
		"account.verified":           "Verificado",
		"account.not_verified":       "Não verificado",
		"account.username":           "Nome de usuário",
		"account.signed_in_with":     "Login com",
		"account.session_active":     "Sessão ativa",
		"account.permission":         "Permissão",
		"account.user":               "Usuário",
		"account.reconfirm":          "Confirmar que é você de novo",
		"account.apps_title":         "Apps conectados",
		"account.apps_lead":          "Apps que usam sua conta MattediWorks.",
		"account.notice_removed":     "App removido. Ele confirmou que vai apagar seus dados.",
		"account.notice_no_callback": "App removido e acesso revogado. Ele não recebe pedidos automáticos de exclusão: fale com o app para apagar seus dados.",
		"account.notice_pending":     "O acesso foi revogado, mas o app ainda não confirmou a exclusão. Tente remover de novo daqui a pouco.",
		"account.remove":             "Remover",
		"account.remove_body":        "O acesso acaba na hora e %s recebe um pedido para apagar seus dados. Se o app permitir, dá para entrar de novo depois.",
		"account.remove_confirm":     "Remover %s da minha conta",
		"account.remove_button":      "Remover app",
		"account.apps_empty":         "Nenhum app conectado ainda.",
		"value.password":             "Senha",
		"value.webauthn":             "Passkey",
		"value.totp":                 "App autenticador",
		"value.user":                 "Usuário",
		"value.admin":                "Admin",
		"value.superadmin":           "Superadmin",
		"account.logout_title":       "Sair",
		"account.logout_lead":        "Encerra sua sessão do MW Identity neste navegador.",
		"account.logout_button":      "Sair",
		"account.delete_title":       "Excluir conta",
		"account.delete_lead":        "Revoga todos os acessos e pede para cada app conectado apagar sua conta. Não dá para desfazer.",
		"account.delete_label":       "Digite DELETE para confirmar",
		"account.delete_button":      "Excluir minha conta",

		"info.tabs_label": "Sobre o MW Identity",
		"info.note":       "Isto é um resumo de como o MW Identity funciona, não um documento jurídico.",

		"privacy.title":    "Privacidade",
		"privacy.lead":     "Como o MW Identity usa e protege os dados de que precisa para manter sua conta.",
		"privacy.s1.title": "Dados da conta",
		"privacy.s1.body":  "Guardamos seu ID estável, nome de usuário, nome, email, se o email foi verificado e como você faz login. Cada app só recebe o que os scopes autorizados liberam.",
		"privacy.s2.title": "Apps conectados",
		"privacy.s2.body":  "A página da conta mostra cada app conectado. Cada app guarda o próprio perfil e os próprios dados. Ao remover um app, o acesso dele acaba na hora e ele recebe um pedido para apagar seus dados; o resto da conta segue igual.",
		"privacy.s3.title": "Sessões e audit log",
		"privacy.s3.body":  "Sessões, eventos de segurança e ações administrativas ficam no audit log, para proteger a conta, investigar problemas e operar o serviço.",
		"privacy.s4.title": "Credenciais",
		"privacy.s4.body":  "Senhas e tokens nunca ficam em texto puro. Cada secret é guardado com hash ou criptografia, conforme o uso.",
		"privacy.s5.title": "Exclusão da conta",
		"privacy.s5.body":  "Ao excluir a conta, o MW Identity revoga todos os acessos na hora e avisa cada app conectado. Cada app apaga ou anonimiza seus dados conforme as próprias regras de retenção. Se algum app não confirmar, a conta fica desativada e a exclusão pendente até um admin tentar de novo.",

		"security.title":    "Segurança",
		"security.lead":     "O que protege sua conta e o ecossistema MattediWorks.",
		"security.s1.title": "Passkeys",
		"security.s1.body":  "A passkey é o jeito principal de entrar e de confirmar ações sensíveis, como rotacionar credenciais.",
		"security.s2.title": "App autenticador",
		"security.s2.body":  "O código TOTP é a alternativa quando a passkey não está à mão. Ações críticas ainda podem pedir uma passkey usada há pouco.",
		"security.s3.title": "Recuperação",
		"security.s3.body":  "Não existe “esqueci minha senha” por email. Um operador inicia a recuperação com um token curto e de uso único, e você cadastra uma passkey nova no lugar da antiga.",

		"recovery.meta_title":   "Recuperação",
		"recovery.back":         "Voltar para o login",
		"recovery.title":        "Recuperar conta",
		"recovery.lead":         "Esta recuperação foi iniciada por um operador. O token fica no fragmento da URL e nunca vai sozinho para o servidor.",
		"recovery.notice_title": "Use um dispositivo confiável",
		"recovery.notice_body":  "Continue só em um dispositivo seu e cadastre uma passkey nova para substituir o acesso antigo.",
		"recovery.start":        "Cadastrar passkey nova",

		"register.meta_title":      "Criar conta",
		"register.title":           "Criar conta",
		"register.lead_app":        "Crie sua conta MattediWorks para continuar no %s.",
		"register.lead_standalone": "Uma conta MattediWorks para entrar nos apps do ecossistema.",
		"register.username":        "Nome de usuário",
		"register.email":           "Email",
		"register.password":        "Senha",
		"register.hint":            "Depois de criar a conta, adicione uma passkey na página da conta.",
		"register.submit":          "Criar conta",
		"register.have_account":    "Já tenho conta",
		"register.error_taken":     "Esse email ou nome de usuário já está em uso.",
		"register.error_generic":   "Não deu para criar a conta. Confira os dados e tente de novo.",

		"continue.meta_title": "Continuando",
		"continue.title":      "Continuando…",
		"continue.body":       "Levando você de volta para o app.",
		"continue.button":     "Continuar",
	},
	LangEN: {
		"lang.switch_code":  "PT",
		"lang.switch_label": "Ver em português",
		"common.theme":      "Toggle theme",
		"common.back":       "Back",
		"common.privacy":    "Privacy",
		"common.security":   "Security",
		"common.account":    "Account",
		"common.copyright":  "© 2026 MattediWorks",

		"account.meta_title":         "Your account",
		"account.your_account":       "Your account",
		"account.strong_title":       "One more step",
		"account.strong_body":        "The app you were using requires strong sign-in: a passkey or an authenticator app. Set one up below, then",
		"account.strong_link":        "go back to the app",
		"account.badge_superadmin":   "Superadmin",
		"account.badge_active":       "Active account",
		"account.tabs_label":         "Account settings",
		"account.passkeys_title":     "Passkeys",
		"account.passkeys_lead":      "Sign in with biometrics, Windows Hello or a security key. No password.",
		"account.passkey_name":       "Passkey name",
		"account.passkey_add":        "Add a passkey",
		"account.totp_title":         "Authenticator app",
		"account.totp_lead":          "A 6-digit code for when your passkey isn’t at hand.",
		"account.totp_setup":         "Set up an authenticator app",
		"account.totp_qr_alt":        "QR code to set up your authenticator app",
		"account.totp_scan":          "Scan the QR code",
		"account.totp_apps":          "Works with Google Authenticator, Microsoft Authenticator, 1Password or any TOTP app.",
		"account.totp_manual":        "Or enter this key manually:",
		"account.totp_code":          "Code from the app",
		"account.totp_confirm":       "Confirm code",
		"account.totp_disable":       "Turn off authenticator app",
		"account.session_title":      "Current session",
		"account.session_lead":       "How you signed in to MW Identity this time.",
		"account.email":              "Email",
		"account.email_verification": "Email verification",
		"account.verified":           "Verified",
		"account.not_verified":       "Not verified",
		"account.username":           "Username",
		"account.signed_in_with":     "Signed in with",
		"account.session_active":     "Active session",
		"account.permission":         "Permission",
		"account.user":               "User",
		"account.reconfirm":          "Confirm it’s you again",
		"account.apps_title":         "Connected apps",
		"account.apps_lead":          "Apps that use your MattediWorks account.",
		"account.notice_removed":     "App removed. It confirmed it will delete your data.",
		"account.notice_no_callback": "App removed and access revoked. It doesn’t take automatic deletion requests, so contact the app to delete your data.",
		"account.notice_pending":     "Access was revoked, but the app hasn’t confirmed the deletion yet. Try removing it again in a few minutes.",
		"account.remove":             "Remove",
		"account.remove_body":        "Access ends right away and %s is asked to delete your data. If the app allows it, you can sign in again later.",
		"account.remove_confirm":     "Remove %s from my account",
		"account.remove_button":      "Remove app",
		"account.apps_empty":         "No connected apps yet.",
		"value.password":             "Password",
		"value.webauthn":             "Passkey",
		"value.totp":                 "Authenticator app",
		"value.user":                 "User",
		"value.admin":                "Admin",
		"value.superadmin":           "Superadmin",
		"account.logout_title":       "Sign out",
		"account.logout_lead":        "Ends your MW Identity session in this browser.",
		"account.logout_button":      "Sign out",
		"account.delete_title":       "Delete account",
		"account.delete_lead":        "Revokes all access and asks every connected app to delete your account. This can’t be undone.",
		"account.delete_label":       "Type DELETE to confirm",
		"account.delete_button":      "Delete my account",

		"info.tabs_label": "About MW Identity",
		"info.note":       "This is a summary of how MW Identity works, not a legal document.",

		"privacy.title":    "Privacy",
		"privacy.lead":     "How MW Identity uses and protects the data it needs to run your account.",
		"privacy.s1.title": "Account data",
		"privacy.s1.body":  "We keep your stable ID, username, name, email, whether the email is verified, and how you sign in. Each app only receives what its authorized scopes allow.",
		"privacy.s2.title": "Connected apps",
		"privacy.s2.body":  "Your account page lists every connected app. Each app keeps its own profile and data. Remove an app and its access ends right away, and it is asked to delete your data; the rest of your account is unaffected.",
		"privacy.s3.title": "Sessions and audit log",
		"privacy.s3.body":  "Sessions, security events and administrative actions go into the audit log, to protect your account, investigate problems and run the service.",
		"privacy.s4.title": "Credentials",
		"privacy.s4.body":  "Passwords and tokens are never stored in plain text. Every secret is hashed or encrypted, depending on what it is for.",
		"privacy.s5.title": "Deleting your account",
		"privacy.s5.body":  "When you delete your account, MW Identity revokes all access immediately and notifies every connected app. Each app deletes or anonymizes your data under its own retention rules. If an app does not confirm, the account stays disabled and pending deletion until an admin retries.",

		"security.title":    "Security",
		"security.lead":     "What protects your account and the MattediWorks ecosystem.",
		"security.s1.title": "Passkeys",
		"security.s1.body":  "A passkey is the main way to sign in and to confirm sensitive actions, such as rotating credentials.",
		"security.s2.title": "Authenticator app",
		"security.s2.body":  "A TOTP code is the fallback when your passkey isn’t at hand. Critical actions can still require a recently used passkey.",
		"security.s3.title": "Recovery",
		"security.s3.body":  "There is no “forgot my password” email. An operator starts recovery with a short-lived, single-use token, and you register a new passkey to replace the old one.",

		"recovery.meta_title":   "Recovery",
		"recovery.back":         "Back to sign-in",
		"recovery.title":        "Recover your account",
		"recovery.lead":         "An operator started this recovery. The token stays in the URL fragment and is never sent to the server on its own.",
		"recovery.notice_title": "Use a trusted device",
		"recovery.notice_body":  "Only continue on a device you trust, and register a new passkey to replace the old access.",
		"recovery.start":        "Register a new passkey",

		"register.meta_title":      "Create account",
		"register.title":           "Create account",
		"register.lead_app":        "Create your MattediWorks account to continue to %s.",
		"register.lead_standalone": "One MattediWorks account to sign in to apps across the ecosystem.",
		"register.username":        "Username",
		"register.email":           "Email",
		"register.password":        "Password",
		"register.hint":            "After creating the account, add a passkey from your account page.",
		"register.submit":          "Create account",
		"register.have_account":    "I already have an account",
		"register.error_taken":     "That email or username is already in use.",
		"register.error_generic":   "The account could not be created. Check the details and try again.",

		"continue.meta_title": "Continuing",
		"continue.title":      "Continuing…",
		"continue.body":       "Taking you back to the app.",
		"continue.button":     "Continue",
	},
}

// Value labels a stored value (an auth method, a role) in lang, falling back
// to the raw value when the catalog does not know it.
func Value(lang, value string) string {
	if label := T(lang, "value."+value); label != "value."+value {
		return label
	}
	return value
}

// T returns the message for key in lang, formatting args into it when given.
// Unknown languages fall back to Portuguese; unknown keys render as the key,
// which the catalog test prevents from shipping.
func T(lang, key string, args ...any) string {
	catalog, ok := catalogs[lang]
	if !ok {
		catalog = catalogs[LangPT]
	}
	message, ok := catalog[key]
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(message, args...)
	}
	return message
}
