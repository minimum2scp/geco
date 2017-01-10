# Geco

[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat)](LICENSE.txt)
[![Build Status](https://travis-ci.org/minimum2scp/geco.svg)](https://travis-ci.org/minimum2scp/geco)
[![Code Climate](https://codeclimate.com/github/minimum2scp/geco/badges/gpa.svg)](https://codeclimate.com/github/minimum2scp/geco)

## Description
 
geco = gcloud + peco: select GCP resource using peco, and run gcloud

## Dependencies

 * [gcloud (Google Cloud SDK)](https://cloud.google.com/sdk/)
   * `geco` uses [application default credential](https://developers.google.com/identity/protocols/application-default-credentials), so please run `gcloud auth application-default login` before use `geco`.
 * [peco](https://github.com/peco/peco)

## Installation

### Binary install

You can download binary from [Github releases](https://github.com/minimum2scp/geco/releases).

1. Download the zip file and unpack it.
2. Put binary file into somewhere you want.
3. Set the binary file to executable.

### Clone the project (for developers)

```bash
$ curl https://glide.sh/get | sh                      # See http://glide.sh/ for details
$ git clone https://github.com/minimum2scp/geco/
$ cd geco
$ glide install
$ go build
```

## Contribution

1. Fork it ( https://github.com/minimum2scp/geco/fork )
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create a new Pull Request

## Contributing

[minimum2scp](https://github.com/minimum2scp)
[Shinichirow KAMITO](https://github.com/kamito)
