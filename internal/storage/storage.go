package storage

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rakyll/statik/fs"
	log "github.com/sirupsen/logrus"

	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	_ "github.com/chirpstack/chirpstack-fuota-server/v4/internal/migrations"
)

var (
	db *sqlx.DB
)

// DB returns the DB instance.
func DB() *sqlx.DB {
	return db
}

// Setup configures the storage package.
func Setup(conf *config.Config) error {
	log.Info("storage: connecting to PostgreSQL database")
	d, err := sqlx.Open("postgres", conf.PostgreSQL.DSN)
	if err != nil {
		return fmt.Errorf("open postgresql connection error: %w", err)
	}
	d.SetMaxOpenConns(conf.PostgreSQL.MaxOpenConnections)
	d.SetMaxIdleConns(conf.PostgreSQL.MaxIdleConnections)
	// for {
	// 	if err := d.Ping(); err != nil {
	// 		log.WithError(err).Warning("storage: ping PostgreSQL database error, will retry in 2s")
	// 		time.Sleep(time.Second * 2)
	// 	} else {
	// 		break
	// 	}
	// }
	if err := d.Ping(); err != nil {
		log.WithError(err).Warning("storage: ping PostgreSQL database error")
		return err
	}

	db = d

	if conf.PostgreSQL.Automigrate {
		if err := MigrateUp(DB()); err != nil {
			return err
		}
	}

	return nil
}

func Reset() error {
	if db == nil {
		return nil
	}
	if err := MigrateDown(DB()); err != nil {
		return err
	}
	db.Close()
	return nil
}

func CloseConn() error {
	if db == nil {
		return nil
	}
	if err := db.Close(); err != nil {
		return err
	}

	return nil
}

func DropAll(db *sqlx.DB) error {
	indexes := []string{
		"DROP INDEX IF EXISTS idx_deployment_log_dev_eui;",
		"DROP INDEX IF EXISTS idx_deployment_log_deployment_id;",
	}

	for _, query := range indexes {
		if _, err := db.Exec(query); err != nil {
			log.Fatalf("Failed to drop index: %v", err)
		} else {
			fmt.Println("Index dropped successfully.")
		}
	}

	// Drop tables
	tables := []string{
		"DROP TABLE IF EXISTS chirpstack.deployment_log;",
		"DROP TABLE IF EXISTS chirpstack.deployment_device;",
		"DROP TABLE IF EXISTS chirpstack.deployment;",
	}

	for _, query := range tables {
		if _, err := db.Exec(query); err != nil {
			log.Fatalf("Failed to drop table: %v", err)
		} else {
			fmt.Println("Table dropped successfully.")
		}
	}
	return nil
}

func MigrateUp(db *sqlx.DB) error {
	log.Info("storage: applying PostgreSQL schema migrations")

	statikFS, err := fs.New()
	if err != nil {
		return fmt.Errorf("statik fs error: %w", err)
	}

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate postgres driver error: %w", err)
	}

	src, err := httpfs.New(statikFS, "/")
	if err != nil {
		return fmt.Errorf("new httpfs error: %w", err)
	}

	m, err := migrate.NewWithInstance("httpfs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("new migrate instance error: %w", err)
	}

	oldVersion, _, _ := m.Version()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up error: %w", err)
	}

	newVersion, _, _ := m.Version()

	if oldVersion != newVersion {
		log.WithFields(log.Fields{
			"from_version": oldVersion,
			"to_version":   newVersion,
		}).Info("storage: applied database migrations")
	}

	return nil
}

func MigrateDown(db *sqlx.DB) error {
	log.Info("storage: reverting PostgreSQL schema migrations")

	statikFS, err := fs.New()
	if err != nil {
		return fmt.Errorf("statik fs error: %w", err)
	}

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate postgres driver error: %w", err)
	}

	src, err := httpfs.New(statikFS, "/")
	if err != nil {
		return fmt.Errorf("new httpfs error: %w", err)
	}

	m, err := migrate.NewWithInstance("httpfs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("new migrate instance error: %w", err)
	}

	oldVersion, _, _ := m.Version()

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate down error: %w", err)
	}

	newVersion, _, _ := m.Version()

	if oldVersion != newVersion {
		log.WithFields(log.Fields{
			"from_version": oldVersion,
			"to_version":   newVersion,
		}).Info("storage: applied database migrations")
	}

	return nil
}

func MigrateDrop(db *sqlx.DB) error {
	if db == nil {
		return nil
	}
	log.Info("storage: dropping PostgreSQL schema migrations")

	statikFS, err := fs.New()
	if err != nil {
		return fmt.Errorf("statik fs error: %w", err)
	}

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate postgres driver error: %w", err)
	}

	src, err := httpfs.New(statikFS, "/")
	if err != nil {
		return fmt.Errorf("new httpfs error: %w", err)
	}

	m, err := migrate.NewWithInstance("httpfs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("new migrate instance error: %w", err)
	}

	if err := m.Drop(); err != nil {
		return fmt.Errorf("migrate drop error: %w", err)
	}
	log.Info("Database reset is successfull")
	return nil
}

// Transaction wraps the given function in a transaction. In case the given
// functions returns an error, the transaction will be rolled back.
func Transaction(f func(tx sqlx.Ext) error) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("storage: begin transaction error: %w", err)
	}

	err = f(tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("storage: transaction rollback error: %w", err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: stransaction commit error: %w", err)
	}
	return nil
}
