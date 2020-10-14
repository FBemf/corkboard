document.addEventListener("DOMContentLoaded", () => {
    let titleArea = document.getElementById("title");
    let bodyArea = document.getElementById("body");
    let submitButton = document.getElementById("submit");
    let statusArea = document.getElementById("status");
    statusArea.textContent = "";

    submitButton.addEventListener("click", event => {
        event.preventDefault();
        let title = titleArea.value;
        let body = bodyArea.value;
        fetch(`/note/${title}`, {
            method: "POST",
            cache: "no-cache",
            headers: {
                "Content-Type": "application/octet-stream",
            },
            redirect: "follow",
            body: body,
        }).then(resp => {
            if (resp.ok) {
                statusArea.textContent = "";
                titleArea.value = "";
                bodyArea.value = "";
            } else {
                if (resp.status == 409) {
                    statusArea.textContent = "That note already exists!";
                } else if (resp.status == 401) {
                    statusArea.textContent = "Authorization error. Try reloading the page.";
                } else {
                    statusArea.textContent = "Unknown server error";
                }
            }
        })
    });
});