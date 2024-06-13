package cmd

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type C2Config struct {
	ServerURL       string
	Username        string
	Password        string
	Frequency       string
	LastSyncTime    string
	ChirpstackHost  string
	ChirpStackPort  string
	ChirpstackToken string
	DbUsername      string
	DbName          string
}

var (
	cfgFile string
	version string
)

var rootCmd = &cobra.Command{
	Use:   "chirpstack-fuota-server",
	Short: "ChirpStack FUOTA Server",
	RunE:  run,
}

func init() {
	cobra.OnInitialize(initConfig)

	var c2config C2Config = getC2ConfigFromToml()

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "path to configuration file (optional)")
	rootCmd.PersistentFlags().Int("log-level", 4, "debug=5, info=4, error=2, fatal=1, panic=0")

	viper.BindPFlag("general.log_level", rootCmd.PersistentFlags().Lookup("log-level"))

	// default values
	// viper.SetDefault("postgresql.dsn", "postgres://localhost/chirpstack_fuota?sslmode=disable")
	viper.SetDefault("postgresql.dsn", "postgres://"+c2config.DbUsername+":"+c2config.Password+"@"+c2config.ChirpstackHost+"/"+c2config.DbName+"?sslmode=disable")
	viper.SetDefault("postgresql.automigrate", true)
	viper.SetDefault("postgresql.max_idle_connections", 2)

	viper.SetDefault("chirpstack.event_handler.marshaler", "protobuf")
	viper.SetDefault("chirpstack.event_handler.http.bind", "0.0.0.0:8090")
	viper.SetDefault("chirpstack.api.server", c2config.ChirpstackHost+":"+c2config.ChirpStackPort)
	// viper.SetDefault("chirpstack.api.token", "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJhdWQiOiJjaGlycHN0YWNrIiwiaXNzIjoiY2hpcnBzdGFjayIsInN1YiI6IjMyMjA4NDczLWNkZjktNGUzYi05M2JjLTBjOTRkMTQ4ZDI0NiIsInR5cCI6ImtleSJ9.STkoqRjpHFz2kDtyd09BqNrh8mHk4RojismyXbygTv8")
	viper.SetDefault("chirpstack.api.token", c2config.ChirpstackToken)
	viper.SetDefault("chirpstack.api.tls_enabled", false)
	viper.SetDefault("fuota_server.api.bind", "0.0.0.0:8070")

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute executes the root command.
func Execute(v string) error {
	version = v

	if err := rootCmd.Execute(); err != nil {
		// log.Fatal(err)
		return err
	}
	return nil
}

func initConfig() {
	config.Version = version

	if cfgFile != "" {
		b, err := ioutil.ReadFile(cfgFile)
		if err != nil {
			log.WithError(err).WithField("config", cfgFile).Fatal("error loading config file")
		}
		viper.SetConfigType("toml")
		if err := viper.ReadConfig(bytes.NewBuffer(b)); err != nil {
			log.WithError(err).WithField("config", cfgFile).Fatal("error loading config file")
		}
	} else {
		// viper.SetConfigName("chirpstack-fuota-server")
		viper.SetConfigName("mgfuotaserver.toml")
		viper.AddConfigPath(".")
		// viper.AddConfigPath("$HOME/.config/chirpstack-fuota-server")
		// viper.AddConfigPath("/etc/chirpstack-fuota-server")
		viper.AddConfigPath("/usr/local/bin")
		if err := viper.ReadInConfig(); err != nil {
			switch err.(type) {
			case viper.ConfigFileNotFoundError:
			default:
				log.WithError(err).Fatal("read configuration file error")
			}
		}
	}

	viperBindEnvs(config.C)

	viperHooks := mapstructure.ComposeDecodeHookFunc(
		viperDecodeJSONSlice,
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	)

	if err := viper.Unmarshal(&config.C, viper.DecodeHook(viperHooks)); err != nil {
		log.WithError(err).Fatal("unmarshal config error")
	}
}

func viperBindEnvs(iface interface{}, parts ...string) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		t := ift.Field(i)
		tv, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			tv = strings.ToLower(t.Name)
		}
		if tv == "-" {
			continue
		}

		switch v.Kind() {
		case reflect.Struct:
			viperBindEnvs(v.Interface(), append(parts, tv)...)
		default:
			// Bash doesn't allow env variable names with a dot so
			// bind the double underscore version.
			keyDot := strings.Join(append(parts, tv), ".")
			keyUnderscore := strings.Join(append(parts, tv), "__")
			viper.BindEnv(keyDot, strings.ToUpper(keyUnderscore))
		}
	}
}

func viperDecodeJSONSlice(rf reflect.Kind, rt reflect.Kind, data interface{}) (interface{}, error) {
	// input must be a string and destination must be a slice
	if rf != reflect.String || rt != reflect.Slice {
		return data, nil
	}

	raw := data.(string)

	// this decoder expects a JSON list
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return data, nil
	}

	var out []map[string]interface{}
	err := json.Unmarshal([]byte(raw), &out)

	return out, err
}

func getC2ConfigFromToml() C2Config {

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

	c2config.ChirpstackToken = viper.GetString("chirpstack.bearer_token")
	if c2config.ChirpstackToken == "" {
		log.Fatal("Bearer token not found in c2intbootconfig.toml file")
	}

	c2config.ChirpstackHost = viper.GetString("chirpstack.host")
	if c2config.ChirpstackHost == "" {
		log.Fatal("host not found in c2intbootconfig.toml file")
	}

	c2config.ChirpStackPort = viper.GetString("chirpstack.port")
	if c2config.ChirpStackPort == "" {
		log.Fatal("port not found in c2intbootconfig.toml file")
	}

	c2config.DbUsername = viper.GetString("database.username")
	if c2config.Username == "" {
		log.Fatal("dbusername not found in c2intbootconfig.toml file")
	}

	c2config.DbName = viper.GetString("database.dbname")
	if c2config.DbName == "" {
		log.Fatal("dbname not found in c2intbootconfig.toml file")
	}

	return c2config
}
