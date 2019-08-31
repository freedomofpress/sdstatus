## sdstatus

This is a tool to check the status of
[SecureDrop](https://securedrop.org) instances by making requests to
the `/metadata` API endpoint.

The list of sites to check can be supplied on the command line, in a
CSV file containing one address and title per line, or retrieved from
the SecureDrop directory API endpoint at securedrop.org.

## System requirements

You must have Tor running with a SOCKS proxy at port 9050 in the system.

## How to build?

- `go build`

## Usage

Run `sdstatus --help` for full instructions. You need to supply one or
both of the `--directory` and `--inputFile` arguments, to provide the
list of SecureDrop sites to scan.

## Output format

By default the tool prints JSON output on standard output. It is a
list of dictionaries reporting whether the site was available, any
error encountered, and if none was, the site's metadata, including the
SecureDrop version, journalist GPG key fingerprint, and supported
languages.

If you pass `--format=csv` flag to the tool, then it will print output
in CSV format.

You can direct output to a file instead with the `--outputFile`
argument.

License: GPLv3+
