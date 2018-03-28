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



## Output format

```
[
	{
		"Info": {
			"sd_version": "0.6",
			"gpg_fpr": "3392A1CE68FE779A95FCAF04EDA0FB6F53FA9093"
		},
		"Url": "m4hynbhhctdk27jr.onion",
		"Available": true
	},
	{
		"Info": {
			"sd_version": "0.6",
			"gpg_fpr": "7C24A77EED0D50838E3315BD7A38590B2996F0C2"
		},
		"Url": "ftugftwajmgsmoau.onion",
		"Available": true
	}
]
```

Currently the tool prints JSON output on the STDOUT. It is a list of dictionaries telling if the site is available, and
the SecureDrop version and also the journalist GPG key fingerprint.

License: GPLv3+