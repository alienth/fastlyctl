# fastlyctl

fastlyctl is a utility which can be used to interact with and manage your Fastly
service configuration. The most notable subcommand of fastlyctl is `sync`, which
can be used to synchronize Fastly settings with definitions in a local config
file.

This utility is still in heavy development. It may break your production
environment, so please use extreme caution.

## Commands

### sync

Use to synchronize remote service configurations with settings defined in a
local config file.

By default, will read `config.toml` in CWD. You can specify alternate files
names with the `-c` flag. Can take in either json or toml files. The suffix of
the file must be either `json` or `toml`.

The `sync` command will by default prompt to activate any changes made to a
service. `sync` can be made to automatically apply changes with the `-y` flag.
Example usage:

```
fastlyctl sync SomeServiceName
```

For further info, run `fastlyctl sync -h`.

#### config file

The config file can have defaults set in the `_default_` service. These defaults
will apply to all other specified services, unless explicitly overridden in
those services. Of note, the `_default_` service does not merge arrays of
instances. If you override a single instance of a definition, such
as a backend, you must re-specify all `backend` instances for that service.

Some configuration parameters may contain tokens that are replaced during
processing. For example, the '_servicename_' token within a Domain.Name will be
replaced by the name of a given service.

TODO: Document all replacements.

Example config.toml file:

```
[_default_]

   [_default_.Settings]
     DefaultTTL = 3600

   [[_default_.Domains]]
     Name = "_servicename_"

# If service name contains a period, it must be quoted
["someservice.com"]

   [["someservice.com".Domains]]
     Name = "_servicename_"

   [["someservice.com".Domains]]
     Name = "*._servicename_"
```

### service

For further info, run `fastlyctl service -h`.


### version

For further info, run `fastlyctl version -h`.
