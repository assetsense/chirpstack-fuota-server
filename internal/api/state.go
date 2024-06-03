package api

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

var stateFile string = "fuota_state.toml"

func LoadState() string {
	viper.SetConfigName(stateFile)
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if err := viper.ReadInConfig(); err != nil {
			switch err.(type) {
			case viper.ConfigFileNotFoundError:
			default:
				log.WithError(err).Fatal("read configuration file error")
			}
		}
	}

	state := viper.GetString("state")

	return state
}

func SaveState(state string) {
	viper.SetConfigName(stateFile)
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/usr/local/bin")

	viper.Reset()
	viper.Set("state", state)
	err := viper.WriteConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			err = viper.SafeWriteConfigAs(stateFile)
		}
		if err != nil {
			// log.Fatalf("Failed to write config file: %v", err)
		}
	}

}
