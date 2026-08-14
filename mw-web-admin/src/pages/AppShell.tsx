import {
  AppWindow,
  BookOpen,
  ChartNoAxesCombined,
  ChevronDown,
  ChevronUp,
  FileClock,
  FolderKanban,
  Gauge,
  Home,
  KeyRound,
  Languages,
  Link2,
  LogOut,
  Mail,
  Menu,
  Moon,
  Network,
  ArrowRightToLine,
  ArrowLeftToLine,
  ServerCog,
  Settings2,
  Sun,
  UserRound,
  Users,
  X,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '@/auth/auth-context'
import { BrandLogo } from '@/components/brand-logo'
import EmailTester from '@/components/EmailTester'
import { Button } from '@/components/ui/button'
import { usePreferences } from '@/preferences/preferences-context'
import type { LucideIcon } from 'lucide-react'
import styles from './app.module.scss'
type Item = { label: string; to: string; icon: LucideIcon; end?: boolean }
type Group = { label: string; items: Item[] }

const groups: Group[] = [
  {
    label: 'Workspace',
    items: [{ label: 'Visão geral', to: '/', icon: Home, end: true }],
  },
  {
    label: 'Identity',
    items: [
      { label: 'Usuários', to: '/identity/users', icon: Users },
      { label: 'Aplicações', to: '/identity/applications', icon: AppWindow },
      { label: 'Atribuições', to: '/identity/assignments', icon: Link2 },
      { label: 'Sessões', to: '/identity/sessions', icon: KeyRound },
      { label: 'Auditoria', to: '/identity/audit', icon: FileClock },
    ],
  },
  {
    label: 'OAuth',
    items: [
      { label: 'Projetos', to: '/oauth/projects', icon: FolderKanban },
      { label: 'Provedores', to: '/oauth/providers', icon: Network },
      { label: 'Métricas', to: '/oauth/metrics', icon: ChartNoAxesCombined },
      { label: 'Auditoria', to: '/oauth/audit', icon: FileClock },
      { label: 'Servidor', to: '/oauth/server', icon: ServerCog },
      { label: 'Documentação', to: '/oauth/docs', icon: BookOpen },
    ],
  },
  {
    label: 'Mail',
    items: [
      { label: 'Projetos', to: '/mail/projects', icon: FolderKanban },
      { label: 'Configuração', to: '/mail/configuration', icon: Settings2 },
      { label: 'Mensagens', to: '/mail/events', icon: Mail },
      { label: 'Uso e métricas', to: '/mail/usage', icon: Gauge },
      { label: 'Auditoria', to: '/mail/audit', icon: FileClock },
      { label: 'Documentação', to: '/mail/docs', icon: BookOpen },
    ],
  },
]

const storageKey = 'mw-control-sidebar-collapsed'

export default function AppShell() {
  const auth = useAuth()
  const location = useLocation()
  const preferences = usePreferences()
  const [mobileOpen, setMobileOpen] = useState(false)
  const [collapsed, setCollapsed] = useState(
    () => localStorage.getItem(storageKey) === 'true',
  )
  const [loggingOut, setLoggingOut] = useState(false)
  useEffect(() => setMobileOpen(false), [location.pathname])
  useEffect(
    () => localStorage.setItem(storageKey, String(collapsed)),
    [collapsed],
  )

  async function logout() {
    setLoggingOut(true)
    try {
      await auth.logout()
    } finally {
      setLoggingOut(false)
    }
  }

  const sidebarProps = {
    email: auth.user?.email ?? '',
    name: auth.user?.name ?? '',
    loggingOut,
    onLogout: () => void logout(),
  }

  return (
    <div className="bg-background text-foreground min-h-dvh">
      <header className="bg-background/90 sticky top-0 z-30 flex h-16 items-center border-b px-4 backdrop-blur lg:hidden">
        <Button
          aria-label={preferences.tr('Abrir navegação')}
          onClick={() => setMobileOpen(true)}
          size="icon"
          variant="ghost"
        >
          <Menu />
        </Button>
        <BrandLogo className="ml-3 h-8 w-auto" linkTo="/" />
      </header>
      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            aria-label={preferences.tr('Fechar navegação')}
            className="absolute inset-0 bg-black/55 backdrop-blur-sm"
            onClick={() => setMobileOpen(false)}
          />
          <aside className="bg-sidebar relative h-full w-[min(19rem,88vw)] border-r shadow-2xl">
            <Sidebar
              {...sidebarProps}
              collapsed={false}
              mobile
              onClose={() => setMobileOpen(false)}
            />
          </aside>
        </div>
      )}
      <aside
        className={`${styles.sidebar} bg-sidebar fixed inset-y-0 left-0 z-40 hidden transition-[width] lg:block ${collapsed ? 'w-[4.75rem]' : 'w-64'}`}
      >
        <Sidebar
          {...sidebarProps}
          collapsed={collapsed}
          onCollapse={() => setCollapsed((value) => !value)}
        />
      </aside>
      <div
        className={`min-h-dvh transition-[padding] ${collapsed ? 'lg:pl-[4.75rem]' : 'lg:pl-64'}`}
      >
        <Outlet />
      </div>
      {location.pathname.startsWith('/mail') && <EmailTester />}
    </div>
  )
}

function Sidebar({
  collapsed,
  email,
  name,
  loggingOut,
  mobile,
  onClose,
  onCollapse,
  onLogout,
}: {
  collapsed: boolean
  email: string
  name: string
  loggingOut: boolean
  mobile?: boolean
  onClose?: () => void
  onCollapse?: () => void
  onLogout: () => void
}) {
  const location = useLocation()
  const preferences = usePreferences()
  const [profileOpen, setProfileOpen] = useState(false)
  const { tr } = preferences
  useEffect(() => setProfileOpen(false), [location.pathname])
  const initials = (name || email || 'MW')
    .split(/[\s@]+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join('')

  return (
    <div className="relative flex h-full flex-col">
      {!mobile && (
        <button
          aria-label={tr(collapsed ? 'Expandir menu' : 'Recolher menu')}
          className="border-border bg-background text-muted-foreground hover:text-foreground absolute top-1/4 -right-4 z-50 grid size-8 -translate-y-1/2 place-items-center rounded-full border shadow-md transition-colors "
          onClick={onCollapse}
          title={tr(collapsed ? 'Expandir menu' : 'Recolher menu')}
          type="button"
        >
          {collapsed ? (
            <ArrowRightToLine />
          ) : (
            <ArrowLeftToLine  />
          )}
        </button>
      )}
      <div
        className={`border-border/70 flex h-17 items-center border-b ${collapsed ? 'justify-center px-2' : 'gap-3 px-4'}`}
      >
        <BrandLogo
          compact={collapsed}
          className={collapsed ? 'size-9' : 'h-10 w-44'}
          linkTo="/"
        />
        {mobile && (
          <Button
            aria-label={tr('Fechar navegação')}
            onClick={onClose}
            size="icon"
            variant="ghost"
          >
            <X />
          </Button>
        )}
      </div>

      <nav
        aria-label={tr('Navegação principal')}
        className={`flex-1 overflow-y-auto px-3 py-4 scrollbar-thin scrollbar-track-transparent scrollbar-thumb-sidebar-accent/50`}
      >
        {groups.map((group) => {
          const open = collapsed || preferences.groupOpen[group.label] !== false
          return (
            <div className="mb-3" key={group.label}>
              {!collapsed && (
                <button
                  aria-expanded={open}
                  className="text-muted-foreground hover:bg-sidebar-accent mb-1 flex w-full items-center justify-between rounded-lg px-3 py-1 text-[10px] font-bold tracking-[0.14em] uppercase"
                  onClick={() => preferences.toggleGroup(group.label)}
                  type="button"
                >
                  <span>{tr(group.label)}</span>
                  {open ? (
                    <ChevronUp className="size-3" />
                  ) : (
                    <ChevronDown className="size-3" />
                  )}
                </button>
              )}
              {open && (
                <div className="space-y-0.5">
                  {group.items.map((item) => {
                    const Icon = item.icon
                    return (
                      <NavLink
                        className={({ isActive }) =>
                          `${collapsed? '' : 'ml-2'} flex h-9 items-center rounded-xl text-[13px] font-medium transition-colors ${collapsed ? 'justify-center' : 'gap-3 px-3'} ${isActive ? 'bg-primary text-primary-foreground shadow-sm' : 'text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground'}`
                        }
                        data-slot="sidebar-nav-link"
                        end={item.end}
                        key={item.to}
                        title={collapsed ? tr(item.label) : undefined}
                        to={item.to}
                      >
                        <Icon className="size-4 shrink-0" />
                        {!collapsed && (
                          <span className="truncate">{tr(item.label)}</span>
                        )}
                      </NavLink>
                    )
                  })}
                </div>
              )}
            </div>
          )
        })}
      </nav>

      <div className="border-border/70 relative border-t p-3">
        {profileOpen && (
          <div
            className={`border-border bg-card absolute bottom-[calc(100%-0.25rem)] z-50 w-60 overflow-hidden rounded-2xl border p-2 shadow-2xl ${collapsed ? 'left-[calc(100%+0.5rem)]' : 'inset-x-3'}`}
          >
            <Link
              className="hover:bg-muted flex items-center gap-3 rounded-xl px-3 py-2 text-sm"
              to="/profile"
            >
              <UserRound className="size-4" />
              {tr('Perfil')}
            </Link>
            <button
              className="hover:bg-muted flex w-full items-center gap-3 rounded-xl px-3 py-2 text-sm"
              onClick={() =>
                preferences.setTheme(
                  preferences.theme === 'light' ? 'dark' : 'light',
                )
              }
              type="button"
            >
              {preferences.theme === 'light' ? (
                <Moon className="size-4" />
              ) : (
                <Sun className="size-4" />
              )}
              <span>
                {tr('Tema')}:{' '}
                {tr(preferences.theme === 'light' ? 'Claro' : 'Escuro')}
              </span>
            </button>
            <button
              className="hover:bg-muted flex w-full items-center gap-3 rounded-xl px-3 py-2 text-sm"
              onClick={() =>
                preferences.setLanguage(
                  preferences.language === 'pt' ? 'en' : 'pt',
                )
              }
              type="button"
            >
              <Languages className="size-4" />
              <span>
                {tr('Idioma')}: {preferences.language.toUpperCase()}
              </span>
            </button>
            <div className="border-border my-1 border-t" />
            <button
              className="text-destructive hover:bg-destructive/10 flex w-full items-center gap-3 rounded-xl px-3 py-2 text-sm"
              disabled={loggingOut}
              onClick={onLogout}
              type="button"
            >
              <LogOut className="size-4" />
              {loggingOut ? tr('Saindo…') : tr('Sair')}
            </button>
          </div>
        )}

        <button
          aria-expanded={profileOpen}
          className={`bg-sidebar-accent/70 hover:bg-sidebar-accent flex w-full items-center rounded-xl p-2 text-left ${collapsed ? 'justify-center' : 'gap-3'}`}
          onClick={() => setProfileOpen((value) => !value)}
          type="button"
        >
          <span className="bg-primary text-primary-foreground grid size-8 shrink-0 place-items-center rounded-xl text-[10px] font-bold">
            {initials}
          </span>
          {!collapsed && (
            <>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-xs font-semibold">
                  {name || 'MattediWorks'}
                </span>
                <span className="text-muted-foreground block truncate text-[10px]">
                  {email}
                </span>
              </span>
              {profileOpen ? (
                <ChevronDown className="size-" />
              ) : (
                <ChevronUp className="size-4" />
              )}
            </>
          )}
        </button>
      </div>
    </div>
  )
}
