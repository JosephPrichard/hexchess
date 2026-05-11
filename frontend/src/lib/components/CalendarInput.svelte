<script lang="ts">
    type InputType = 'date' | 'datetime-local' | 'month' | 'week';
    type InputState = 'default' | 'error' | 'success' | 'disabled';

    interface Props {
        type?: InputType;
        value?: string;
        label?: string;
        hint?: string;
        state?: InputState;
        min?: string;
        max?: string;
        placeholder?: string;
        onChange?: (value: string) => void;
    }

    let {
        type = 'date',
        value = $bindable(''),
        label,
        hint,
        state = 'default',
        min,
        max,
        placeholder,
        onChange,
    }: Props = $props();

    function handleChange(e: Event) {
        value = (e.target as HTMLInputElement).value;
        onChange?.(value);
    }
</script>

<div class="cal-group {state}">
    {#if label}
        <label for="cal-label" class="cal-label">{label}</label>
    {/if}
    <input
        name="cal-label"
        {type}
        {value}
        {min}
        {max}
        {placeholder}
        disabled={state === 'disabled'}
        onchange={handleChange}
    />
    {#if hint}
        <span class="cal-hint">{hint}</span>
    {/if}
</div>

<style>
    .cal-group {
        display: flex;
        flex-direction: column;
        gap: 5px;
    }

    .cal-label {
        font-size: 13px;
        color: var(--color-text-secondary, #666);
    }

    .cal-hint {
        font-size: 12px;
        color: var(--color-text-tertiary, #999);
    }

    input[type="date"],
    input[type="datetime-local"],
    input[type="month"],
    input[type="week"] {
        font-family: inherit;
        font-size: 14px;
        color: var(--color-text-primary, #111);
        background: var(--color-background-primary, #fff);
        border: 0.5px solid var(--color-border-secondary, rgba(0,0,0,0.3));
        border-radius: var(--border-radius-md, 8px);
        padding: 0 12px;
        height: 36px;
        outline: none;
        cursor: pointer;
        transition: border-color 0.15s, box-shadow 0.15s;
        min-width: 180px;
        box-sizing: border-box;
    }

    input:hover {
        border-color: var(--color-border-primary, rgba(0,0,0,0.4));
    }

    input:focus {
        border-color: var(--color-border-primary, rgba(0,0,0,0.4));
        box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-text-primary, #111) 8%, transparent);
    }

    .error input {
        border-color: var(--color-border-danger, #e24b4a);
    }
    .error input:focus {
        box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-text-danger, #e24b4a) 12%, transparent);
    }
    .error .cal-hint {
        color: var(--color-text-danger, #e24b4a);
    }

    .success input {
        border-color: var(--color-border-success, #1d9e75);
    }
    .success input:focus {
        box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-text-success, #1d9e75) 12%, transparent);
    }

    .disabled input {
        opacity: 0.4;
        cursor: not-allowed;
    }
</style>