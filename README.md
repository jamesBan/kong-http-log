# kong http日志

记录 [kong](http://konghq.com/ "With a Title")请求日志，日志[结构](https://docs.konghq.com/hub/kong-inc/http-log) 。

## 使用

```shell
#go build
#./kong-http-log
Usage of kong-http-log:
  -address string
        listen ip (default "127.0.0.1:9513")
  -log_path string
        log path (default "/var/log/kong-log")
  -worker_num int
        worker_num (default 2)
```





配置kong

