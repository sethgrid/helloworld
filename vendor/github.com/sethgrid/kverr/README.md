# KVErr

Package `kverr` provides for key:value pair error values that can be bubbled up from lower level function calls to allow structured key:value paired logging at higher levels.

Any number of nested calls may be wrapped with an additional `kverr.New()`, and any additional key:value pairs will be added onto any existing error context and bubbled up.


## Reasoning

When aggregating and analyzing errors at any kind of scale, dynamic error strings can hide error trends. For example, compare these two styles of error logs.

Structured logs:
```
{"event": "error", "message": "unable to connect to remote host", "host": "service.example.com", "node_id": 12,  "user_id": 42}
...
{"event": "error", "message": "some other error", "some other key", "some other value", "node_id": 12,  "user_id": 42}
{"event": "error", "message": "unable to connect to remote host", "host": "service.example.com", "node_id": 32, "user_id": 180}
```

vs a dynamic log string:
```
[error] unable to connect to service.example.com on node 12
...
[error] some other error doing some other key with some other vlaue on node 12 for user 42
[error] unable to connect to service.example.com on node 32
```

When doing log analysis, the second set of logs require parsing to find relations. With structured logging, we can see the cardinality of `node_id` and know that 12 is seeing more errors. We can get the cardidnality of errors themsevles, and know that "unable to connect to remote host" is our most prolific error. We can be sure to always include required data, such as user ids in log lines. Structured logging enables log insights.

To get structured logging at all layers in our application, we can either pass a logger down that will allow for structured logging, or we can pass errors up. With `kverr`, we pass errors up. 


## Example Usage

```
func skipLevel() error {
	if err := returnKVErr(); err != nil {
		return fmt.Errorf("oh noes, err: %w", err)
	}
	return nil
}

func returnKVErr() error {
	return New(fmt.Errorf("root error"), "kv_present", true)
}

func doThingWithLogs() error {
    err := skipLevel()

    // args will be a list of kv pairs ["kv_present", true]
    args := kverr.YoinkArgs(err) 
    logger.Error("had an error", args...)

    // could also copy the map:
    m := kverr.Map(err)
    logger.Error("unexpected internal state", "got_kv_error", m["kv_present"])
}


```
