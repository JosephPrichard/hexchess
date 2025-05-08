<script lang="ts">
    import { onMount } from 'svelte';

    interface Props {
        targetPage: number;
        totalPages?: number;
    }

    interface Page {
        location: string;
        value: number;
    }

    const { targetPage, totalPages }: Props = $props();

    let pages: Page[] = $state([]);
    let leftPage: string | undefined = $state(undefined);
    let rightPage: string | undefined = $state(undefined);

    onMount(() => {
        function createPage(page: number) {
            const url = new URL(window.location.href);
            url.searchParams.set('page', String(page));
            return url.toString();
        }

        if (totalPages !== undefined) {
            const start = Math.max(targetPage - 2, 1);
            const end = Math.min(targetPage + 2, totalPages);
            pages = Array.from({ length: end - start + 1 }, (_, i) => ({ location: createPage(start + i), value: start + i }));

            leftPage = targetPage > 1 ? createPage(targetPage - 1) : undefined;
            rightPage = targetPage < totalPages ? createPage(targetPage + 1) : undefined;
        } else {
            pages = [{ location: createPage(targetPage), value: targetPage }];

            leftPage = targetPage > 1 ? createPage(targetPage - 1) : undefined;
            rightPage = createPage(targetPage + 1);
        }
    });
</script>

<div class="pagination">
    {#if leftPage}
        <a class="page-button" href={leftPage}> &#11164; </a>
    {/if}
    {#each pages as page (page.value)}
        <a class="page-button" href={page.location}>
            {page.value}
        </a>
    {/each}
    {#if rightPage}
        <a class="page-button" href={rightPage}> &#11166; </a>
    {/if}
</div>
