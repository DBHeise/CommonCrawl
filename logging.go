package webcache

import (
	"os"
	"time"

	"github.com/spf13/viper"

	"github.com/rs/zerolog"
)

// InitializeLogging - Setup the logger
func InitializeLogger(name string) zerolog.Logger {
	var log zerolog.Logger

	switch viper.GetString("Log.Output") {
	case "console":
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339, NoColor: true}).With().Timestamp().Logger()
	default:
		file, err := os.OpenFile(viper.GetString("Log.Output"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log = zerolog.New(file).With().Timestamp().Logger()
		} else {
			log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: true}).With().Timestamp().Logger()
			log.Warn().Msg("Failed to log to file, using stderr")
		}
	}

	logLevel, err := zerolog.ParseLevel(viper.GetString("Log.Level"))
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	log = log.Level(logLevel)

	if viper.GetBool("Log.ReportCaller") {
		log = log.With().Caller().Logger()
	}

	return log.With().Str("name", name).Logger()
}

