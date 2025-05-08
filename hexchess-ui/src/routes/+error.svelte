<script lang="ts">
    import Banner from '$lib/components/Banner.svelte';
    import { page } from '$app/state';
</script>

<svelte:head>
    <title>Error {page.status} - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-container">
    <div class="panel" style="width: 550px; height: 550px; text-align: center">
        <div class="text-xlg">
            Error {page.status}
        </div>
        <p class="text-xsm">
            {page.error?.message || ''} Return to the <a class="highlight" href="/">homepage</a>?
        </p>
        <img alt="Error" src="/static/images/error-image.jpg" style="margin: 20px; width: 500px; height: auto;" />
    </div>
</div>
