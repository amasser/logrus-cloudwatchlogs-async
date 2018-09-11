# Asynchronous CloudWatch Logs hook for Logrus

Send [Logrus](https://github.com/sirupsen/logrus) logs to Amazon's [CloudWatch
Logs](https://aws.amazon.com/cloudwatch/details/#log-monitoring) service.

Inspired by [logrus-cloudwatchlogs](https://github/kdar/logrus-cloudwatchlogs), but it sends logs
asynchronously, therefore it doesn't impair application performance. It can also handle the host
temporarily going offline.

## Example

```
package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/szemate/logrus-cloudwatchlogs-async"
)

func main() {
	key := os.Getenv("AWS_ACCESS_KEY")
	secret := os.Getenv("AWS_SECRET_KEY")
	group := os.Getenv("AWS_CLOUDWATCHLOGS_GROUP_NAME")
	stream := os.Getenv("AWS_CLOUDWATCHLOGS_STREAM_NAME")

	cred := credentials.NewStaticCredentials(key, secret, "")
	cfg := aws.NewConfig().WithRegion("us-east-1").WithCredentials(cred)

	logger := logrus.New()

	hook, err := logrus_cloudwatchlogs_async.NewHook(group, stream, cfg)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Hooks.Add(hook)

	logger.Fatal("Some fatal event")
}
```
