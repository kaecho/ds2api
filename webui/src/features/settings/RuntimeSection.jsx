export default function RuntimeSection({ t, form, setForm }) {
    const schedule = form.runtime.account_schedule || 'round_robin'
    return (
        <div className="bg-card border border-border rounded-xl p-5 space-y-4">
            <h3 className="font-semibold">{t('settings.runtimeTitle')}</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <label className="text-sm space-y-2 md:col-span-2">
                    <span className="text-muted-foreground">{t('settings.accountSchedule')}</span>
                    <select
                        value={schedule}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            runtime: { ...prev.runtime, account_schedule: e.target.value },
                        }))}
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    >
                        <option value="round_robin">{t('settings.scheduleRoundRobin')}</option>
                        <option value="fill">{t('settings.scheduleFill')}</option>
                        <option value="least_used">{t('settings.scheduleLeastUsed')}</option>
                        <option value="random">{t('settings.scheduleRandom')}</option>
                    </select>
                    <p className="text-xs text-muted-foreground">{t(`settings.scheduleHelp.${schedule}`)}</p>
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.accountDailyLimit')}</span>
                    <input
                        type="number"
                        min={0}
                        value={form.runtime.account_daily_limit}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            runtime: { ...prev.runtime, account_daily_limit: Number(e.target.value || 0) },
                        }))}
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                    <p className="text-xs text-muted-foreground">{t('settings.accountDailyLimitHelp')}</p>
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.accountMaxInflight')}</span>
                    <input
                        type="number"
                        min={1}
                        value={form.runtime.account_max_inflight}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            runtime: { ...prev.runtime, account_max_inflight: Number(e.target.value || 1) },
                        }))}
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.accountMaxQueue')}</span>
                    <input
                        type="number"
                        min={1}
                        value={form.runtime.account_max_queue}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            runtime: { ...prev.runtime, account_max_queue: Number(e.target.value || 1) },
                        }))}
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.globalMaxInflight')}</span>
                    <input
                        type="number"
                        min={1}
                        value={form.runtime.global_max_inflight}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            runtime: { ...prev.runtime, global_max_inflight: Number(e.target.value || 1) },
                        }))}
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.tokenRefreshIntervalHours')}</span>
                    <input
                        type="number"
                        min={1}
                        max={720}
                        step={1}
                        value={form.runtime.token_refresh_interval_hours}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            runtime: { ...prev.runtime, token_refresh_interval_hours: Number(e.target.value || 1) },
                        }))}
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
            </div>
            <div className="border-t border-border pt-4">
                <h4 className="text-sm font-medium mb-2">{t('settings.retryOnFailureTitle')}</h4>
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                    <label className="text-sm space-y-2">
                        <span className="text-muted-foreground">{t('settings.retryOnFailureMaxAttempts')}</span>
                        <input
                            type="number"
                            min={0}
                            max={100}
                            value={form.runtime.retry_on_failure_max_attempts}
                            onChange={(e) => setForm((prev) => ({
                                ...prev,
                                runtime: { ...prev.runtime, retry_on_failure_max_attempts: Number(e.target.value || 0) },
                            }))}
                            className="w-full bg-background border border-border rounded-lg px-3 py-2"
                        />
                        <p className="text-xs text-muted-foreground">{t('settings.retryOnFailureMaxAttemptsHelp')}</p>
                    </label>
                    <label className="text-sm space-y-2">
                        <span className="text-muted-foreground">{t('settings.retryOnFailureMuteDuration')}</span>
                        <input
                            type="number"
                            min={1}
                            max={1440}
                            step={1}
                            value={form.runtime.retry_on_failure_mute_duration_minutes}
                            onChange={(e) => setForm((prev) => ({
                                ...prev,
                                runtime: { ...prev.runtime, retry_on_failure_mute_duration_minutes: Number(e.target.value || 30) },
                            }))}
                            className="w-full bg-background border border-border rounded-lg px-3 py-2"
                        />
                        <p className="text-xs text-muted-foreground">{t('settings.retryOnFailureMuteDurationHelp')}</p>
                    </label>
                </div>
            </div>
        </div>
    )
}
