<script lang="ts">
    import Banner from '$lib/components/Banner.svelte';
    import Pagination from '$lib/components/Pagination.svelte';
    import StatsList from '$lib/components/StatsList.svelte';
    import type { UserView } from '$lib/models';

    export interface LeaderboardProps {
        page: number;
        pageCount: number;
        userList: UserView[];
    }

    const { data }: { data: LeaderboardProps } = $props();
    const { page, pageCount, userList } = data;
</script>

<svelte:head>
    <title>Leaderboard - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
    <div class="title-lg">Leaderboard</div>
    <div class="wrapper">
        <StatsList {userList} />
    </div>
</div>
<Pagination targetPage={page} totalPages={pageCount} />
