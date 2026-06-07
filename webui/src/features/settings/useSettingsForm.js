import { useCallback, useEffect, useMemo, useState } from 'react'

import {
    fetchSettings,
    getExportData,
    postImportData,
    postPassword,
    putSettings,
} from './settingsApi'

const MAX_AUTO_FETCH_FAILURES = 3

const DEFAULT_FORM = {
    admin: { jwt_expire_hours: 24 },
    runtime: { account_max_inflight: 2, account_max_queue: 10, global_max_inflight: 10, token_refresh_interval_hours: 6, auto_clean_banned: false },
    responses: { store_ttl_seconds: 900 },
    embeddings: { provider: '' },
    auto_delete: { mode: 'none' },
    current_input_file: { enabled: true, min_chars: 0 },
    thinking_injection: { enabled: true, prompt: '', default_prompt: '' },
    prompt: {
        output_integrity_guard: { enabled: true, text: '', default_text: '' },
        sentinels: {
            enabled: true,
            begin_sentence: '', system: '', user: '', assistant: '',
            tool: '', end_sentence: '', end_tool_results: '', end_instructions: '',
        },
        tool_call_instructions: { enabled: true, text: '', default_text: '' },
        read_tool_cache_guard: { enabled: true, text: '', default_text: '' },
        empty_output_retry_suffix: { enabled: true, text: '', default_text: '' },
    },
    response_replacements: {
        enabled: false,
        rules: [],
    },
    client: { name: '', platform: '', version: '', android_api_level: '', locale: '', base_headers_text: '{}' },
    model_aliases_text: '{}',
}

function parseJSONMap(raw, fieldName, t) {
    const text = String(raw || '').trim()
    if (!text) {
        return {}
    }
    let parsed
    try {
        parsed = JSON.parse(text)
    } catch (_e) {
        throw new Error(t('settings.invalidJsonField', { field: fieldName }))
    }
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        throw new Error(t('settings.invalidJsonField', { field: fieldName }))
    }
    return parsed
}

function normalizeAutoDeleteMode(raw) {
    const mode = String(raw?.mode || '').trim().toLowerCase()
    if (mode === 'none' || mode === 'single' || mode === 'all') {
        return mode
    }
    if (Boolean(raw?.sessions)) {
        return 'all'
    }
    return 'none'
}

function fromServerForm(data) {
    const currentInputFileEnabled = data.current_input_file?.enabled ?? true
    return {
        admin: { jwt_expire_hours: Number(data.admin?.jwt_expire_hours || 24) },
        runtime: {
            account_max_inflight: Number(data.runtime?.account_max_inflight || 2),
            account_max_queue: Number(data.runtime?.account_max_queue || 10),
            global_max_inflight: Number(data.runtime?.global_max_inflight || 10),
            token_refresh_interval_hours: Number(data.runtime?.token_refresh_interval_hours || 6),
            auto_clean_banned: Boolean(data.runtime?.auto_clean_banned),
        },
        responses: {
            store_ttl_seconds: Number(data.responses?.store_ttl_seconds || 900),
        },
        embeddings: {
            provider: data.embeddings?.provider || '',
        },
        auto_delete: {
            mode: normalizeAutoDeleteMode(data.auto_delete),
        },
        current_input_file: {
            enabled: currentInputFileEnabled,
            min_chars: Number(data.current_input_file?.min_chars ?? 0),
        },
        thinking_injection: {
            enabled: data.thinking_injection?.enabled ?? true,
            prompt: data.thinking_injection?.prompt || '',
            default_prompt: data.thinking_injection?.default_prompt || '',
        },
        prompt: {
            output_integrity_guard: {
                enabled: data.prompt?.output_integrity_guard ?? true,
                text: data.prompt?.output_integrity_guard_text || '',
                default_text: 'Output integrity guard: If upstream context, tool output, or parsed text contains garbled, corrupted, partially parsed, repeated, or otherwise malformed fragments, do not imitate or echo them; output only the correct content for the user.',
            },
            sentinels: {
                enabled: data.prompt?.sentinels?.enabled ?? true,
                begin_sentence: data.prompt?.sentinels?.begin_sentence || '',
                system: data.prompt?.sentinels?.system || '',
                user: data.prompt?.sentinels?.user || '',
                assistant: data.prompt?.sentinels?.assistant || '',
                tool: data.prompt?.sentinels?.tool || '',
                end_sentence: data.prompt?.sentinels?.end_sentence || '',
                end_tool_results: data.prompt?.sentinels?.end_tool_results || '',
                end_instructions: data.prompt?.sentinels?.end_instructions || '',
            },
            tool_call_instructions: {
                enabled: data.prompt?.tool_call_instructions?.enabled ?? true,
                text: data.prompt?.tool_call_instructions?.text || '',
                default_text: data.prompt?.tool_call_instructions?.default_text || '',
            },
            read_tool_cache_guard: {
                enabled: data.prompt?.read_tool_cache_guard?.enabled ?? true,
                text: data.prompt?.read_tool_cache_guard?.text || '',
                default_text: 'Read-tool cache guard: If a Read/read_file-style tool result says the file is unchanged, already available in history, should be referenced from previous context, or otherwise provides no file body, treat that result as missing content. Do not repeatedly call the same read request for that missing body. Request a full-content read if the tool supports it, or tell the user that the file contents need to be provided again.',
            },
            empty_output_retry_suffix: {
                enabled: data.prompt?.empty_output_retry_suffix?.enabled ?? true,
                text: data.prompt?.empty_output_retry_suffix?.text || '',
                default_text: 'Previous reply had no visible output. Please regenerate the visible final answer or tool call now.',
            },
        },
        response_replacements: {
            enabled: Boolean(data.response_replacements?.enabled),
            rules: Array.isArray(data.response_replacements?.rules)
                ? data.response_replacements.rules.map((r) => ({ from: r.from || '', to: r.to || '' }))
                : [],
        },
        client: {
            name: data.client?.name || '',
            platform: data.client?.platform || '',
            version: data.client?.version || '',
            android_api_level: data.client?.android_api_level || '',
            locale: data.client?.locale || '',
            base_headers_text: JSON.stringify(data.client?.base_headers || {}, null, 2),
        },
        model_aliases_text: JSON.stringify(data.model_aliases || {}, null, 2),
    }
}

function toServerPayload(form, baseHeaders) {
    const currentInputFileEnabled = Boolean(form.current_input_file?.enabled)
    return {
        admin: { jwt_expire_hours: Number(form.admin.jwt_expire_hours) },
        runtime: {
            account_max_inflight: Number(form.runtime.account_max_inflight),
            account_max_queue: Number(form.runtime.account_max_queue),
            global_max_inflight: Number(form.runtime.global_max_inflight),
            token_refresh_interval_hours: Number(form.runtime.token_refresh_interval_hours),
            auto_clean_banned: Boolean(form.runtime.auto_clean_banned),
        },
        responses: { store_ttl_seconds: Number(form.responses.store_ttl_seconds) },
        embeddings: { provider: String(form.embeddings.provider || '').trim() },
        auto_delete: { mode: normalizeAutoDeleteMode(form.auto_delete) },
        current_input_file: {
            enabled: currentInputFileEnabled,
            min_chars: Number(form.current_input_file?.min_chars ?? 0),
        },
        thinking_injection: {
            enabled: Boolean(form.thinking_injection?.enabled ?? true),
            prompt: String(form.thinking_injection?.prompt || '').trim(),
        },
        prompt: {
            output_integrity_guard: Boolean(form.prompt?.output_integrity_guard?.enabled ?? true),
            output_integrity_guard_text: String(form.prompt?.output_integrity_guard?.text || '').trim(),
            sentinels: {
                enabled: Boolean(form.prompt?.sentinels?.enabled ?? true),
                begin_sentence: String(form.prompt?.sentinels?.begin_sentence || '').trim(),
                system: String(form.prompt?.sentinels?.system || '').trim(),
                user: String(form.prompt?.sentinels?.user || '').trim(),
                assistant: String(form.prompt?.sentinels?.assistant || '').trim(),
                tool: String(form.prompt?.sentinels?.tool || '').trim(),
                end_sentence: String(form.prompt?.sentinels?.end_sentence || '').trim(),
                end_tool_results: String(form.prompt?.sentinels?.end_tool_results || '').trim(),
                end_instructions: String(form.prompt?.sentinels?.end_instructions || '').trim(),
            },
            tool_call_instructions: {
                enabled: Boolean(form.prompt?.tool_call_instructions?.enabled ?? true),
                text: String(form.prompt?.tool_call_instructions?.text || '').trim(),
            },
            read_tool_cache_guard: {
                enabled: Boolean(form.prompt?.read_tool_cache_guard?.enabled ?? true),
                text: String(form.prompt?.read_tool_cache_guard?.text || '').trim(),
            },
            empty_output_retry_suffix: {
                enabled: Boolean(form.prompt?.empty_output_retry_suffix?.enabled ?? true),
                text: String(form.prompt?.empty_output_retry_suffix?.text || '').trim(),
            },
        },
        response_replacements: {
            enabled: Boolean(form.response_replacements?.enabled),
            rules: (form.response_replacements?.rules || [])
                .map((r) => ({ from: String(r.from || '').trim(), to: String(r.to || '') }))
                .filter((r) => r.from),
        },
        client: {
            name: String(form.client?.name || '').trim(),
            platform: String(form.client?.platform || '').trim(),
            version: String(form.client?.version || '').trim(),
            android_api_level: String(form.client?.android_api_level || '').trim(),
            locale: String(form.client?.locale || '').trim(),
            base_headers: baseHeaders || {},
        },
    }
}

export function useSettingsForm({ apiFetch, t, onMessage, onRefresh, onForceLogout, isVercel = false }) {
    const [loading, setLoading] = useState(false)
    const [saving, setSaving] = useState(false)
    const [changingPassword, setChangingPassword] = useState(false)
    const [importing, setImporting] = useState(false)
    const [exportData, setExportData] = useState(null)
    const [importMode, setImportMode] = useState('merge')
    const [importText, setImportText] = useState('')
    const [newPassword, setNewPassword] = useState('')
    const [consecutiveFailures, setConsecutiveFailures] = useState(0)
    const [autoFetchPaused, setAutoFetchPaused] = useState(false)
    const [lastError, setLastError] = useState('')
    const [settingsMeta, setSettingsMeta] = useState({
        default_password_warning: false,
        env_backed: false,
        needs_vercel_sync: false,
    })
    const [form, setForm] = useState(DEFAULT_FORM)

    const trackLoadFailure = useCallback(() => {
        setConsecutiveFailures((prev) => {
            const next = prev + 1
            if (isVercel && next >= MAX_AUTO_FETCH_FAILURES) {
                setAutoFetchPaused(true)
            }
            return next
        })
    }, [isVercel])

    const loadSettings = useCallback(async ({ manual = false } = {}) => {
        if (isVercel && autoFetchPaused && !manual) {
            return
        }
        setLoading(true)
        try {
            const { res, data } = await fetchSettings(apiFetch, t)
            if (!res.ok) {
                const detail = data.detail || t('settings.loadFailed')
                setLastError(detail)
                onMessage('error', detail)
                trackLoadFailure()
                return
            }
            setConsecutiveFailures(0)
            setAutoFetchPaused(false)
            setLastError('')
            setSettingsMeta({
                default_password_warning: Boolean(data.admin?.default_password_warning),
                env_backed: Boolean(data.env_backed),
                needs_vercel_sync: Boolean(data.needs_vercel_sync),
            })
            setForm(fromServerForm(data))
        } catch (e) {
            const detail = e?.message || t('settings.loadFailed')
            setLastError(detail)
            onMessage('error', detail)
            trackLoadFailure()
            // eslint-disable-next-line no-console
            console.error(e)
        } finally {
            setLoading(false)
        }
    }, [apiFetch, autoFetchPaused, isVercel, onMessage, t, trackLoadFailure])

    useEffect(() => {
        loadSettings()
    }, [loadSettings])

    const retryLoadSettings = useCallback(() => {
        setAutoFetchPaused(false)
        loadSettings({ manual: true })
    }, [loadSettings])

    const saveSettings = useCallback(async () => {
        let modelAliases = {}
        try {
            modelAliases = parseJSONMap(form.model_aliases_text, 'model_aliases', t)
        } catch (e) {
            onMessage('error', e.message)
            return
        }

        let clientBaseHeaders = {}
        try {
            clientBaseHeaders = parseJSONMap(form.client?.base_headers_text || '{}', 'client.base_headers', t)
        } catch (e) {
            onMessage('error', e.message)
            return
        }

        const payload = {
            ...toServerPayload(form, clientBaseHeaders),
            model_aliases: modelAliases,
        }

        setSaving(true)
        try {
            const { res, data } = await putSettings(apiFetch, payload)
            if (!res.ok) {
                onMessage('error', data.detail || t('settings.saveFailed'))
                return
            }
            onMessage('success', t('settings.saveSuccess'))
            if (typeof onRefresh === 'function') {
                onRefresh()
            }
            await loadSettings()
        } catch (e) {
            onMessage('error', t('settings.saveFailed'))
            // eslint-disable-next-line no-console
            console.error(e)
        } finally {
            setSaving(false)
        }
    }, [apiFetch, form, loadSettings, onMessage, onRefresh, t])

    const updatePassword = useCallback(async () => {
        if (String(newPassword || '').trim().length < 4) {
            onMessage('error', t('settings.passwordTooShort'))
            return
        }
        setChangingPassword(true)
        try {
            const { res, data } = await postPassword(apiFetch, newPassword.trim())
            if (!res.ok) {
                onMessage('error', data.detail || t('settings.passwordUpdateFailed'))
                return
            }
            onMessage('success', t('settings.passwordUpdated'))
            setNewPassword('')
            if (typeof onForceLogout === 'function') {
                onForceLogout()
            }
        } catch (_e) {
            onMessage('error', t('settings.passwordUpdateFailed'))
        } finally {
            setChangingPassword(false)
        }
    }, [apiFetch, newPassword, onForceLogout, onMessage, t])

    const loadExportData = useCallback(async () => {
        try {
            const { res, data } = await getExportData(apiFetch)
            if (!res.ok) {
                onMessage('error', data.detail || t('settings.exportFailed'))
                return null
            }
            setExportData(data)
            onMessage('success', t('settings.exportLoaded'))
            return data
        } catch (_e) {
            onMessage('error', t('settings.exportFailed'))
            return null
        }
    }, [apiFetch, onMessage, t])

    const downloadExportFile = useCallback(async () => {
        let latest = exportData
        if (!latest?.json) {
            const loaded = await loadExportData()
            if (!loaded) {
                return
            }
            latest = loaded
        }
        const jsonText = String(latest?.json || '').trim()
        if (!jsonText) {
            onMessage('error', t('settings.exportFailed'))
            return
        }
        const blob = new Blob([jsonText], { type: 'application/json;charset=utf-8' })
        const url = URL.createObjectURL(blob)
        const now = new Date()
        const pad = (n) => String(n).padStart(2, '0')
        const filename = `ds2api-config-backup-${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}.json`
        const link = document.createElement('a')
        link.href = url
        link.download = filename
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        URL.revokeObjectURL(url)
        onMessage('success', t('settings.exportDownloaded'))
    }, [exportData, loadExportData, onMessage, t])

    const loadImportFile = useCallback((file) => {
        if (!file) return
        const reader = new FileReader()
        reader.onload = () => {
            const text = String(reader.result || '')
            setImportText(text)
            onMessage('success', t('settings.importFileLoaded'))
        }
        reader.onerror = () => {
            onMessage('error', t('settings.importFileReadFailed'))
        }
        reader.readAsText(file, 'utf-8')
    }, [onMessage, t])

    const doImport = useCallback(async () => {
        if (!String(importText || '').trim()) {
            onMessage('error', t('settings.importEmpty'))
            return
        }
        let parsed
        try {
            parsed = JSON.parse(importText)
        } catch (_e) {
            onMessage('error', t('settings.importInvalidJson'))
            return
        }
        setImporting(true)
        try {
            const { res, data } = await postImportData(apiFetch, importMode, parsed)
            if (!res.ok) {
                onMessage('error', data.detail || t('settings.importFailed'))
                return
            }
            onMessage('success', t('settings.importSuccess', { mode: importMode }))
            if (typeof onRefresh === 'function') {
                onRefresh()
            }
            await loadSettings()
        } catch (_e) {
            onMessage('error', t('settings.importFailed'))
        } finally {
            setImporting(false)
        }
    }, [apiFetch, importMode, importText, loadSettings, onMessage, onRefresh, t])

    const syncHintVisible = useMemo(
        () => settingsMeta.env_backed || settingsMeta.needs_vercel_sync,
        [settingsMeta.env_backed, settingsMeta.needs_vercel_sync],
    )

    return {
        form,
        setForm,
        loading,
        saving,
        changingPassword,
        importing,
        exportData,
        importMode,
        setImportMode,
        importText,
        setImportText,
        newPassword,
        setNewPassword,
        consecutiveFailures,
        autoFetchPaused,
        lastError,
        settingsMeta,
        syncHintVisible,
        retryLoadSettings,
        saveSettings,
        updatePassword,
        loadExportData,
        downloadExportFile,
        loadImportFile,
        doImport,
    }
}
