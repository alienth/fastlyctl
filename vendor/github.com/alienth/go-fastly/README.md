Go Fastly
=========

Go Fastly is a Golang API client for interacting with most facets of the
[Fastly API](https://docs.fastly.com/api).

This library is a fork of an [existing
library](https://github.com/sethvargo/go-fastly) by Seth Vargo. The primary
difference is related to the types that are used to interact with the various
API functions.  All functions for a given thing you're trying to adjust, such as
a backend, utilize a single `Backend` type, rather than a separate type for
creating/updating/deleting. Additionally, this library only communicates with
the API in JSON.

Another difference is I haven't rewritten the test code for this library. As
such, use at your own risk!

The primary use of this library is in
[fastlyctl](https://github.com/alienth/fastlyctl), a utility for synchronizing a
fastly config based on definitions within a local config file.

Installation
------------
This is a client library, so there is nothing to install.

Usage
-----
Download the library into your `$GOPATH`:

    $ go get github.com/alienth/go-fastly

Import the library into your tool:

```go
import "github.com/alienth/go-fastly"
```

Examples
--------
Fastly's API is designed to work in the following manner:

1. Create (or clone) a new configuration version for the service
2. Make any changes to the version
3. Validate the version
4. Activate the version

This flow using the Golang client looks like this:

```go
// Create a client object. The client has no state, so it can be persisted
// and re-used. It is also safe to use concurrently due to its lack of state.
client := fastly.NewClient(nil, "YOUR_FASTLY_API_KEY")

// You can find the service ID in the Fastly web console.
var serviceID = "SU1Z0isxPaozGVKXdv0eY"

// Get the service
service, _, err := client.Service.Get(serviceID)
if err != nil {
  log.Fatal(err)
}

// Clone the service's latest version so we can make changes without affecting
// the active configuration.
version, _, err := client.Version.Clone(serviceID, service.Version)
if err != nil {
  log.Fatal(err)
}

// Now you can make any changes to the new version. In this example, we will add
// a new domain.
newDomain = new(fastly.Domain)
newDomain.Name = "example.com"
domain, _, err := client.Domain.Create(serviceID, version.Number, newDomain)
if err != nil {
  log.Fatal(err)
}

// Output: "example.com"
fmt.Println(domain.Name)

// Finally, activate this new version.
activeVersion, _, err := client.Version.Activate(serviceID, version.Number)
if err != nil {
  log.Fatal(err)
}

// Output: true
fmt.Printf("%b", activeVersion.Locked)
```

More information can be found in the
[Godoc](https://godoc.org/github.com/alienth/go-fastly).
