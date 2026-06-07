import { useState, useCallback } from 'react'
import { ShieldBan, Trash2 } from 'lucide-react'

export default function BannedAccountSection({ t, form, setForm, apiFetch, onMessage, onRefresh }) {
    const [cleaning, setCleaning] = useState(false)
    const autoCleanBanned = form.runtime?.auto_clean_banned ?? false

    const handleCleanBanned = useCallback(async () => {
        if (!window.confirm(t('settings.cleanBannedConfirm'))) {
            return
        }
        setCleaning(true)
        try {
            const res = await apiFetch('/admin/accounts/clean-banned', { method: 'POST' })
            const data = await res.json()
            if (!res.ok) {
                onMessage('error', data.detail || t('settings.cleanBannedFailed'))
                return
            }
            if (data.removed === 0) {
                onMessage('success', t('settings.cleanBannedNone'))
            } else {
                onMessage('success', t('settings.cleanBannedSuccess', { count: data.removed }))
            }
            if (typeof onRefresh === 'function') {
                onRefresh()
            }
        } catch (_e) {
            onMessage('error', t('settings.cleanBannedFailed'))
        } finally {
            setCleaning(false)
        }
    }, [apiFetch, onMessage, onRefresh, t])

    return (
        <div className="bg-card border border-border rounded-xl p-5 space-y-4">
            <div className="flex items-center gap-2">
                <ShieldBan className="w-4 h-4 text-muted-foreground" />
                <h3 className="font-semibold">{t('settings.bannedAccountsTitle')}</h3>
            </div>
            <p className="text-sm text-muted-foreground">{t('settings.bannedAccountsDesc')}</p>

            <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                    <label className="text-sm font-medium leading-6">{t('settings.autoCleanBanned')}</label>
                    <p className="text-xs text-muted-foreground">{t('settings.autoCleanBannedDesc')}</p>
                </div>
                <button
                    type="button"
                    role="switch"
                    aria-checked={autoCleanBanned}
                    onClick={() => setForm((prev) => ({
                        ...prev,
                        runtime: { ...(prev.runtime || {}), auto_clean_banned: !autoCleanBanned },
                    }))}
                    className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 ${autoCleanBanned ? 'bg-primary' : 'bg-input'}`}
                >
                    <span
                        className={`pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform duration-200 ease-in-out ${autoCleanBanned ? 'translate-x-4' : 'translate-x-0'}`}
                    />
                </button>
            </div>

            <button
                type="button"
                onClick={handleCleanBanned}
                disabled={cleaning}
                className="px-4 py-2 rounded-lg border border-destructive/40 text-destructive hover:bg-destructive/10 flex items-center gap-2 text-sm"
            >
                <Trash2 className="w-4 h-4" />
                {cleaning ? t('actions.loading') : t('settings.cleanBannedNow')}
            </button>
        </div>
    )
}
