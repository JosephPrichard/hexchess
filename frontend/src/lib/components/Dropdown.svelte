<script lang="ts" generics="Value extends string = string">
	import { onMount } from 'svelte';
	import {generateRenderID} from "$lib/utils/id";

	interface Option<Value> {
		label: string;
		value: Value;
	}

	interface Props {
		options: Option<Value>[];
		selected: string;
		onChange?: (value: Value) => void;
	}

	let { options, selected, onChange }: Props = $props();

	let open = $state(false);
	let dropdownID = $state("");

	function pick(option: Option<Value>) {
		selected = option.value;
		open = false;
		if (onChange) onChange(option.value);
	}

	function toggle() {
		open = !open;
	}

	onMount(() => {
		dropdownID = generateRenderID();
		function outsideClick(e: MouseEvent) {
			if (!(e.target as HTMLElement).closest(`#${dropdownID}`)) {
				open = false;
			}
		}
		if (typeof window !== "undefined") {
			window.addEventListener("click", outsideClick);
		}
		return () => {
			if (typeof window !== "undefined") {
				window.removeEventListener("click", outsideClick);
			}
		}
	});
</script>

<div class="dropdown-container" id={dropdownID}>
	<button type="button" class="dropdown-selected" onclick={toggle}>
		<span class="dropdown-selected-text">{options[options.findIndex((o) => o.value === selected)].label}</span>
		<span class="dropdown-arrow">{open ? "▲" : "▼"}</span>
	</button>

	{#if open}
		<div class="dropdown-menu">
			{#each options as option}
				<button class="dropdown-item {selected === option.value ? 'active' : ''}" onclick={() => pick(option)}>
					{option.label}
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.dropdown-selected-text {
		height: 1.2rem;
	}

    .dropdown-selected {
        all: unset;
        box-sizing: border-box;
        width: 100%;
        background: rgb(64,64,64);
        padding: 0.6rem 0.8rem;
        border-radius: 6px;
        cursor: pointer;
        color: rgb(200,200,200);
        display: flex;
        justify-content: space-between;
        align-items: center;
        transition: border-color 0.2s ease;
        border: 1px solid rgb(100,100,100);
        z-index: 1;
    }

    .dropdown-selected:hover {
        border-color: rgb(100,100,100);
    }

    .dropdown-arrow {
        font-size: 0.65rem;
        opacity: 0.6;
    }
</style>
