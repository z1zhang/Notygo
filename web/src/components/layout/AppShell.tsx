import { useState, type ReactNode } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router'
import {
  Bell,
  ChevronsLeft,
  FileText,
  Filter,
  Gauge,
  LogOut,
  Menu,
  Notebook,
  Plug,
  ScrollText,
  Send,
  SendHorizontal,
  Settings,
  Shuffle,
  Wand2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { type TranslationKey } from '@/i18n'
import { cn } from '@/lib/utils'
import { clearSession } from '@/lib/api-client'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { LocaleSwitch } from './LocaleSwitch'
import { ThemeToggle } from './ThemeToggle'

interface NavItem {
  to: string
  titleKey: TranslationKey
  icon: ReactNode
}

const mainNav: NavItem[] = [
  { to: '/dashboard', titleKey: 'nav.dashboard', icon: <Gauge className="size-4.5" /> },
  { to: '/endpoints', titleKey: 'nav.endpoints', icon: <Plug className="size-4.5" /> },
  { to: '/mappings', titleKey: 'nav.mappings', icon: <Shuffle className="size-4.5" /> },
  { to: '/rules', titleKey: 'nav.rules', icon: <Filter className="size-4.5" /> },
  { to: '/templates', titleKey: 'nav.templates', icon: <FileText className="size-4.5" /> },
  { to: '/channels', titleKey: 'nav.channels', icon: <Send className="size-4.5" /> },
]

const logsNav: NavItem[] = [
  { to: '/events', titleKey: 'nav.eventLogs', icon: <Bell className="size-4.5" /> },
  { to: '/deliveries', titleKey: 'nav.deliveryLogs', icon: <SendHorizontal className="size-4.5" /> },
  { to: '/audit', titleKey: 'nav.auditLogs', icon: <ScrollText className="size-4.5" /> },
]

const bottomNav: NavItem[] = [
  { to: '/playground', titleKey: 'nav.playground', icon: <Wand2 className="size-4.5" /> },
  { to: '/settings', titleKey: 'nav.settings', icon: <Settings className="size-4.5" /> },
]

function NavLinks({ collapsed, onNavigate }: { collapsed: boolean; onNavigate?: () => void }) {
  const { t } = useTranslation()

  function renderLink(item: NavItem) {
    const link = (
      <NavLink
        key={item.to}
        to={item.to}
        onClick={onNavigate}
        className={({ isActive }) =>
          cn(
            'flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm text-muted-foreground transition-colors',
            'hover:bg-accent hover:text-foreground',
            isActive && 'bg-accent font-medium text-foreground',
          )
        }
      >
        {item.icon}
        {!collapsed && <span>{t(item.titleKey)}</span>}
      </NavLink>
    )
    // Collapsed sidebar shows icon only — hover reveals the page name
    if (!collapsed) return link
    return (
      <Tooltip>
        <TooltipTrigger asChild>{link}</TooltipTrigger>
        <TooltipContent side="right">{t(item.titleKey)}</TooltipContent>
      </Tooltip>
    )
  }

  return (
    <nav className="flex flex-col gap-0.5">
      {mainNav.map(renderLink)}
      {!collapsed && (
        <div className="mt-4 mb-1 flex items-center gap-2 px-2.5 text-xs font-medium tracking-wide text-muted-foreground/70 uppercase">
          <Notebook className="size-3.5" />
          {t('nav.logs')}
        </div>
      )}
      {collapsed && <div className="my-2 border-t" />}
      {logsNav.map(renderLink)}
      <div className="my-2 border-t" />
      {bottomNav.map(renderLink)}
    </nav>
  )
}

function SidebarBrand({ collapsed }: { collapsed: boolean }) {
  return (
    <div className="flex h-14 items-center gap-2 px-3">
      <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-primary font-bold text-primary-foreground text-sm">
        z
      </div>
      {!collapsed && <span className="text-base font-semibold tracking-wide">Notygo</span>}
    </div>
  )
}

export function AppShell() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)

  const currentTitleKey = [...mainNav, ...logsNav, ...bottomNav].find(
    (item) => location.pathname.startsWith(item.to),
  )?.titleKey

  function handleLogout() {
    clearSession()
    navigate('/login', { replace: true })
  }

  return (
    <div className="flex h-dvh overflow-hidden">
      {/* Desktop sidebar */}
      <aside
        className={cn(
          'hidden shrink-0 flex-col border-r bg-sidebar transition-[width] duration-200 md:flex',
          collapsed ? 'w-14' : 'w-56',
        )}
      >
        <SidebarBrand collapsed={collapsed} />
        <div className="flex-1 overflow-y-auto p-2">
          <NavLinks collapsed={collapsed} />
        </div>
        <div className="border-t p-2">
          <Button
            variant="ghost"
            size="icon"
            className="w-full"
            aria-label={collapsed ? t('layout.expand') : t('layout.collapse')}
            onClick={() => setCollapsed(!collapsed)}
          >
            <ChevronsLeft
              className={cn('size-4.5 transition-transform', collapsed && 'rotate-180')}
            />
          </Button>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Top bar */}
        <header className="flex h-14 shrink-0 items-center gap-2 border-b bg-background px-4">
          {/* Mobile nav trigger */}
          <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
            <SheetTrigger asChild>
              <Button variant="ghost" size="icon" className="md:hidden" aria-label={t('nav.logs')}>
                <Menu className="size-5" />
              </Button>
            </SheetTrigger>
            <SheetContent side="left" className="w-64 p-0">
              <div className="flex h-14 items-center border-b px-3">
                <SheetTitle className="flex items-center gap-2">
                  <div className="flex size-7 items-center justify-center rounded-md bg-primary font-bold text-primary-foreground text-sm">
                    z
                  </div>
                  Notygo
                </SheetTitle>
              </div>
              <div className="p-2">
                <NavLinks collapsed={false} onNavigate={() => setMobileOpen(false)} />
              </div>
            </SheetContent>
          </Sheet>
          <h2 className="truncate text-sm font-semibold">
            {currentTitleKey ? t(currentTitleKey) : 'Notygo'}
          </h2>
          <div className="ml-auto flex items-center gap-1">
            <LocaleSwitch />
            <ThemeToggle />
            <Button
              variant="ghost"
              size="sm"
              aria-label={t('nav.logout')}
              onClick={handleLogout}
            >
              <LogOut className="mr-1 size-4" />
              <span className="hidden sm:inline">{t('nav.logout')}</span>
            </Button>
          </div>
        </header>

        {/* [scrollbar-gutter:stable]：main 是页面滚动容器，滚动条出现/消失会改变内容区宽度，
            造成整页内容"被放大"的抖动（如沙盒分析时结构区卸载/挂载） */}
        <main className="min-h-0 flex-1 overflow-y-auto bg-background [scrollbar-gutter:stable]">
          <div className="mx-auto max-w-7xl space-y-5 p-4 md:p-6">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
