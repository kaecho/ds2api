import { ChevronDown, ChevronRight } from 'lucide-react'
import { useState } from 'react'

function ToggleRow({ t, label, desc, checked, onChange }) {
    return (
        <label className="flex items-start gap-3 rounded-lg border border-border bg-background/60 p-4 cursor-pointer">
            <input
                type="checkbox"
                checked={checked}
                onChange={(e) => onChange(e.target.checked)}
                className="mt-1 h-4 w-4 rounded border-border"
            />
            <div className="space-y-1">
                <span className="text-sm font-medium block">{label}</span>
                <span className="text-xs text-muted-foreground block">{desc}</span>
            </div>
        </label>
    )
}

function TextAreaWithDefault({ t, label, help, value, placeholder, onChange, visible }) {
    if (!visible) return null
    return (
        <label className="text-sm space-y-2 md:col-span-2">
            <span className="text-muted-foreground">{label}</span>
            <textarea
                rows={4}
                value={value}
                placeholder={placeholder}
                onChange={(e) => onChange(e.target.value)}
                className="w-full bg-background border border-border rounded-lg px-3 py-2 resize-y min-h-24 font-mono text-xs"
            />
            <p className="text-xs text-muted-foreground">{help}</p>
        </label>
    )
}

function TextInput({ label, value, placeholder, onChange }) {
    return (
        <label className="text-sm space-y-1">
            <span className="text-muted-foreground text-xs">{label}</span>
            <input
                type="text"
                value={value}
                placeholder={placeholder}
                onChange={(e) => onChange(e.target.value)}
                className="w-full bg-background border border-border rounded-lg px-2 py-1.5 font-mono text-xs"
            />
        </label>
    )
}

export default function PromptSection({ t, form, setForm }) {
    const [showSentinels, setShowSentinels] = useState(false)

    const updatePrompt = (key, updates) => setForm((prev) => ({
        ...prev,
        prompt: { ...prev.prompt, [key]: { ...prev.prompt[key], ...updates } },
    }))

    const updateSentinels = (key, value) => setForm((prev) => ({
        ...prev,
        prompt: {
            ...prev.prompt,
            sentinels: { ...prev.prompt.sentinels, [key]: value },
        },
    }))

    const p = form.prompt || {}

    return (
        <div className="bg-card border border-border rounded-xl p-5 space-y-4">
            <div className="space-y-1">
                <h3 className="font-semibold">{t('settings.promptTitle')}</h3>
                <p className="text-sm text-muted-foreground">{t('settings.promptDesc')}</p>
            </div>

            {/* Output Integrity Guard */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <ToggleRow
                    t={t}
                    label={t('settings.promptGuardEnabled')}
                    desc={t('settings.promptGuardDesc')}
                    checked={Boolean(p.output_integrity_guard?.enabled ?? true)}
                    onChange={(v) => updatePrompt('output_integrity_guard', { enabled: v })}
                />
                <TextAreaWithDefault
                    t={t}
                    label={t('settings.promptGuardText')}
                    help={t('settings.promptGuardHelp')}
                    value={p.output_integrity_guard?.text || ''}
                    placeholder={p.output_integrity_guard?.default_text || ''}
                    onChange={(v) => updatePrompt('output_integrity_guard', { text: v })}
                    visible={Boolean(p.output_integrity_guard?.enabled ?? true)}
                />
            </div>

            {/* Sentinels */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <ToggleRow
                    t={t}
                    label={t('settings.promptSentinelsEnabled')}
                    desc={t('settings.promptSentinelsDesc')}
                    checked={Boolean(p.sentinels?.enabled ?? true)}
                    onChange={(v) => updateSentinels('enabled', v)}
                />
                <div />
                {(p.sentinels?.enabled ?? true) && (
                    <div className="md:col-span-2 space-y-2">
                        <button
                            type="button"
                            onClick={() => setShowSentinels((v) => !v)}
                            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
                        >
                            {showSentinels ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                            {t('settings.promptSentinelsCustomize')}
                        </button>
                        {showSentinels && (
                            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2">
                                <TextInput
                                    label={t('settings.promptSentinelBegin')}
                                    value={p.sentinels?.begin_sentence || ''}
                                    placeholder="<|begin▁of▁sentence|>"
                                    onChange={(v) => updateSentinels('begin_sentence', v)}
                                />
                                <TextInput
                                    label={t('settings.promptSentinelSystem')}
                                    value={p.sentinels?.system || ''}
                                    placeholder="<|System|>"
                                    onChange={(v) => updateSentinels('system', v)}
                                />
                                <TextInput
                                    label={t('settings.promptSentinelUser')}
                                    value={p.sentinels?.user || ''}
                                    placeholder="<|User|>"
                                    onChange={(v) => updateSentinels('user', v)}
                                />
                                <TextInput
                                    label={t('settings.promptSentinelAssistant')}
                                    value={p.sentinels?.assistant || ''}
                                    placeholder="<|Assistant|>"
                                    onChange={(v) => updateSentinels('assistant', v)}
                                />
                                <TextInput
                                    label={t('settings.promptSentinelTool')}
                                    value={p.sentinels?.tool || ''}
                                    placeholder="<|Tool|>"
                                    onChange={(v) => updateSentinels('tool', v)}
                                />
                                <TextInput
                                    label={t('settings.promptSentinelEndSentence')}
                                    value={p.sentinels?.end_sentence || ''}
                                    placeholder="<|end▁of▁sentence|>"
                                    onChange={(v) => updateSentinels('end_sentence', v)}
                                />
                                <TextInput
                                    label={t('settings.promptSentinelEndToolResults')}
                                    value={p.sentinels?.end_tool_results || ''}
                                    placeholder="<|end▁of▁toolresults|>"
                                    onChange={(v) => updateSentinels('end_tool_results', v)}
                                />
                                <TextInput
                                    label={t('settings.promptSentinelEndInstructions')}
                                    value={p.sentinels?.end_instructions || ''}
                                    placeholder="<|end▁of▁instructions|>"
                                    onChange={(v) => updateSentinels('end_instructions', v)}
                                />
                            </div>
                        )}
                    </div>
                )}
            </div>

            {/* Tool Call Instructions */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <ToggleRow
                    t={t}
                    label={t('settings.promptToolInstructionsEnabled')}
                    desc={t('settings.promptToolInstructionsDesc')}
                    checked={Boolean(p.tool_call_instructions?.enabled ?? true)}
                    onChange={(v) => updatePrompt('tool_call_instructions', { enabled: v })}
                />
                <TextAreaWithDefault
                    t={t}
                    label={t('settings.promptToolInstructionsText')}
                    help={t('settings.promptToolInstructionsHelp')}
                    value={p.tool_call_instructions?.text || ''}
                    placeholder={p.tool_call_instructions?.default_text || ''}
                    onChange={(v) => updatePrompt('tool_call_instructions', { text: v })}
                    visible={Boolean(p.tool_call_instructions?.enabled ?? true)}
                />
            </div>

            {/* Read Tool Cache Guard */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <ToggleRow
                    t={t}
                    label={t('settings.promptReadCacheGuardEnabled')}
                    desc={t('settings.promptReadCacheGuardDesc')}
                    checked={Boolean(p.read_tool_cache_guard?.enabled ?? true)}
                    onChange={(v) => updatePrompt('read_tool_cache_guard', { enabled: v })}
                />
                <TextAreaWithDefault
                    t={t}
                    label={t('settings.promptReadCacheGuardText')}
                    help={t('settings.promptReadCacheGuardHelp')}
                    value={p.read_tool_cache_guard?.text || ''}
                    placeholder={p.read_tool_cache_guard?.default_text || ''}
                    onChange={(v) => updatePrompt('read_tool_cache_guard', { text: v })}
                    visible={Boolean(p.read_tool_cache_guard?.enabled ?? true)}
                />
            </div>

            {/* Empty Output Retry Suffix */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <ToggleRow
                    t={t}
                    label={t('settings.promptEmptyRetryEnabled')}
                    desc={t('settings.promptEmptyRetryDesc')}
                    checked={Boolean(p.empty_output_retry_suffix?.enabled ?? true)}
                    onChange={(v) => updatePrompt('empty_output_retry_suffix', { enabled: v })}
                />
                <TextAreaWithDefault
                    t={t}
                    label={t('settings.promptEmptyRetryText')}
                    help={t('settings.promptEmptyRetryHelp')}
                    value={p.empty_output_retry_suffix?.text || ''}
                    placeholder={p.empty_output_retry_suffix?.default_text || ''}
                    onChange={(v) => updatePrompt('empty_output_retry_suffix', { text: v })}
                    visible={Boolean(p.empty_output_retry_suffix?.enabled ?? true)}
                />
            </div>

            {/* Response Replacements */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <ToggleRow
                    t={t}
                    label={t('settings.responseReplacementsEnabled')}
                    desc={t('settings.responseReplacementsDesc')}
                    checked={Boolean(form.response_replacements?.enabled)}
                    onChange={(v) => setForm((prev) => ({
                        ...prev,
                        response_replacements: { ...prev.response_replacements, enabled: v },
                    }))}
                />
                {Boolean(form.response_replacements?.enabled) && (
                    <div className="md:col-span-2 space-y-2">
                        {(form.response_replacements?.rules || []).map((rule, idx) => (
                            <div key={idx} className="grid grid-cols-1 md:grid-cols-[1fr_1fr_auto] gap-2 items-end">
                                <TextInput
                                    label={t('settings.responseReplacementFrom')}
                                    value={rule.from || ''}
                                    placeholder="<|DEML"
                                    onChange={(v) => setForm((prev) => {
                                        const rules = [...(prev.response_replacements?.rules || [])]
                                        rules[idx] = { ...rules[idx], from: v }
                                        return { ...prev, response_replacements: { ...prev.response_replacements, rules } }
                                    })}
                                />
                                <TextInput
                                    label={t('settings.responseReplacementTo')}
                                    value={rule.to || ''}
                                    placeholder="<|DSML"
                                    onChange={(v) => setForm((prev) => {
                                        const rules = [...(prev.response_replacements?.rules || [])]
                                        rules[idx] = { ...rules[idx], to: v }
                                        return { ...prev, response_replacements: { ...prev.response_replacements, rules } }
                                    })}
                                />
                                <button
                                    type="button"
                                    onClick={() => setForm((prev) => ({
                                        ...prev,
                                        response_replacements: {
                                            ...prev.response_replacements,
                                            rules: (prev.response_replacements?.rules || []).filter((_, i) => i !== idx),
                                        },
                                    }))}
                                    className="px-3 py-2 rounded-lg border border-border text-xs hover:bg-muted"
                                >
                                    {t('settings.responseReplacementRemove')}
                                </button>
                            </div>
                        ))}
                        <button
                            type="button"
                            onClick={() => setForm((prev) => ({
                                ...prev,
                                response_replacements: {
                                    ...prev.response_replacements,
                                    rules: [...(prev.response_replacements?.rules || []), { from: '', to: '' }],
                                },
                            }))}
                            className="px-3 py-2 rounded-lg border border-border text-xs hover:bg-muted"
                        >
                            {t('settings.responseReplacementAdd')}
                        </button>
                    </div>
                )}
            </div>
        </div>
    )
}
