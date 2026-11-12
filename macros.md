# List of available MACROs

* header.ABC - ANY header recorded by name, if missing it will be replaced with "missing", if empty (i.e. "") it will be empty
* urlhash.X_Y - sha256 hash split into X length parts, Y is the part instance (1 = 1st, 2 = 2nd, etc); therefore urlhash.2_3 would be the the hash split into 2 character chunks, and the 3rd chunk...if in a zero based arry it would be urlhash[4:6]


* header.content-length
* header.content-type
* header.warc-block-digest
* header.warc-cipher-suite
* header.warc-concurrent-to
* header.warc-date
* header.warc-ip-address
* header.warc-identified-payload-type
* header.warc-payload-digest
* header.warc-protocol
* header.warc-record-id
* header.warc-target-uri
* header.warc-type
* header.warc-warcinfo-id
* url.host
* url.hostname
* url.fragment
* url.opaque
* url.path
* url.rawfragment
* url.rawpath
* url.rawquery
* url.scheme
* url.user
* url.username
* url.password
* url.hash
* tld.tld
* tld.domainroot
* tld.subdomain
