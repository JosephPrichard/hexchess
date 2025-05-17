<script lang="ts">
	export function createPage(page: number) {
		const url = new URL(window.location.href);
		url.searchParams.set('page', String(page));
		return url.toString();
	}

	export function createNumeratedPages(targetPage: number, totalPages: number) {
		const pages: Record<number, string> = {};
		for (let i = Math.max(targetPage - 2, 1); i <= Math.min(targetPage + 2, totalPages); i++) {
			pages[i] = createPage(i);
		}
		if (totalPages > 0) {
			// if there is at least one page
			pages[1] = createPage(1); // we can always visit the first page
			pages[totalPages] = createPage(totalPages); // and we can always visit the last page
		}
		return pages;
	}

	export function createLeftPage(targetPage: number) {
		return targetPage > 1 ? createPage(targetPage - 1) : undefined;
	}

	export function createNumeratedRightPage(targetPage: number, totalPages: number) {
		return targetPage < totalPages ? createPage(targetPage + 1) : undefined;
	}

	interface Props {
		targetPage: number;
		totalPages?: number;
	}

	const { targetPage, totalPages }: Props = $props();

	interface Pagination {
		pages: Record<number, string>;
		leftPage: string | undefined;
		rightPage: string | undefined;
	}

	let pagination: Pagination = $state({ pages: {}, leftPage: undefined, rightPage: undefined });

	$effect(() => {
		if (totalPages !== undefined) {
			pagination = {
				pages: createNumeratedPages(targetPage, totalPages),
				leftPage: createLeftPage(targetPage),
				rightPage: createNumeratedRightPage(targetPage, totalPages)
			};
		} else {
			pagination = {
				pages: { [targetPage]: createPage(targetPage) },
				leftPage: createLeftPage(targetPage),
				rightPage: createPage(targetPage + 1)
			};
		}
	});
</script>

<div class="pagination">
	{#if pagination.leftPage}
		<a class="page-button" href={pagination.leftPage}> &#11164; </a>
	{/if}
	{#each Object.entries(pagination.pages) as [page, location] (page)}
		<!-- Returns numeric keys in ascending order -->
		<a class="page-button" href={location}>
			{page}
		</a>
	{/each}
	{#if pagination.rightPage}
		<a class="page-button" href={pagination.rightPage}> &#11166; </a>
	{/if}
</div>
