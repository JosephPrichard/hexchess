async function getHistories(userId, afterId) {
    const url = `/partials/player-history?userId=${userId}&afterId=${afterId}`;
    try {
        const resp = await fetch(url, {method: 'GET'});
        return [await resp.text(), resp.ok];
    } catch (ex) {
        console.error(ex);
        return ["An unexpected error has occurred", false];
    }
}

export function handleScrollHistories(userId) {
    let hasMoreRecords = true;
    return () => {
        const table = document.getElementById('history-table-tbody');
        const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight;
        if (!hasMoreRecords || !isAtPageBottom) {
            return;
        }
        const lastId = table.lastElementChild.getAttribute("data-id");
        getHistories(userId, lastId)
            .then(([html, ok]) => {
                if (ok) {
                    table.insertAdjacentHTML('beforeend', html);
                } else {
                    hasMoreRecords = false;
                }
            });
    };
}