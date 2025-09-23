# Design Static Registry Server

## Goal
The goal of the Static Registry is to provide an OCI registry that follows the Pull and Discovery
use cases of the [OCI distribution spec](https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#endpoints).

This registry should serve using TLS the images stored in a specified filesystem path (`<solutions>`) that
follows this hierarchy:
```
<solutions>/
    solution1/
        1.0.0/
        1.2.0/
        2.0.0/
    solution2/
        3.5.9/
        4.2.0/
```

When an image `image1` from solution `mysolution` is requested, using reference `myregistry.lan/mysolution/image1:v1.2`
then the registry should iterate over every version sub-directory of `<solutions>/mysolution/` until it finds
one that contains the requested image.

The same logic must be followed when implementing registry discovery endpoints (getting manifests and referrers)

## Implementation details
This registry is a server built using Go. It uses a clean architecture paradigm.
for now, it only supports reading registry contents from a filesystem but the clean architecture makes
it extensible for other kinds of content sources.
