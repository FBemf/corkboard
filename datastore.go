package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"strings"
	"time"
)

const (
	NO_CLOBBER = iota
	CREATED
	UPDATED
)

type Datastore struct {
	database *sql.DB
}

func (ds *Datastore) RunMigrations(migrations fs.FS) error {
	err := fs.WalkDir(migrations, ".", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		file, err := fs.ReadFile(migrations, path)
		if err != nil {
			return err
		}
		_, err = ds.database.Exec(string(file))
		if err != nil {
			return fmt.Errorf("running migration %s: %v", path, err)
		}
		return nil
	})
	return err
}

func (ds *Datastore) Close() error {
	return ds.database.Close()
}

func (ds *Datastore) getNote(name string) ([]byte, bool, error) {
	row := ds.database.QueryRow(`select (body) from note where name = ?`, name)
	buf := []byte{}
	if err := row.Scan(&buf); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		} else {
			return nil, false, err
		}
	}
	_, err := ds.database.Exec(
		`update note set last_viewed = datetime("now") where name = ?`, name)
	if err != nil {
		return buf, true, err
	}
	return buf, true, nil
}

func (ds *Datastore) setNote(name string, body []byte, clobber bool) (int, error) {
	_, err := ds.database.Exec(`insert into note (name, body)
			values (?, ?)`, name, body)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		if clobber {
			// overwrite the body
			_, err = ds.database.Exec(`update note set body = ? where name = ?`, body, name)
			return UPDATED, err
		} else {
			// don't clobber a note
			return NO_CLOBBER, nil
		}
	}
	return CREATED, err
}

func (ds *Datastore) deleteNote(name string) error {
	_, err := ds.database.Exec("delete from note where name = ?", name)
	return err
}

// gets the `maxNotes` most recently-created notes
func (ds *Datastore) getLatestNotes(maxNotes int) ([]string, error) {
	var names = make([]string, 0)
	rows, err := ds.database.Query(
		`select (name) from note order by create_time asc limit ?`, maxNotes)
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

// deletes notes older than `age`
func (ds *Datastore) deleteOldNotes(age time.Duration) error {
	_, err := ds.database.Exec(
		`delete from note where strftime("%s", "now") - strftime("%s", last_viewed) > ?`,
		age/time.Second)
	return err
}
