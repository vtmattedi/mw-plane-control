import {
  Languages,
  LoaderCircle,
  LogOut,
  MonitorCog,
  Palette,
  ShieldCheck,
  UserRound,
} from 'lucide-react'
import { useState } from 'react'
import { useAuth } from '@/auth/auth-context'
import { PageHeader } from '@/components/control/page-header'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { usePreferences } from '@/preferences/preferences-context'
import type { ReactNode } from 'react'

export default function Profile() {
  const auth = useAuth()
  const preferences = usePreferences()
  const [loggingOut, setLoggingOut] = useState(false)
  const { tr } = preferences

  async function logout() {
    setLoggingOut(true)
    try {
      await auth.logout()
    } finally {
      setLoggingOut(false)
    }
  }

  return (
    <main className="bg-background min-h-dvh px-4 py-7 sm:px-7 lg:px-10">
      <div className="mx-auto w-full max-w-5xl">
        <PageHeader
          description={tr(
            'Gerencie sua identidade e preferências locais deste navegador.',
          )}
          eyebrow="MattediWorks"
          icon={UserRound}
          title={tr('Perfil')}
        />
        <div className="mt-7 grid gap-5 lg:grid-cols-[1fr_1.4fr]">
          <Card className="border-border/70 shadow-sm">
            <CardHeader className="flex-row items-center justify-between space-y-0">
              <CardTitle className="flex items-center gap-2">
                <ShieldCheck className="text-brand-orange size-4" />
                {tr('Identidade')}
              </CardTitle>
              <Button
                aria-label={tr('Sair')}
                className="text-destructive hover:bg-destructive/10"
                disabled={loggingOut}
                onClick={() => void logout()}
                size="sm"
                variant="ghost"
              >
                {loggingOut ? (
                  <LoaderCircle className="animate-spin" />
                ) : (
                  <LogOut />
                )}
                {loggingOut ? tr('Saindo…') : tr('Sair')}
              </Button>
            </CardHeader>
            <CardContent className="space-y-4 text-sm">
              <ProfileValue label={tr('Nome')} value={auth.user?.name || '—'} />
              <ProfileValue label="E-mail" value={auth.user?.email || '—'} />
              <ProfileValue
                label={tr('Método de autenticação')}
                value={auth.user?.authMethod || '—'}
              />
              <ProfileValue
                label={tr('Nível de garantia')}
                value={auth.user?.assurance || '—'}
              />
            </CardContent>
          </Card>
          <Card className="border-border/70 shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <MonitorCog className="text-brand-orange size-4" />
                {tr('Preferências')}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <PreferenceRow
                icon={Languages}
                label={tr('Idioma')}
                description="Português / English"
              >
                <ToggleGroup
                  aria-label={tr('Idioma')}
                  onValueChange={(value) =>
                    value && preferences.setLanguage(value as 'pt' | 'en')
                  }
                  value={preferences.language}
                >
                  <ToggleGroupItem value="pt">PT</ToggleGroupItem>
                  <ToggleGroupItem value="en">EN</ToggleGroupItem>
                </ToggleGroup>
              </PreferenceRow>
              <PreferenceRow
                icon={Palette}
                label={tr('Tema')}
                description={tr('Escolha a aparência da interface.')}
              >
                <ToggleGroup
                  aria-label={tr('Tema')}
                  onValueChange={(value) =>
                    value && preferences.setTheme(value as 'light' | 'dark')
                  }
                  value={preferences.theme}
                >
                  <ToggleGroupItem value="light">{tr('Claro')}</ToggleGroupItem>
                  <ToggleGroupItem value="dark">{tr('Escuro')}</ToggleGroupItem>
                </ToggleGroup>
              </PreferenceRow>
              <PreferenceRow
                icon={MonitorCog}
                label={tr('Densidade')}
                description={tr('Ajuste o espaçamento dos painéis.')}
              >
                <ToggleGroup
                  aria-label={tr('Densidade')}
                  onValueChange={(value) =>
                    value &&
                    preferences.setDensity(value as 'comfortable' | 'compact')
                  }
                  value={preferences.density}
                >
                  <ToggleGroupItem value="comfortable">
                    {tr('Confortável')}
                  </ToggleGroupItem>
                  <ToggleGroupItem value="compact">
                    {tr('Compacta')}
                  </ToggleGroupItem>
                </ToggleGroup>
              </PreferenceRow>
            </CardContent>
          </Card>
        </div>
      </div>
    </main>
  )
}

function ProfileValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-border/60 border-b pb-3 last:border-0">
      <p className="text-muted-foreground text-xs">{label}</p>
      <p className="mt-1 font-medium">{value}</p>
    </div>
  )
}

function PreferenceRow({
  icon: Icon,
  label,
  description,
  children,
}: {
  icon: typeof Languages
  label: string
  description: string
  children: ReactNode
}) {
  return (
    <div className="border-border/60 flex flex-col gap-3 border-b pb-5 last:border-0 last:pb-0 sm:flex-row sm:items-center">
      <span className="bg-muted grid size-10 shrink-0 place-items-center rounded-xl">
        <Icon className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-sm font-semibold">{label}</p>
        <p className="text-muted-foreground text-xs">{description}</p>
      </div>
      {children}
    </div>
  )
}
