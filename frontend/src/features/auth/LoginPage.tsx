import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useNavigate, useSearchParams } from 'react-router'
import { toast } from 'sonner'
import { Loader2, Lock, User } from 'lucide-react'
import { useI18n } from '@/i18n'
import { login } from '@/lib/api'
import { getErrorMessage, setSession } from '@/lib/api-client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/ui/password-input'
import { Label } from '@/components/ui/label'

const loginSchema = z.object({
  username: z.string().min(1),
  password: z.string().min(1),
})

type LoginForm = z.infer<typeof loginSchema>

export function LoginPage() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginForm>({
    resolver: zodResolver(loginSchema),
    defaultValues: { username: '', password: '' },
  })

  async function onSubmit(values: LoginForm) {
    setLoading(true)
    setError(null)
    try {
      const res = await login(values.username, values.password)
      setSession(res.token, res.expires_at)
      toast.success(t('login.success'))
      const redirect = searchParams.get('redirect')
      navigate(redirect && redirect.startsWith('/') ? redirect : '/dashboard', { replace: true })
    } catch (e) {
      setError(getErrorMessage(e, t('login.failed')))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-dvh items-center justify-center bg-muted/40 px-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex flex-col items-center gap-2">
          <div className="flex size-10 items-center justify-center rounded-lg bg-primary font-bold text-lg text-primary-foreground">
            z
          </div>
          <h1 className="text-2xl font-bold tracking-tight">zPush</h1>
          <p className="text-sm text-muted-foreground">{t('login.subtitle')}</p>
        </div>

        <form
          onSubmit={handleSubmit(onSubmit)}
          className="space-y-4 rounded-xl border bg-card p-6 shadow-sm"
        >
          <div className="space-y-2">
            <Label htmlFor="username">{t('login.username')}</Label>
            <div className="relative">
              <User className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                id="username"
                autoComplete="username"
                placeholder={t('login.enterUsername')}
                className="pl-8"
                {...register('username')}
              />
            </div>
            {errors.username ? (
              <p className="text-xs text-destructive">{t('login.usernameRequired')}</p>
            ) : null}
          </div>

          <div className="space-y-2">
            <Label htmlFor="password">{t('login.password')}</Label>
            <div className="relative">
              <Lock className="pointer-events-none absolute top-1/2 left-2.5 z-10 size-4 -translate-y-1/2 text-muted-foreground" />
              <PasswordInput
                id="password"
                autoComplete="current-password"
                placeholder={t('login.enterPassword')}
                className="pl-8"
                {...register('password')}
              />
            </div>
            {errors.password ? (
              <p className="text-xs text-destructive">{t('login.passwordRequired')}</p>
            ) : null}
          </div>

          {error ? (
            <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
          ) : null}

          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? <Loader2 className="size-4 animate-spin" /> : null}
            {t('login.submit')}
          </Button>
        </form>
      </div>
    </div>
  )
}
