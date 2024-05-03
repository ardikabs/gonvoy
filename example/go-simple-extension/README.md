# example/go-simple-extension

this example creates simple go extension to enable Envoy HTTP filter, this filter could do HTTP header modification during request and response, while also emit request counter with prometheus tags.

## try it

### run envoy instance with the go plugin

```bash
$ git clone https://github.com/ardikabs/go-envoy
$ cd go-envoy
$ docker-compose up --build
```

### request header modified

```bash
$ curl 127.0.0.1:10000/headers

```

### generate http request counter

```bash
$ curl 127.0.0.1:10000/index.html
$ curl 127.0.0.1:10000/index.html
$ curl 127.0.0.1:10000/index.html

$ curl -sSfL 127.0.0.1:8001/stats/prometheus | grep go_http_plugin
# TYPE envoy_go_http_plugin counter
envoy_go_http_plugin{host="127.0.0.1",status_code="200",method="GET"} 3
```

