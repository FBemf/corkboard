document.addEventListener("DOMContentLoaded", () => {
    let titleArea = document.getElementById("title");
    let bodyArea = document.getElementById("body");
    let passwordArea = document.getElementById("password");
    let submitButton = document.getElementById("submit");
    let statusArea = document.getElementById("status");

    /*  
    passwordBox.addEventListener("keyup", (event) => {
      if (event.keyCode === 13) {
        event.preventDefault();
        submitButton.click();
      }
    });
    */

    submitButton.addEventListener("click", event => {
        event.preventDefault();
        let title = titleArea.value;
        let body = bodyArea.value;
        let password = passwordArea.value;
        fetch(`/note/${title}`, {
            method: "POST",
            cache: "no-cache",
            headers: {
                "Content-Type": "application/json"
            },
            redirect: "follow",
            body: JSON.stringify({
                body: body,
                password: password
            })
        }).then(resp => {
            if (resp.ok) {
                statusArea.textContent = "Posted!";
                titleArea.value = "";
                bodyArea.value = "";
                passwordArea.value = "";
            } else {
                statusArea.textContent = "Error: Not Authorized";
            }
        })
    });
});