# example/go-simple-extension

this example creates simple go extension to enable Envoy HTTP filter, this filter could do HTTP header and body manipulation during request and response, while also emit request counter with prometheus tags.

## try it

### run envoy instance with the go plugin

```bash
git clone https://github.com/ardikabs/gaetway
cd gaetway
docker-compose up --build
```

### request header modified

```bash
$ curl 127.0.0.1:10000/index.html

Request served by 37204d3136b5

HTTP/1.1 GET /index.html

Host: 127.0.0.1:10000
Accept: */*
Boo: far                                // <--- added by filter
User-Agent: curl/7.88.1
X-Envoy-Expected-Rq-Timeout-Ms: 15000
X-Forwarded-Proto: http
X-Key-Id: 0                             // <--- added by filter
X-Key-Id: 1                             // <--- added by filter
X-Request-Id: d009761d-1eba-4d28-a313-646aba685cf6
X-User-Id: 0                            // <--- added by filter
X-User-Id: 1                            // <--- added by filter
```

### request body modified

```bash
$ curl 127.0.0.1:10000/index.html -X POST --data-raw '{"name": "John Doe"}'

Request served by 37204d3136b5

HTTP/1.1 POST /index.html

Host: 127.0.0.1:10000
Accept: */*
Boo: far
Content-Length: 92
Content-Type: application/x-www-form-urlencoded
User-Agent: curl/7.88.1
X-Envoy-Expected-Rq-Timeout-Ms: 15000
X-Forwarded-Proto: http
X-Key-Id: 0
X-Key-Id: 1
X-Request-Id: bf0530af-2af2-4104-82e9-51f59df89495
X-User-Id: 0
X-User-Id: 1

{
    "handlerName":"HandlerThree",           // <--- added by filter
    "name":"John Doe",
    "newData":"newValue",                   // <--- added by filter
    "phase":"HTTPRequest"                   // <--- added by filter
}
```

### generate http request counter

```bash
$ curl 127.0.0.1:10000/index.html
$ curl 127.0.0.1:10000/index.html
$ curl 127.0.0.1:10000/index.html

$ curl -sSfL 127.0.0.1:8001/stats/prometheus | grep mystats
# TYPE envoy_mystats_requests_total counter
envoy_mystats_requests_total{host="127.0.0.1",status_code="200",method="get"} 3
```
