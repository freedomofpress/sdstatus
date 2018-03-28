## sdstatus

This is a PoC tool to check the status of [SecureDrop](https://securedrop.org) instances.

All the addresses are mentioned in the `sdonion.txt` file, one address each line.
I have a copy of the addresses from the SecureDrop [directory](https://securedrop.org/directory)
to test.


## System requirement

The tool assumes that Tor is running with a SOCKS proxy at port 9050 in the system.

The dependencies are managed using [dep](https://golang.github.io/dep/) tool.


## How to build?

Checkout the git repo in your `$GOPATH`, and then simple `go build` will do the work
for you.



Currently the tool prints comma separated values in the STDOUT.


License: GPLv3+