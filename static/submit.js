document.addEventListener("DOMContentLoaded", () => {
    let titleArea = document.getElementById("title");
    let bodyArea = document.getElementById("body");
    let submitButton = document.getElementById("submit");
    let statusArea = document.getElementById("status");
    let tokenArea = document.getElementById("authToken");
    let token = tokenArea.textContent;
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
                "Authentication": `token ${token}`
                //"X-Password": '"' + password + '"',
            },
            redirect: "follow",
            body: body,
        }).then(resp => {
            if (resp.ok) {
                statusArea.textContent = "Posted!";
                titleArea.value = "";
                bodyArea.value = "";
            } else {
                statusArea.textContent = "Error: Session expired; reload the page";
            }
        })
    });
});