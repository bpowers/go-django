go-django - work with Django data in Go
=======================================

go-django is not a web framework, instead it is a collection of
utilities to help Go webservers integrate with Django servers.

Initially, it exposes a single method, `signedcookie.Decode`, which
will decode the payload of a cookie generated with Django's
`signed_cookie` session backend, using either the JSON or Pickle
serializer.

usage
-----

First, `go get` it:

    $ go get -u github.com/bpowers/go-django/...


Then import and use:

```Go
package main

import "github.com/bpowers/go-django/signedcookie"

const SecretKey = "64d446b6cabc3038d4c4398d210aaa5122b6bc5a"

var Pickle, MaxAge = signedcookie.Pickle, signedcookie.DefaultMaxAge

func example(cookie string) {
     session, err := signedcookie.Decode(Pickle, MaxAge, SecretKey, cookie)
     if err != nil {
         panic(err) // FIXME: elegantly propagate error
     }
     _ = session // use session; it is a map[string]interface{}
}
```

license
-------

go-django is offered under the MIT license, see LICENSE for details.
