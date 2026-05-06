<script lang="ts">
    import {onMount} from "svelte";
    import {generateRenderID} from "$lib/utils/id";
    import services from "$lib/api/services";
    import {getNotificationsContext} from "$lib/utils/context";
    import type {LbdUserModel} from "$lib/api/models";
    import ProfilePic from "$lib/components/ProfilePic.svelte";

    export interface Props {
        username: string;
        inputName: string;
    }

    const { addErrorNotification } = getNotificationsContext();

    let { username = $bindable(), inputName }: Props = $props();

    let suggestedUsers: LbdUserModel[] = $state([]);
    let dropdownID = $state("");

    interface Timeout {
        timeout: ReturnType<typeof setTimeout> | undefined;
        controller: AbortController;
    }

    let timeout: Timeout | undefined = $state();

    function pick(option: string) {
        suggestedUsers = [];
        username = option;
    }

    onMount(() => {
        dropdownID = generateRenderID();
        function outsideClick(e: MouseEvent) {
            if (!(e.target as HTMLElement).closest(`#${dropdownID}`)) {
               suggestedUsers = [];
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

    function onChangeText() {
        if (timeout) {
            clearTimeout(timeout.timeout);
            timeout.controller.abort();
            timeout = undefined;
        }
        const controller = new AbortController();
        timeout = {
            timeout: setTimeout(async () => {
                const [data, err] = await services.getSearchPlayers(username, undefined, undefined, controller.signal);
                if (data) {
                    suggestedUsers = data.userList ?? [];
                } else {
                    addErrorNotification(err);
                }
                timeout = undefined;
            }, 500),
            controller
        };
    }
</script>

<div class="dropdown-container">
    <input name={inputName} bind:value={username} autocomplete="off" oninput={onChangeText}/>

    {#if suggestedUsers.length > 0}
        <div class="dropdown-menu">
            {#each suggestedUsers as user}
                {@const handle = () => pick(user.username)}
                <div role="button" tabindex="0" class="dropdown-item dropdown-user" onclick={handle} onkeydown={handle}>
                    <ProfilePic userId={user.id} size={30}/>
                    <div>
                        {user.username}
                    </div>
                    <img class="flag" src="/flags/{user.country}.png" alt="" />
                </div>
            {/each}
        </div>
    {/if}
</div>

<style>
    .flag {
        position: relative;
        border-radius: 2px;
        width: auto;
        height: 13px;
    }

    .dropdown-user {
        height: 45px;
        display: flex;
        flex-direction: row;
        gap: 10px;
        width: 100%;
        align-items: center;
    }
</style>