# Cloudwatch core for zap

Amazon AWS Cloudwatch core for zap logging library. Sends logs in batches using the official AWS Go SDK v2.

The batch frequency is configurable and defaults to 2 seconds.
[AWS API limits](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html) apply:

* The maximum batch size is 1 MiB or 10,000 events.
* Maximum 5 requests per second per log stream (thus frequency cannot be higher than every 200ms).
* Not older than 2 weeks or the retention period.
* Not more than 2 hours in the future (get your system time and zone right).
* Events in a batch must not span more than 24 hours.

## Example

See an [example](example/main.go)

## Install

```
$ go get -u github.com/lzap/zapcloudwatch2
```

## Authors

* [Lukáš Zapletal](https://lukas.zapletalovi.com), updated to AWS SDK v2, simplified API
* [Victor Lellis](https://github.com/vmlellis/zapcloudwatchcore): improved version, no batching, AWS SDK v1
* [Bahadır Bozdağ](https://github.com/bahadirbb/zapcloudwatch): the original code, no batching
