document.addEventListener("DOMContentLoaded", () => {
    let deleteButton = document.getElementById("delete");

    deleteButton.addEventListener("click", event => {
        event.preventDefault();
        if (window.confirm("Are you sure you want to delete this note?")) {
            fetch(window.location.href, {
                method: "DELETE",
                cache: "no-cache",
                redirect: "follow",
            }).then(resp => {
                if (resp.ok) {
                    window.location = "/"
                }
            });
        }
    });
});