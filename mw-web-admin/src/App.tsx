import { LogIn, ShieldCheck } from 'lucide-react'
import { lazy, Suspense, useEffect, type ReactNode } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { AuthProvider } from '@/auth/AuthProvider'
import { ProtectedRoute } from '@/auth/guards'
import { AppConfirmProvider } from '@/components/confirm-provider'
import { MwWordmark } from '@/components/mw-wordmark'
import { ThemeToggle } from '@/components/theme-toggle'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/auth/auth-context'
import { beginIdentitySetup } from '@/lib/control-api'
import AppShell from '@/pages/AppShell'
import OAuthProjectLayout, {
  OAuthProvidersRedirect,
} from '@/pages/Control/OAuthProjectLayout'
import Home from '@/pages/Home/Home'
import Profile from '@/pages/Profile/Profile'
import { PreferencesProvider } from '@/preferences/preferences-context'
import { usePreferences } from '@/preferences/preferences-context'
import {
  IdentityApplicationsPage,
  IdentityAssignmentsPage,
  IdentityAuditPage,
  IdentitySessionsPage,
  IdentityUsersPage,
  MailAuditPage,
  MailEventsPage,
  MailDocsPage,
  MailProjectsPage,
  MailUsagePage,
  MqttDocsPage,
  OAuthAuditPage,
  OAuthDocsPage,
  OAuthMetricsPage,
  OAuthProjectsPage,
  OAuthServerPage,
} from '@/pages/Control/pages'

const OAuthDocPage = lazy(() => import('@/pages/Control/OAuthDocPage'))
const OAuthProjectConfigurationPage = lazy(
  () => import('@/pages/Control/OAuthProjectConfigurationPage'),
)
const OAuthProvidersPage = lazy(
  () => import('@/pages/Control/OAuthProvidersPage'),
)
const OAuthProjectActivityPage = lazy(
  () => import('@/pages/Control/OAuthProjectActivityPage'),
)
const MailConfigurationPage = lazy(
  () => import('@/pages/Control/MailConfigurationPage'),
)
const MqttClientsPage = lazy(() => import('@/pages/Control/MqttClientsPage'))
const MqttServerPage = lazy(() => import('@/pages/Control/MqttServerPage'))
const MqttRolesPage = lazy(() => import('@/pages/Control/MqttRolesPage'))
const IdentityApplicationDetailPage = lazy(() =>
  import('@/pages/Control/IdentityDetailPages').then((module) => ({
    default: module.IdentityApplicationDetailPage,
  })),
)
const IdentityUserDetailPage = lazy(() =>
  import('@/pages/Control/IdentityDetailPages').then((module) => ({
    default: module.IdentityUserDetailPage,
  })),
)

function DeferredPage({ children }: { children: ReactNode }) {
  return (
    <Suspense
      fallback={
        <main className="text-muted-foreground grid min-h-dvh place-items-center text-sm">
          Loading…
        </main>
      }
    >
      {children}
    </Suspense>
  )
}

function LoginPage() {
  const auth = useAuth()
  const { tr } = usePreferences()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from ?? '/'

  useEffect(() => {
    if (auth.status === 'authenticated') window.location.replace(from)
  }, [auth.status, from])

  return (
    <main className="bg-background relative grid min-h-dvh place-items-center overflow-hidden p-4">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_10%_10%,color-mix(in_srgb,var(--brand-orange)_14%,transparent),transparent_34rem),radial-gradient(circle_at_90%_90%,color-mix(in_srgb,var(--primary)_14%,transparent),transparent_32rem)]" />
      <ThemeToggle className="absolute top-5 right-5 z-10" />
      <section className="border-border/70 bg-card shadow-primary/10 relative w-full max-w-md rounded-[2rem] border p-7 shadow-2xl sm:p-9">
        <MwWordmark className="mb-8" />
        <h1 className="flex items-center justify-center gap-2 text-center text-2xl font-semibold tracking-tight">
          <ShieldCheck
            className="text-brand-orange size-5"
            aria-hidden="true"
          />
          MW Control Plane
        </h1>
        <p className="text-muted-foreground mx-auto mt-2 max-w-sm text-center text-sm leading-6">
          {tr('Acesse identidades, OAuth e e-mail com sua conta MattediWorks.')}
        </p>
        <Button className="mt-7 w-full" onClick={auth.login} size="lg">
          {tr('Entrar com MattediWorks')} <LogIn />
        </Button>
        <button
          className="text-muted-foreground hover:text-foreground mx-auto mt-4 block text-xs font-medium underline-offset-4 hover:underline"
          onClick={beginIdentitySetup}
          type="button"
        >
          {tr('Primeiro acesso ou configurar TOTP')}
        </button>
        <p className="text-muted-foreground mt-5 text-center text-xs">
          {tr(
            'Senha, passkey e TOTP são verificados com segurança no MW Identity.',
          )}
        </p>
      </section>
    </main>
  )
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<AppShell />}>
          <Route index element={<Home />} />
          <Route path="profile" element={<Profile />} />
          <Route path="identity/users" element={<IdentityUsersPage />} />
          <Route
            path="identity/users/:userID"
            element={
              <DeferredPage>
                <IdentityUserDetailPage />
              </DeferredPage>
            }
          />
          <Route
            path="identity/applications"
            element={<IdentityApplicationsPage />}
          />
          <Route
            path="identity/applications/:applicationID"
            element={
              <DeferredPage>
                <IdentityApplicationDetailPage />
              </DeferredPage>
            }
          />
          <Route
            path="identity/assignments"
            element={<IdentityAssignmentsPage />}
          />
          <Route path="identity/sessions" element={<IdentitySessionsPage />} />
          <Route path="identity/audit" element={<IdentityAuditPage />} />
          <Route path="oauth/projects" element={<OAuthProjectsPage />} />
          <Route
            path="oauth/projects/:projectID"
            element={<OAuthProjectLayout />}
          >
            <Route
              index
              element={
                <DeferredPage>
                  <OAuthProjectConfigurationPage key="overview" />
                </DeferredPage>
              }
            />
            <Route
              path="credentials"
              element={
                <DeferredPage>
                  <OAuthProjectConfigurationPage
                    key="credentials"
                    section="credentials"
                  />
                </DeferredPage>
              }
            />
            <Route
              path="providers"
              element={<Navigate replace to="google" />}
            />
            <Route
              path="providers/google"
              element={
                <DeferredPage>
                  <OAuthProvidersPage />
                </DeferredPage>
              }
            />
            <Route
              path="activity"
              element={
                <DeferredPage>
                  <OAuthProjectActivityPage />
                </DeferredPage>
              }
            />
            <Route
              path="configuration"
              element={<Navigate replace to="../credentials" />}
            />
          </Route>
          <Route path="oauth/providers" element={<OAuthProvidersRedirect />} />
          <Route path="oauth/audit" element={<OAuthAuditPage />} />
          <Route path="oauth/metrics" element={<OAuthMetricsPage />} />
          <Route path="oauth/server" element={<OAuthServerPage />} />
          <Route path="oauth/docs" element={<OAuthDocsPage />} />
          <Route
            path="oauth/docs/:version"
            element={
              <Suspense
                fallback={
                  <main className="text-muted-foreground grid min-h-dvh place-items-center text-sm">
                    Loading documentation…
                  </main>
                }
              >
                <OAuthDocPage />
              </Suspense>
            }
          />
          <Route
            path="mqtt/clients"
            element={
              <DeferredPage>
                <MqttClientsPage />
              </DeferredPage>
            }
          />
          <Route path="mqtt/docs" element={<MqttDocsPage />} />
          <Route
            path="mqtt/docs/:version"
            element={
              <DeferredPage>
                <OAuthDocPage module="MQTT" basePath="/mqtt/docs" />
              </DeferredPage>
            }
          />
          <Route
            path="mqtt/roles"
            element={
              <DeferredPage>
                <MqttRolesPage />
              </DeferredPage>
            }
          />
          <Route
            path="mqtt/server"
            element={
              <DeferredPage>
                <MqttServerPage />
              </DeferredPage>
            }
          />
          <Route path="mail/projects" element={<MailProjectsPage />} />
          <Route
            path="mail/configuration"
            element={
              <DeferredPage>
                <MailConfigurationPage />
              </DeferredPage>
            }
          />
          <Route
            path="mail/configuration/:projectID"
            element={
              <DeferredPage>
                <MailConfigurationPage />
              </DeferredPage>
            }
          />
          <Route path="mail/events" element={<MailEventsPage />} />
          <Route path="mail/usage" element={<MailUsagePage />} />
          <Route path="mail/audit" element={<MailAuditPage />} />
          <Route path="mail/docs" element={<MailDocsPage />} />
          <Route
            path="mail/docs/:version"
            element={
              <DeferredPage>
                <OAuthDocPage module="Mail" basePath="/mail/docs" />
              </DeferredPage>
            }
          />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default function App() {
  return (
    <PreferencesProvider>
      <AuthProvider>
        <AppConfirmProvider>
          <AppRoutes />
        </AppConfirmProvider>
      </AuthProvider>
    </PreferencesProvider>
  )
}
