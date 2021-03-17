package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"path"
	"regexp"
	"strconv"
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
	// initialize _migration table, get latest migration
	_, err := ds.database.Exec(`create table if not exists _migration (
		date	date,
		number	number,
		primary key (date, number))`)
	if err != nil {
		return fmt.Errorf("creating _migration: %s", err)
	}
	last_migration, err := ds.database.Query(`select * from _migration order by date desc, number desc limit 1`)
	if err != nil {
		return fmt.Errorf("finding last migration: %s", err)
	}

	db_empty := true
	var last_date string
	var last_number int
	if last_migration.Next() {
		db_empty = false
		last_migration.Scan(&last_date, &last_number)
		if last_migration.Next() {
			return fmt.Errorf("'limit 1' migration query returned multiple rows")
		}
	} else {
		log.Println("creating database")
	}
	migrations_performed := 0

	// Execute only migrations which are more recent than the latest migration
	// Because walkdir traverses the directory in lexicographical order, we don't need
	// to worry about the migrations being performed in the wrong order
	err = fs.WalkDir(migrations, ".", func(filepath string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking dir: %s", err)
		}
		if d.IsDir() {
			return nil
		}

		filename := path.Base(filepath)
		match, err := regexp.MatchString(`^\d{4}-\d{2}-\d{2}\.\d+\.sql$`, filename)
		if err != nil {
			return fmt.Errorf("validating migration name: %s", err)
		}
		if !match {
			return fmt.Errorf("parsing migration name %s: bad format", filename)
		}
		name_components := strings.Split(filename, ".")
		if len(name_components) != 3 {
			return fmt.Errorf("parsing migration name %s: bad format", filename)
		}
		date := name_components[0]
		number, err := strconv.ParseInt(name_components[1], 10, 32)
		if err != nil {
			return fmt.Errorf("parsing migration name %s: number too large", filename)
		}

		if !db_empty {
			if last_date > date || (last_date == date && last_number >= int(number)) {
				return nil
			}
		}

		file, err := fs.ReadFile(migrations, filepath)
		if err != nil {
			return fmt.Errorf("reading migration file %s: %s", filepath, err)
		}
		_, err = ds.database.Exec(string(file))
		if err != nil {
			return fmt.Errorf("running migration %s: %s", filepath, err)
		}
		migrations_performed += 1
		_, err = ds.database.Exec(`insert into _migration values (?, ?)`, date, number)
		if err != nil {
			return fmt.Errorf("inserting migration into table: %s", err)
		}
		return nil
	})
	if migrations_performed > 0 {
		log.Printf("applied %d migrations", migrations_performed)
	}
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
