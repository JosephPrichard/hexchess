async function renderAndAttachEventListeners() {
    const resp = await fetch("/static/initial-board", { method: "GET" });
    const initialBoard = await resp.json();


}