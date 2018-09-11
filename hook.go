package logrus_cloudwatchlogs_async

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/sirupsen/logrus"
)

const (
	eventBufferSize  = 100
	sendingFrequency = 200 * time.Millisecond
)

type Hook struct {
	groupName       string
	streamName      string
	config          *aws.Config
	sequenceToken   *string
	eventChannel    chan cloudwatchlogs.InputLogEvent
	isSendingEvents bool
}

func NewHook(groupName, streamName string, config *aws.Config) (*Hook, error) {
	hook := &Hook{
		groupName:       groupName,
		streamName:      streamName,
		config:          config,
		eventChannel:    make(chan cloudwatchlogs.InputLogEvent, eventBufferSize),
		isSendingEvents: true,
	}

	service := hook.newCloudWatchService()

	resp, err := service.DescribeLogStreams(
		&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName:        aws.String(hook.groupName),
			LogStreamNamePrefix: aws.String(hook.streamName),
		})
	if err != nil { // The log group doesn't exist
		return nil, err
	}

	if len(resp.LogStreams) > 0 { // The log stream already exists
		hook.sequenceToken = resp.LogStreams[0].UploadSequenceToken
	} else { // Create new log stream
		_, err = service.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
			LogGroupName:  aws.String(hook.groupName),
			LogStreamName: aws.String(hook.streamName),
		})
		if err != nil {
			return nil, err
		}
	}

	go hook.runLoop()
	return hook, nil
}

// Implements logrus.Hook interface
func (hook *Hook) Fire(entry *logrus.Entry) error {
	message, err := entry.String()
	if err != nil {
		return err
	}

	event := cloudwatchlogs.InputLogEvent{
		Message: aws.String(message),
		Timestamp: aws.Int64( // Convert nanosecond to millisecond
			int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)),
	}
	select {
	case hook.eventChannel <- event:
	default: // The channel is full; the event is discarded to avoid blocking (very unlikely)
	}

	return nil
}

// Implements logrus.Hook interface
func (hook *Hook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

func (hook *Hook) StartSendingEvents() {
	hook.isSendingEvents = true
}

func (hook *Hook) StopSendingEvents() {
	hook.isSendingEvents = false
}

func (hook *Hook) runLoop() {
	// Avoid flooding CloudWatch with updates; send log events in batches
	tickChannel := time.Tick(sendingFrequency)
	// Collect log messages in between ticks and while the terminal is offline
	var events []*cloudwatchlogs.InputLogEvent

	for {
		select {
		case event := <-hook.eventChannel:
			events = append(events, &event)
		case <-tickChannel:
			if hook.isSendingEvents && len(events) > 0 {
				input := &cloudwatchlogs.PutLogEventsInput{
					LogEvents:     events,
					LogGroupName:  aws.String(hook.groupName),
					LogStreamName: aws.String(hook.streamName),
					SequenceToken: hook.sequenceToken,
				}
				resp, err := hook.newCloudWatchService().PutLogEvents(input)
				if err == nil {
					hook.sequenceToken = resp.NextSequenceToken
					events = nil
				}
			}
		}
	}
}

func (hook *Hook) newCloudWatchService() *cloudwatchlogs.CloudWatchLogs {
	return cloudwatchlogs.New(session.New(hook.config))
}
