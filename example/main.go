package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
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

	// Read default config from $HOME/.aws/credentials
	//cfg, err := config.LoadDefaultConfig(context.TODO())

	// Read config section from $HOME/.aws/credentials
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile("saml"), config.WithRegion("eu-central-1"))

	// Or static configuration
	/*
		prov := credentials.NewStaticCredentialsProvider(
			os.Getenv("AWS_ACCESS_KEY"),
			os.Getenv("AWS_SECRET_KEY"),
			os.Getenv("AWS_TOKEN"),
		)

		cfg, err := config.LoadDefaultConfig(
			context.Background(),
			config.WithCredentialsProvider(prov),
			config.WithRegion(os.Getenv("AWS_REGION")),
		)
	*/

	if err != nil {
		return nil, err
	}

	cloudWatchParams := zcw.NewCloudwatchCoreParams{
		GroupName:    "test",
		StreamName:   "stream",
		Config:       &cfg,
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
	logger := getLogger("test").Sugar()

	// It is very important to sync at the program exit since messages
	// are sent in batches.
	defer logger.Sync()

	logger.Debug("don't need to send a message")

	for i := 1; i <= 5; i++ {
		logger.Infof("E%d", i)
		logger.Errorf("E%d", i)
	}
}
