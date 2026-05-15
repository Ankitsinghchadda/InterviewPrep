import { useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { LogOut, Menu, Sparkles, User as UserIcon, X } from 'lucide-react'

import { useAuth } from '@/auth/AuthContext'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'

const NAV_ITEMS = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/topics', label: 'Topics' },
  { to: '/questions', label: 'Questions' },
  { to: '/interview', label: 'Mock Interview' },
]

export function Layout() {
  const { user, status, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const isLanding = location.pathname === '/'
  const authed = status === 'authenticated'
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const closeMobileNav = () => setMobileNavOpen(false)

  const handleLogout = async () => {
    await logout()
    navigate('/', { replace: true })
  }

  const initials = (user?.name || user?.email || '?')
    .split(/\s+|@/)
    .filter(Boolean)
    .slice(0, 2)
    .map((s) => s[0]?.toUpperCase())
    .join('')

  return (
    <div className="min-h-screen flex flex-col">
      <header
        className={cn(
          'sticky top-0 z-40 w-full border-b border-border/60 backdrop-blur',
          isLanding ? 'bg-background/40' : 'bg-background/80',
        )}
      >
        <div className="mx-auto flex h-14 max-w-7xl items-center gap-3 px-4 sm:gap-6 sm:px-6 lg:px-8">
          <Link to="/" className="flex items-center gap-2 font-semibold tracking-tight">
            <span className="grid size-7 place-items-center rounded-md bg-gradient-to-br from-brand-400 to-brand-700 text-white shadow-sm">
              <Sparkles className="size-4" />
            </span>
            <span>10xInterview</span>
          </Link>

          {authed && (
            <nav className="hidden items-center gap-1 md:flex">
              {NAV_ITEMS.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={({ isActive }) =>
                    cn(
                      'rounded-md px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground',
                      isActive && 'bg-accent text-foreground',
                    )
                  }
                >
                  {item.label}
                </NavLink>
              ))}
            </nav>
          )}

          <div className="ml-auto flex items-center gap-2">
            {status === 'loading' ? (
              <div className="h-8 w-20 animate-pulse rounded-md bg-muted" />
            ) : authed && user ? (
              <>
                <button
                  type="button"
                  onClick={() => setMobileNavOpen((v) => !v)}
                  aria-label={mobileNavOpen ? 'Close menu' : 'Open menu'}
                  aria-expanded={mobileNavOpen}
                  className="inline-flex size-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground md:hidden"
                >
                  {mobileNavOpen ? <X className="size-5" /> : <Menu className="size-5" />}
                </button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      className="flex items-center gap-2 rounded-full p-0.5 outline-none ring-offset-background transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                    >
                      <Avatar>
                        {user.pictureUrl && <AvatarImage src={user.pictureUrl} alt={user.name || user.email} />}
                        <AvatarFallback>{initials || 'U'}</AvatarFallback>
                      </Avatar>
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="min-w-56">
                    <DropdownMenuLabel className="font-normal">
                      <div className="flex flex-col">
                        <span className="text-sm font-medium">{user.name || 'Signed in'}</span>
                        <span className="text-xs text-muted-foreground">{user.email}</span>
                      </div>
                    </DropdownMenuLabel>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onSelect={() => navigate('/dashboard')}>
                      <UserIcon className="size-4" />
                      Dashboard
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => navigate('/onboarding')}>
                      <UserIcon className="size-4" />
                      Edit profile
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onSelect={handleLogout}>
                      <LogOut className="size-4" />
                      Sign out
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </>
            ) : (
              <Button asChild size="sm" variant="ghost">
                <Link to="/login">Sign in</Link>
              </Button>
            )}
          </div>
        </div>

        {authed && mobileNavOpen && (
          <nav className="border-t border-border/60 bg-background/95 md:hidden">
            <div className="mx-auto flex max-w-7xl flex-col gap-1 px-4 py-3 sm:px-6">
              {NAV_ITEMS.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  onClick={closeMobileNav}
                  className={({ isActive }) =>
                    cn(
                      'rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground',
                      isActive && 'bg-accent text-foreground',
                    )
                  }
                >
                  {item.label}
                </NavLink>
              ))}
            </div>
          </nav>
        )}
      </header>

      <main className={cn('flex-1', !isLanding && 'mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 sm:py-8 lg:px-8')}>
        <Outlet />
      </main>

      {!isLanding && (
        <footer className="border-t border-border/60 py-6 text-xs text-muted-foreground">
          <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-2 px-4 sm:flex-row sm:px-6 lg:px-8">
            <span>© {new Date().getFullYear()} 10xInterview — practice, record, improve.</span>
            <Link to="/contact" className="hover:text-foreground">
              Contact
            </Link>
          </div>
        </footer>
      )}
    </div>
  )
}
