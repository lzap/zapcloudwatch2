# Cloudwatch core for zap

Amazon AWS Cloudwatch core for [zap](https://github.com/uber-go/zap) logging library. Sends logs in batches using the official AWS Go SDK v2.
The batch frequency is configurable and defaults to 2 seconds.
[AWS API limits](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html) apply:

* The maximum batch size is 1 MiB or 10,000 events (the code takes care of this).
* Maximum 5 requests per second per log stream (thus frequency cannot be higher than every 200ms).
* Not older than 2 weeks or the retention period.
* Not more than 2 hours in the future (get your system time and zone right).
* Events in a batch must not span more than 24 hours.

## Example

All you need is AWS region, key secret, token and frequency (> 200ms), then use zap logging library as usual. One important thing is to call `Sync` before your program exists otherwise buffered events will not be sent. You can use `defer` in the `main` function to ensure this even when program panics.

See this [example](example/main.go)

## Install

```
$ go get -u github.com/lzap/zapcloudwatch2
```

## Wait

Yes my nick is and has always been `lzap` and the logging library is `zap`. This github organization name has nothing to do with `zap` or Uber, it is just my personal account.

## Authors

* [Lukáš Zapletal](https://lukas.zapletalovi.com), updated to AWS SDK v2, simplified API
* [Victor Lellis](https://github.com/vmlellis/zapcloudwatchcore): improved version, no batching, AWS SDK v1
* [Bahadır Bozdağ](https://github.com/bahadirbb/zapcloudwatch): the original code, no batching
