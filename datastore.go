package main

import (
	"database/sql"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

type Datastore struct {
	mode      int     // database or flatFile
	database  *sql.DB // nil if mode == flatFile
	directory string  // "" if mode == database
}

func (ds *Datastore) getNote(name string) ([]byte, bool, error) {
	if ds.mode == flatFile {
		notePath := path.Join(ds.directory, name)
		data, err := ioutil.ReadFile(notePath)
		return data, true, err
	} else { // database
		row := ds.database.QueryRow(`SELECT (Body) FROM Notes WHERE Name = ?`, name)
		buf := []byte{}
		if err := row.Scan(&buf); err != nil {
			if err == sql.ErrNoRows {
				return nil, false, nil
			} else {
				return nil, false, err
			}
		}
		_, err := ds.database.Exec(
			`UPDATE Notes SET LastViewed = datetime("now") WHERE Name = ?`, name)
		if err != nil {
			return buf, true, err
		}
		return buf, true, nil
	}
}

func (ds *Datastore) createNote(name string, body []byte) (bool, error) {
	if ds.mode == flatFile {
		notePath := path.Join(ds.directory, name)
		if _, err := os.Stat(notePath); err == nil {
			return false, nil
		}
		err := ioutil.WriteFile(notePath, body, 0755)
		return true, err
	} else { //database
		_, err := ds.database.Exec(`INSERT INTO Notes (Name, Body)
			VALUES (?, ?)`, name, body)
		if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return false, nil
		}
		return true, err
	}
}

func (ds *Datastore) deleteNote(name string) error {
	if ds.mode == flatFile {
		notePath := path.Join(ds.directory, name)
		err := os.Remove(notePath)
		return err
	} else { //database
		_, err := ds.database.Exec("DELETE FROM Notes WHERE Name = ?", name)
		return err
	}
}

// gets the `maxNotes` most recently-created notes
func (ds *Datastore) getLatestNotes(maxNotes int) ([]string, error) {
	var names = make([]string, 0)
	if ds.mode == database {
		rows, err := ds.database.Query(
			"SELECT (Name) FROM Notes ORDER BY CreateTime ASC LIMIT ?", maxNotes)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			err := rows.Scan(&name)
			if err != nil {
				return names, err
			}
			names = append(names, name)
		}
		err = rows.Err()
		return names, err
	}
	return names, nil
}

// deletes notes older than `age`
func (ds *Datastore) deleteOldNotes(age time.Duration) error {
	if ds.mode == database {
		_, err := ds.database.Exec(
			`DELETE FROM Notes WHERE strftime("%s", "now") - strftime("%s", LastViewed) > ?`,
			age/time.Second)
		return err
	}
	return nil
}
