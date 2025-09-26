# Static Container Registy

A container registry implementation that respects [OCI spec](https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md)
for Pull and Discovery. Written in Go.

## Endpoints

| Method         | API Endpoint | Success     | Failure           |
| -------------- | ------------ | ----------- | ----------------- |
| `GET`          | `/v2/`       | `200`       | `404`/`401`       |

## Environment variables

This service can be configured through environment variables:

| Variable  | Behaviour                                                        |
| --------- | ---------------------------------------------------------------- |
| LOG_LEVEL | Sets the log level for the service                               |
| HTTP_ADDR | Sets the address the service listens and serves HTTP requests on |
