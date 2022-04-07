package main

import (
	"io/ioutil"
	"log"
	"os"

	zcw "github.com/lzap/zapcloudwatch2"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getConsoleCore() zapcore.Core {
	level := zap.NewAtomicLevelAt(zapcore.DebugLevel)
	encoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	consoleDebugging := zapcore.Lock(os.Stdout)
	return zapcore.NewCore(encoder, consoleDebugging, level)
}

func getCloudwatchCore() (*zapcore.Core, error) {

	cloudWatchParams := zcw.NewCloudwatchCoreParams{
		GroupName:    "test",
		StreamName:   "stream",
		AWSRegion:    os.Getenv("AWS_REGION"),
		AWSAccessKey: os.Getenv("AWS_ACCESS_KEY"),
		AWSSecretKey: os.Getenv("AWS_SECRET_KEY"),
		AWSToken:     os.Getenv("AWS_TOKEN"),
		Level:        zapcore.InfoLevel,
		LevelEnabler: zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Enc:          zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		Out:          zapcore.AddSync(ioutil.Discard),
	}

	core, err := zcw.NewCloudwatchCore(&cloudWatchParams)
	if err != nil {
		log.Printf("can't initialize cloudwatch logger: %v", err)
		return nil, err
	}

	return &core, nil
}

func getLogger(name string) *zap.Logger {
	consoleCore := getConsoleCore()

	cloudwatchCore, err := getCloudwatchCore()

	if err != nil {
		return zap.New(consoleCore).Named(name)
	}

	core := zapcore.NewTee(consoleCore, *cloudwatchCore)

	return zap.New(core).Named(name)
}

func main() {
	logger := getLogger("test")

	// It is very important to sync at the program exit since messages
	// are sent in batches.
	defer logger.Sync()

	logger.Debug("don't need to send a message")
	logger.Error("an error happened!")
}
