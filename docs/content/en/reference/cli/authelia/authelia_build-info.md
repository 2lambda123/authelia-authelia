---
title: "authelia build-info"
description: "Reference for the authelia build-info command."
lead: ""
date: 2022-06-15T17:51:47+10:00
draft: false
images: []
menu:
  reference:
    parent: "cli-authelia"
weight: 905
toc: true
---

## authelia build-info

Show the build information of Authelia

### Synopsis

Show the build information of Authelia.

This outputs detailed version information about the specific version
of the Authelia binary. This information is embedded into Authelia
by the continuous integration.

This could be vital in debugging if you're not using a particular
tagged build of Authelia. It's suggested to provide it along with
your issue.


```
authelia build-info [flags]
```

### Examples

```
authelia build-info
```

### Options

```
  -h, --help   help for build-info
```

### Options inherited from parent commands

```
  -c, --config strings                        configuration files to load (default [configuration.yml])
      --config.directory string               configuration directory to load configuration files from
      --config.experimental.filters strings   list of filters to apply to all configuration files between loading them from disk and parsing their content, options are 'template', 'expand-env'
```

### SEE ALSO

* [authelia](authelia.md)	 - authelia untagged-unknown-dirty (master, unknown)

