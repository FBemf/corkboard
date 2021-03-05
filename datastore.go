package main

import (
	"database/sql"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

const (
	NO_CLOBBER = iota
	CREATED
	UPDATED
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

func (ds *Datastore) setNote(name string, body []byte, clobber bool) (int, error) {
	if ds.mode == flatFile {
		notePath := path.Join(ds.directory, name)
		if _, err := os.Stat(notePath); err == nil {
			if clobber {
				err := ioutil.WriteFile(notePath, body, 0755)
				return UPDATED, err
			} else {
				// return an error if you're overwriting something
				return NO_CLOBBER, nil
			}
		} else {
			err := ioutil.WriteFile(notePath, body, 0755)
			return CREATED, err
		}
	} else { //database
		_, err := ds.database.Exec(`INSERT INTO Notes (Name, Body)
			VALUES (?, ?)`, name, body)
		if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
			if clobber {
				// overwrite the body
				_, err = ds.database.Exec(`UPDATE Notes SET Body = ? WHERE Name = ?`, body, name)
				return UPDATED, err
			} else {
				// don't clobber a note
				return NO_CLOBBER, nil
			}
		}
		return CREATED, err
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
