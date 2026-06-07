export default function ClientSection({ t, form, setForm }) {
    return (
        <div className="bg-card border border-border rounded-xl p-5 space-y-4">
            <div className="space-y-1">
                <h3 className="font-semibold">{t('settings.clientTitle')}</h3>
                <p className="text-sm text-muted-foreground">{t('settings.clientDesc')}</p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.clientName')}</span>
                    <input
                        type="text"
                        value={form.client?.name || ''}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            client: { ...prev.client, name: e.target.value },
                        }))}
                        placeholder="DeepSeek"
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.clientPlatform')}</span>
                    <input
                        type="text"
                        value={form.client?.platform || ''}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            client: { ...prev.client, platform: e.target.value },
                        }))}
                        placeholder="android"
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.clientVersion')}</span>
                    <input
                        type="text"
                        value={form.client?.version || ''}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            client: { ...prev.client, version: e.target.value },
                        }))}
                        placeholder="2.1.0"
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.clientAndroidAPILevel')}</span>
                    <input
                        type="text"
                        value={form.client?.android_api_level || ''}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            client: { ...prev.client, android_api_level: e.target.value },
                        }))}
                        placeholder="30"
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.clientLocale')}</span>
                    <input
                        type="text"
                        value={form.client?.locale || ''}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            client: { ...prev.client, locale: e.target.value },
                        }))}
                        placeholder="zh_CN"
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
            </div>
            <label className="text-sm space-y-2 block">
                <span className="text-muted-foreground">{t('settings.clientBaseHeaders')}</span>
                <textarea
                    rows={4}
                    value={form.client?.base_headers_text || '{}'}
                    onChange={(e) => setForm((prev) => ({
                        ...prev,
                        client: { ...prev.client, base_headers_text: e.target.value },
                    }))}
                    placeholder='{}'
                    className="w-full bg-background border border-border rounded-lg px-3 py-2 font-mono text-xs"
                />
            </label>
        </div>
    )
}
