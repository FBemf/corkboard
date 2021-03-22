package main

import (
	"context"
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

type migration struct {
	date   string
	number int
}

func (m1 migration) before(m2 migration) bool {
	return m1.date < m2.date || (m1.date == m2.date && m1.number < m2.number)
}

func (ds *Datastore) RunMigrations(migrations fs.FS) error {
	_, err := ds.database.Exec(`pragma foreign_keys = on`)
	if err != nil {
		return fmt.Errorf("applying pragma: %s", err)
	}
	// initialize _migration table
	_, err = ds.database.Exec(`create table if not exists _migration (
		date	text,
		number	number,
		primary key (date, number))`)
	if err != nil {
		return fmt.Errorf("creating _migration: %s", err)
	}

	// stores list of migrations & whether they've been applied
	migrationSet := make(map[migration]bool)

	// find date & number of lastest migration
	// if there have been no migrations (incuding original schema), this DB is new
	rows, err := ds.database.Query(`select * from _migration`)
	if err != nil {
		return fmt.Errorf("finding migrations: %s", err)
	}

	justBegun := true
	latestMigration := migration{}
	for rows.Next() {
		var date string
		var number int
		err = rows.Scan(&date, &number)
		if err != nil {
			return fmt.Errorf("reading row of migrations: %s", err)
		}
		migrationSet[migration{date, number}] = false
		if justBegun {
			latestMigration = migration{date, number}
			justBegun = false
		} else {
			if latestMigration.before(migration{date, number}) {
				latestMigration = migration{date, number}
			}
		}
	}
	if len(migrationSet) == 0 {
		log.Println("creating database")
	}
	migrationsPerformed := 0

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

		// parse & validate migration name
		filename := path.Base(filepath)
		match, err := regexp.MatchString(`^\d{4}-\d{2}-\d{2}\.\d+\.sql$`, filename)
		if err != nil {
			return fmt.Errorf("validating migration name: %s", err)
		}
		if !match {
			return fmt.Errorf("parsing migration name %s: bad format", filename)
		}
		nameComponents := strings.Split(filename, ".")
		if len(nameComponents) != 3 {
			return fmt.Errorf("parsing migration name %s: bad format", filename)
		}
		date := nameComponents[0]
		number64, err := strconv.ParseInt(nameComponents[1], 10, 32)
		if err != nil {
			return fmt.Errorf("parsing migration name %s: number too large", filename)
		}
		number := int(number64)
		newMigration := migration{date, number}

		// if migration is already in db
		isRun, exists := migrationSet[newMigration]
		if exists {
			if isRun {
				return fmt.Errorf("duplicate migration: %s.%d has already been visited", newMigration.date, newMigration.number)
			}
			if latestMigration.before(newMigration) {
				// this migration is somehow out of order
				return fmt.Errorf("migrations are out of order: old migration %s.%d is newer than latest migration %s.%d", newMigration.date, newMigration.number, latestMigration.date, latestMigration.number)
			}
			// skip old migration
			migrationSet[newMigration] = true
			return nil

		} else {
			// this is a new migration

			if !latestMigration.before(newMigration) {
				return fmt.Errorf("migrations are out of order: new migration %s.%d is no newer than than latest migration %s.%d", newMigration.date, newMigration.number, latestMigration.date, latestMigration.number)
			}
			// this is our new latest migration; continue as normal
			migrationSet[newMigration] = true
			latestMigration = newMigration
		}

		// do migration
		file, err := fs.ReadFile(migrations, filepath)
		if err != nil {
			return fmt.Errorf("reading migration file %s: %s", filepath, err)
		}
		ctx, stop := context.WithCancel(context.Background())
		tx, err := ds.database.BeginTx(ctx, nil)
		if err != nil {
			stop()
			return fmt.Errorf("beginning transaction: %s", err)
		}
		_, err = tx.Exec(string(file))
		if err != nil {
			stop()
			return fmt.Errorf("running migration %s: %s", filepath, err)
		}
		migrationsPerformed += 1
		_, err = tx.Exec(`insert into _migration values (?, ?)`, date, number)
		if err != nil {
			stop()
			return fmt.Errorf("inserting migration into table: %s", err)
		}
		err = tx.Commit()
		if err != nil {
			stop()
			return fmt.Errorf("committing migration %s: %s", filepath, err)
		}
		stop()
		return nil
	})

	for m, done := range migrationSet {
		if !done {
			return fmt.Errorf("missing migration: migration %s.%d was never visited", m.date, m.number)
		}
	}
	if migrationsPerformed > 0 {
		log.Printf("applied %d migrations", migrationsPerformed)
	}
	return err
}

func (ds *Datastore) Close() error {
	return ds.database.Close()
}

func (ds *Datastore) getNote(name string) ([]byte, bool, error) {
	row := ds.database.QueryRow(`select (body) from "note" where name = ?`, name)
	buf := []byte{}
	if err := row.Scan(&buf); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		} else {
			return nil, false, err
		}
	}
	_, err := ds.database.Exec(
		`update "note" set last_viewed = datetime("now") where name = ?`, name)
	if err != nil {
		return buf, true, err
	}
	return buf, true, nil
}

func (ds *Datastore) setNote(name string, body []byte, clobber bool) (int, error) {
	_, err := ds.database.Exec(`insert into "note" (name, body)
			values (?, ?)`, name, body)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		if clobber {
			// overwrite the body
			_, err = ds.database.Exec(`update "note" set body = ? where name = ?`, body, name)
			return UPDATED, err
		} else {
			// don't clobber a note
			return NO_CLOBBER, nil
		}
	}
	return CREATED, err
}

func (ds *Datastore) deleteNote(name string) error {
	_, err := ds.database.Exec(`delete from "note" where name = ?`, name)
	return err
}

// gets the `maxNotes` most recently-created notes
func (ds *Datastore) getLatestNotes(maxNotes int) ([]string, error) {
	var names = make([]string, 0)
	rows, err := ds.database.Query(
		`select (name) from "note" order by create_time asc limit ?`, maxNotes)
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
		`delete from "note" where strftime("%s", "now") - strftime("%s", last_viewed) > ?`,
		age/time.Second)
	return err
}
