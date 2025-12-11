package utils

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func init() {
	Log = logrus.New()
	Log.SetFormatter(&logrus.JSONFormatter{})
	Log.SetOutput(os.Stdout)
	Log.SetLevel(logrus.InfoLevel)
}

// Configure sets up the logger based on environment variables.
// It should be called after loading .env.
func Configure() {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	logLevel := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))

	switch {
	case logLevel != "":
		setLogLevelFromString(logLevel)
	case env == "production" || env == "prod":
		Log.SetLevel(logrus.WarnLevel)
	case env == "development" || env == "dev":
		Log.SetLevel(logrus.InfoLevel)
		Log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
			ForceColors:   true,
		})
	case env == "debug":
		Log.SetLevel(logrus.DebugLevel)
		Log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
			ForceColors:   true,
		})
	default:
		Log.SetLevel(logrus.InfoLevel)
	}

	Log.WithFields(logrus.Fields{
		"app_env":   env,
		"log_level": Log.GetLevel().String(),
	}).Debug("logger configured")
}

func setLogLevelFromString(level string) {
	switch level {
	case "debug":
		Log.SetLevel(logrus.DebugLevel)
	case "info":
		Log.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		Log.SetLevel(logrus.WarnLevel)
	case "error":
		Log.SetLevel(logrus.ErrorLevel)
	case "fatal":
		Log.SetLevel(logrus.FatalLevel)
	case "panic":
		Log.SetLevel(logrus.PanicLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
		Log.Warnf("Unknown log level '%s', defaulting to info", level)
	}
}
