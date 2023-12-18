## sdstatus

This is a tool to check the status of
[SecureDrop](https://securedrop.org) instances by making requests to
the `/metadata` API endpoint.

The list of sites to check can be supplied on the command line.
By default, the script will use the SecureDrop directory API endpoint
at securedrop.org.

## How to build?

- `cargo build`

## Usage

Run `sdstatus --help` for full instructions.

## Output format

By default the tool prints JSON output on standard output. It is a
list of dictionaries reporting whether the site was available, any
error encountered, and if none was, the site's metadata, including the
SecureDrop version, journalist GPG key fingerprint, and supported
languages.

License: GPLv3+
