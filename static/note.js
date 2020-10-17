function copyToClipboard(str) {
    const el = document.createElement('textarea');
    el.value = str;
    document.body.appendChild(el);
    el.select();
    document.execCommand('copy');
    document.body.removeChild(el);
};

document.addEventListener("DOMContentLoaded", () => {
    let deleteButton = document.getElementById("delete");
    let copyButton = document.getElementById("copy");
    let noteArea = document.getElementById("note");

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

    copyButton.addEventListener("click", event => {
        event.preventDefault();
        copyToClipboard(noteArea.textContent);
        let previousContent = copyButton.textContent;
        copyButton.textContent = "Copied!";
        setTimeout(() => copyButton.textContent = previousContent, 1500);
    });
});