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
	"github.com/spf13/viper"

	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	_ "github.com/chirpstack/chirpstack-fuota-server/v4/internal/migrations"
)

type C2Config struct {
	ServerURL     string
	Username      string
	Password      string
	Frequency     string
	LastSyncTime  string
	FuotaInterval int64
	SessionTime   int
	MulticastIP   string
	MulticastPort int
	Schema        string
}

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

	// if conf.PostgreSQL.Automigrate {
	// 	if err := MigrateUp(DB()); err != nil {
	// 		return err
	// 	}
	// }
	InitDB()
	return nil
}

func InitDB() {
	var c2config C2Config = GetC2ConfigFromToml()
	schema := `
    CREATE SCHEMA IF NOT EXISTS ` + c2config.Schema + ` ;

    CREATE TABLE IF NOT EXISTS ` + c2config.Schema + `.deployment (
        id UUID PRIMARY KEY NOT NULL,
        created_at TIMESTAMPTZ NOT NULL,
        updated_at TIMESTAMPTZ NOT NULL,
        mc_group_setup_completed_at TIMESTAMPTZ NULL,
        mc_session_completed_at TIMESTAMPTZ NULL,
        frag_session_setup_completed_at TIMESTAMPTZ NULL,
        enqueue_completed_at TIMESTAMPTZ NULL,
        frag_status_completed_at TIMESTAMPTZ NULL
    );

    CREATE TABLE IF NOT EXISTS ` + c2config.Schema + `.deployment_device (
        deployment_id UUID NOT NULL REFERENCES ` + c2config.Schema + `.deployment ON DELETE CASCADE,
        dev_eui BYTEA NOT NULL,
        created_at TIMESTAMPTZ NOT NULL,
        updated_at TIMESTAMPTZ NOT NULL,
        mc_group_setup_completed_at TIMESTAMPTZ NULL,
        mc_session_completed_at TIMESTAMPTZ NULL,
        frag_session_setup_completed_at TIMESTAMPTZ NULL,
        frag_status_completed_at TIMESTAMPTZ NULL,
        PRIMARY KEY (deployment_id, dev_eui)
    );

    CREATE TABLE IF NOT EXISTS ` + c2config.Schema + `.deployment_log (
        id BIGSERIAL PRIMARY KEY,
        created_at TIMESTAMPTZ NOT NULL,
        deployment_id UUID NOT NULL REFERENCES ` + c2config.Schema + `.deployment ON DELETE CASCADE,
        dev_eui BYTEA NOT NULL,
        f_port SMALLINT NOT NULL,
        command VARCHAR(50) NOT NULL,
        fields HSTORE
    );

    CREATE TABLE IF NOT EXISTS ` + c2config.Schema + `.device_profile (
        profileId BIGINT PRIMARY KEY,
        region VARCHAR(255),
        macVersion VARCHAR(255),
        regionParameter VARCHAR(255)
    );

    CREATE TABLE IF NOT EXISTS ` + c2config.Schema + `.device (
        deviceCode VARCHAR(255) PRIMARY KEY,
        modelId BIGINT,
        profileId BIGINT,
        firmwareVersion VARCHAR(255),
        status BIGINT,
        firmwareUpdateFailed BOOLEAN,
        attempts BIGINT,
        CONSTRAINT fk_device_profile FOREIGN KEY (profileId) REFERENCES ` + c2config.Schema + `.device_profile(profileId)
    );

    CREATE INDEX IF NOT EXISTS idx_deployment_log_deployment_id ON ` + c2config.Schema + `.deployment_log(deployment_id);
    CREATE INDEX IF NOT EXISTS idx_deployment_log_dev_eui ON ` + c2config.Schema + `.deployment_log(dev_eui);
    `

	// Execute the SQL commands
	_, err := db.Exec(schema)
	if err != nil {
		log.Fatalf("Failed to create tables & indexes in DB: %v", err)
	}

	log.Println("Tables and indexes created successfully")
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

func DropAll() error {
	var c2config C2Config = GetC2ConfigFromToml()

	if db == nil {
		log.Info("db is not connected")
		return nil
	}
	indexes := []string{
		"DROP INDEX IF EXISTS idx_deployment_log_dev_eui;",
		"DROP INDEX IF EXISTS idx_deployment_log_deployment_id;",
	}

	for _, query := range indexes {
		if _, err := db.Exec(query); err != nil {
			log.Fatalf("Failed to drop index: %v", err)
		}
	}
	log.Debug("Index dropped successfully.")
	// Drop tables
	tables := []string{
		`DROP TABLE IF EXISTS ` + c2config.Schema + `.deployment_log;`,
		`DROP TABLE IF EXISTS ` + c2config.Schema + `.deployment_device;`,
		`DROP TABLE IF EXISTS ` + c2config.Schema + `.deployment;`,
		`DROP TABLE IF EXISTS ` + c2config.Schema + `.device;`,
		`DROP TABLE IF EXISTS ` + c2config.Schema + `.device_profile;`,
	}

	for _, query := range tables {
		if _, err := db.Exec(query); err != nil {
			log.Fatalf("Failed to drop table: %v", err)
		}
	}
	log.Debug("Table dropped successfully.")
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

func GetC2ConfigFromToml() C2Config {

	viper.SetConfigName("c2intbootconfig")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2intbootconfig.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2intbootconfig.toml file: %v", err)
		}
	}

	var c2config C2Config

	c2config.Username = viper.GetString("c2App.username")
	if c2config.Username == "" {
		log.Fatal("username not found in c2intbootconfig.toml file")
	}

	c2config.Password = viper.GetString("c2App.password")
	if c2config.Password == "" {
		log.Fatal("password not found in c2intbootconfig.toml file")
	}

	c2config.ServerURL = viper.GetString("c2App.serverUrl")
	if c2config.ServerURL == "" {
		log.Fatal("serverUrl not found in c2intbootconfig.toml file")
	}

	c2config.Frequency = viper.GetString("c2App.frequency")
	if c2config.Frequency == "" {
		log.Fatal("frequency not found in c2intbootconfig.toml file")
	}

	c2config.FuotaInterval = viper.GetInt64("c2App.fuotainterval")
	if c2config.FuotaInterval == 0 {
		log.Fatal("fuotainterval not found in c2intbootconfig.toml file")
	}
	c2config.SessionTime = viper.GetInt("c2App.sessiontime")
	if c2config.SessionTime == 0 {
		log.Fatal("sessiontime not found in c2intbootconfig.toml file")
	}

	c2config.MulticastIP = viper.GetString("c2App.multicastip")
	if c2config.MulticastIP == "" {
		log.Fatal("multicastip not found in c2intbootconfig.toml file")
	}

	c2config.MulticastPort = viper.GetInt("c2App.multicastport")
	if c2config.MulticastPort == 0 {
		log.Fatal("multicastport not found in c2intbootconfig.toml file")
	}

	c2config.Schema = viper.GetString("database.schema")
	if c2config.Schema == "" {
		log.Fatal("schema not found in c2intbootconfig.toml file")
	}

	return c2config
}
