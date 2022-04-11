package zapcloudwatch2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"go.uber.org/zap/zapcore"
)

// CloudwatchCore is a zap Core for dispatching messages to the specified
type CloudwatchCore struct {
	// Messages with a log level not contained in this array
	// will not be dispatched. If nil, all messages will be dispatched.
	AcceptedLevels    []zapcore.Level
	GroupName         string
	StreamName        string
	Options           *cloudwatchlogs.Options
	BatchFrequency    time.Duration
	nextSequenceToken *string
	svc               *cloudwatchlogs.Client
	m                 sync.Mutex
	ch                chan *types.InputLogEvent
	flush             chan bool
	flushWG           sync.WaitGroup
	err               *error

	zapcore.LevelEnabler
	enc zapcore.Encoder
	out zapcore.WriteSyncer
}

type NewCloudwatchCoreParams struct {
	GroupName    string
	StreamName   string
	AWSRegion    string
	AWSAccessKey string
	AWSSecretKey string
	AWSToken     string
	Level        zapcore.Level
	Enc          zapcore.Encoder
	Out          zapcore.WriteSyncer
	LevelEnabler zapcore.LevelEnabler
}

func NewCloudwatchCore(params *NewCloudwatchCoreParams) (zapcore.Core, error) {
	options := cloudwatchlogs.Options{
		Region: params.AWSRegion,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			params.AWSAccessKey,
			params.AWSSecretKey,
			params.AWSToken)),
	}

	core := &CloudwatchCore{
		GroupName:      params.GroupName,
		StreamName:     params.StreamName,
		Options:        &options,
		AcceptedLevels: LevelThreshold(params.Level),
		LevelEnabler:   params.LevelEnabler,
		enc:            params.Enc,
		out:            params.Out,
	}

	err := core.cloudWatchInit()
	if err != nil {
		return nil, err
	}

	return core, nil
}

func (c *CloudwatchCore) With(fields []zapcore.Field) zapcore.Core {
	clone := c.clone()
	addFields(clone.enc, fields)
	return clone
}

func (c *CloudwatchCore) clone() *CloudwatchCore {
	return &CloudwatchCore{
		AcceptedLevels: c.AcceptedLevels,
		GroupName:      c.GroupName,
		StreamName:     c.StreamName,
		Options:        c.Options,
		BatchFrequency: c.BatchFrequency,
		LevelEnabler:   c.LevelEnabler,
		enc:            c.enc.Clone(),
		out:            c.out,
	}
}

func addFields(enc zapcore.ObjectEncoder, fields []zapcore.Field) {
	for i := range fields {
		fields[i].AddTo(enc)
	}
}

func (c *CloudwatchCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *CloudwatchCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	if !c.isAcceptedLevel(ent.Level) {
		return nil
	}

	buf, err := c.enc.EncodeEntry(ent, fields)
	if err != nil {
		return err
	}
	defer buf.Free()

	event := types.InputLogEvent{
		Message:   aws.String(buf.String()),
		Timestamp: aws.Int64(int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)),
	}

	c.ch <- &event
	if c.err != nil {
		lastErr := c.err
		c.err = nil
		return fmt.Errorf("%v", lastErr)
	}

	return nil
}

func (c *CloudwatchCore) Sync() error {
	c.flushWG.Add(1)
	c.flush <- true
	c.flushWG.Wait()
	if c.err != nil {
		return *c.err
	}
	return nil
}

// GetHook function returns hook to zap
func (c *CloudwatchCore) cloudWatchInit() error {
	c.svc = cloudwatchlogs.New(*c.Options)

	lgresp, err := c.svc.DescribeLogGroups(context.TODO(), &cloudwatchlogs.DescribeLogGroupsInput{LogGroupNamePrefix: aws.String(c.GroupName), Limit: aws.Int32(1)})
	if err != nil {
		return err
	}

	if len(lgresp.LogGroups) < 1 {
		// we need to create this log group
		_, err := c.svc.CreateLogGroup(context.TODO(), &cloudwatchlogs.CreateLogGroupInput{LogGroupName: aws.String(c.GroupName)})
		if err != nil {
			return err
		}
	}

	resp, err := c.svc.DescribeLogStreams(context.TODO(), &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(c.GroupName), // Required
		LogStreamNamePrefix: aws.String(c.StreamName),
	})
	if err != nil {
		return err
	}

	if len(resp.LogStreams) > 0 {
		c.nextSequenceToken = resp.LogStreams[0].UploadSequenceToken
	} else {
		_, err = c.svc.CreateLogStream(context.TODO(), &cloudwatchlogs.CreateLogStreamInput{
			LogGroupName:  aws.String(c.GroupName),
			LogStreamName: aws.String(c.StreamName),
		})

		if err != nil {
			return err
		}
	}

	c.ch = make(chan *types.InputLogEvent, 10000)
	c.flush = make(chan bool)
	if c.BatchFrequency == 0 || c.BatchFrequency < 200*time.Millisecond {
		c.BatchFrequency = 2 * time.Second
	}
	ticker := time.NewTicker(c.BatchFrequency)
	go c.processBatches(c.flush, ticker.C)

	return nil
}

func (c *CloudwatchCore) processBatches(flush <-chan bool, ticker <-chan time.Time) {
	var batch []types.InputLogEvent
	size := 0
	for {
		select {
		case p := <-c.ch:
			messageSize := len(*p.Message) + 26
			if size+messageSize >= 1_048_576 || len(batch) == 10000 {
				c.sendBatch(batch)
				batch = nil
				size = 0
			}
			batch = append(batch, *p)
			size += messageSize
		case <-flush:
			c.sendBatch(batch)
			c.flushWG.Done()
			batch = nil
			size = 0
		case <-ticker:
			c.sendBatch(batch)
			batch = nil
			size = 0
		}
	}
}

func (c *CloudwatchCore) sendBatch(batch []types.InputLogEvent) {
	if len(batch) == 0 {
		return
	}
	params := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     batch,
		LogGroupName:  aws.String(c.GroupName),
		LogStreamName: aws.String(c.StreamName),
		SequenceToken: c.nextSequenceToken,
	}
	resp, err := c.svc.PutLogEvents(context.TODO(), params)
	if err == nil {
		c.nextSequenceToken = resp.NextSequenceToken
		return
	}

	c.err = &err
	if aerr, ok := err.(*types.InvalidSequenceTokenException); ok {
		c.nextSequenceToken = aerr.ExpectedSequenceToken
		c.sendBatch(batch)
		return
	}
}

// Levels sets which levels to sent to cloudwatch
func (c *CloudwatchCore) Levels() []zapcore.Level {
	if c.AcceptedLevels == nil {
		return AllLevels
	}
	return c.AcceptedLevels
}

func (c *CloudwatchCore) isAcceptedLevel(level zapcore.Level) bool {
	for _, lv := range c.Levels() {
		if lv == level {
			return true
		}
	}
	return false
}

// AllLevels Supported log levels
var AllLevels = []zapcore.Level{
	zapcore.DebugLevel,
	zapcore.InfoLevel,
	zapcore.WarnLevel,
	zapcore.ErrorLevel,
	zapcore.FatalLevel,
	zapcore.PanicLevel,
}

// LevelThreshold - Returns every logging level above and including the given parameter.
func LevelThreshold(l zapcore.Level) []zapcore.Level {
	for i := range AllLevels {
		if AllLevels[i] == l {
			return AllLevels[i:]
		}
	}
	return []zapcore.Level{}
}
