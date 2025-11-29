<script lang="ts">
  import { onMount } from "svelte";

  interface Option {
      label: string;
      value: string;
  }

  interface Props {
      options: Option[];
      selected: string;
      onChange?: (value: string) => void;
  }

  let { options, selected, onChange }: Props = $props();

  let open = $state(false);

  function pick(option: Option) {
      selected = option.value;
      open = false;
      if (onChange) onChange(option.value);
  }

  function toggle() {
      open = !open;
  }

  function outsideClick(e: MouseEvent) {
      if (!(e.target as HTMLElement).closest("#dropdown")) {
          open = false;
      }
  }

  onMount(() => {
      if (typeof window !== "undefined") {
          window.addEventListener("click", outsideClick);
      }
  });
</script>

<div class="dropdown-container" id="dropdown">
  <button class="dropdown-selected" onclick={toggle}>
      {options[options.findIndex((o) => o.value === selected)].label}
      <span class="dropdown-arrow">{open ? "▲" : "▼"}</span>
  </button>

  {#if open}
      <div class="dropdown-menu">
          {#each options as option}
            <button
              class="dropdown-item {selected === option.value ? 'active' : ''}"
              onclick={() => pick(option)}
            >
                {option.label}
          </button>
          {/each}
      </div>
  {/if}
</div>

<style>
  .dropdown-container {
    width: 100%;
    z-index: 1000;
    position: relative;
    /* width: 200px; */
    font-size: 0.95rem;
  }

  .dropdown-selected {
    all: unset;
    box-sizing: border-box;
    width: 100%;
    background: rgb(64,64,64);
    border: 1px solid rgb(100,100,100);
    padding: 0.6rem 0.8rem;
    border-radius: 6px;
    cursor: pointer;
    color: rgb(200,200,200);
    display: flex;
    justify-content: space-between;
    align-items: center;
    transition: border-color 0.2s ease;
  }

  .dropdown-selected:hover {
    border-color: rgb(100,100,100);
  }

  .dropdown-arrow {
    font-size: 0.75rem;
    opacity: 0.6;
  }

  .dropdown-menu {
    all: unset;
    width: 100%;
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    background: rgb(64,64,64);
    border: 1px solid rgb(100,100,100);
    border-radius: 6px;
    padding: 4px 0;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    z-index: 50;
  }

  .dropdown-item {
    all: unset;
    width: 100%;
    box-sizing: border-box;
    display: block;
    padding: 0.5rem 0.8rem;
    cursor: pointer;
    color: rgb(200,200,200);
    border-radius: 4px;
  }

  .dropdown-item:hover {
    background: rgb(104,104,104);
  }

  .dropdown-item.active {
    background: rgb(84,84,84);
    font-weight: 600;
  }
</style>
