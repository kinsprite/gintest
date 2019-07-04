
# build

on Windows:
```cmd
set GOPROXY=https://goproxy.io
set GIN_MODE=release
go build -tags=jsoniter .
```

or Linux:

```shell
export GOPROXY=https://goproxy.io
export GIN_MODE=release
go build -tags=jsoniter .
```


RUN:
```shell
# Set the service name. Allowed characters:
# a-z, A-Z, 0-9, -, _, and space.
# If ELASTIC_APM_SERVICE_NAME is not specified,
# the executable name will be used.
export ELASTIC_APM_SERVICE_NAME=


# Set custom APM Server URL
# (default: http://localhost:8200)
export ELASTIC_APM_SERVER_URL= <apm_server_url>

# Set if APM Server requires a token.
export ELASTIC_APM_SECRET_TOKEN= <apm_token>

export GIN_MODE=release
./gin-test
```
