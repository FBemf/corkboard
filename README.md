# Corkboard

This is a little pastebin-style service which I whipped up in a couple evenings.

Specifically, the inspiration for this was that I wanted to be able to go

```sh
curl -u user:password "https://corkboard.example.com/name_of_note" --data-binary "Some information, or maybe @filename.txt"
```

and have that data all go up to `https://corkboard.example.com/name_of_note` for me to find.
Other services like this one don't let you specify the note's name, which makes it really hard if you want to (eg) get a Raspberry Pi to post its IP address so you don't have to hook up a monitor to find out how to `ssh` into it.

Here's the API:

```
GET /note/:note         Returns an HTML page containing the note named :note.
GET /api/note/:note     Returns the raw contents of the note named :note.
POST /api/note/:note    Creates a new note named :note.
                        The contents of the note are the body of the request.
PUT /api/note/:note     Creates a new note named :note, or overwrites it if it already exists.
                        The contents of the note are the body of the request.
DELETE /api/note/:note  Removes the note named :note. Returns 200 even if that note didn't exist.
```

And of course the web UI is at `/`.

Here's the help page:

```
Usage of corkboard:
  -creds string
        Access credentials in the form
        "username:password".
  -creds-file string
        Path to a file holding login credentials in the form
        "username:password". Each line holds a valid set of credentials.
  -db-path string
        Path to the sqlite db. (default "./notes.db")
  -migrate
        Run schema and all migrations upon database, then exit.
  -note-expiry int
        Notes which have not been viewed in this many days will be deleted.
        If set to zero, notes never expire. (default 7)
  -port int
        Port to serve the application on. (default 8080)
  -recent-notes int
        Display this many recent notes on the main page.
         (default 8)
```

To set up a new database or to automatically run migrations after an update, run `./corkboard -migrate`.
