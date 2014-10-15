go-django - work with Django data in Go
=======================================

go-django is not a web framework, instead it is a collection of
utilities to help Go webservers integrate with Django servers.

Initially, it exposes a single method, `signedcookie.Decode`, which
will decode the payload of a cookie generated with Django's
`signed_cookie` session backend, using either the JSON or Pickle
serializer.

license
-------

go-django is offered under the MIT license, see LICENSE for details.
