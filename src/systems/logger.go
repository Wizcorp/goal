package systems

import (
	"fmt"
	"os"

	"github.com/go-errors/errors"
	"github.com/sirupsen/logrus"

	. "github.com/Wizcorp/goal/src/api"
)

type GoalLogger interface {
	GoalSystem
	GetInstance() *logrus.Logger
}

type logger struct {
	Instance *logrus.Logger
}

type LogFields = logrus.Fields

func NewLogger() *logger {
	return &logger{
		Instance: logrus.New(),
	}
}

func (logger *logger) Setup(server GoalServer, config *GoalConfig) error {
	instance := logger.Instance

	forceColors := os.Getenv("COLORS") == "true"
	format := config.String("format", "text")
	reportCaller := config.Bool("reportCaller", false)
	level, err := getConfigLevel(config)

	if err != nil {
		return errors.Wrap(err, 0)
	}

	if format == "json" {
		instance.SetFormatter(&logrus.JSONFormatter{})
	} else {
		instance.SetFormatter(&logrus.TextFormatter{
			ForceColors: forceColors,
		})
	}

	instance.SetLevel(level)
	instance.SetReportCaller(reportCaller)

	instance.WithFields(LogFields{
		"format":      format,
		"forceColors": forceColors,
	}).Info("Logger system set")

	if format == "text" {
		instance.Debug("                        ___")
		instance.Debug("    o__        o__     |   |\\")
		instance.Debug("   /|          /\\      |   |X\\")
		instance.Debug("   / > o        <\\     |   |XX\\")
		instance.Debug("                       GOAL//NG")
	}

	return nil
}

func (logger *logger) Teardown(server GoalServer, config *GoalConfig) error {
	logger.Instance.Info("Tearing down logger system")

	return nil
}

func (logger *logger) GetStatus() Status {
	return UpStatus
}

func (logger *logger) GetInstance() *logrus.Logger {
	return logger.Instance
}

func getConfigLevel(config *GoalConfig) (logrus.Level, error) {
	level := config.String("level", "info")

	switch level {
	case "trace":
		return logrus.TraceLevel, nil
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warn":
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	case "fatal":
		return logrus.FatalLevel, nil
	case "panic":
		return logrus.PanicLevel, nil
	}

	err := fmt.Sprintf("unknown log level %s", level)
	return logrus.TraceLevel, errors.Wrap(err, 0)
}
