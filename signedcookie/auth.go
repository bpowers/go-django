// Copyright 2014 Bobby Powers. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package signedcookie

import (
	"bytes"
	"compress/zlib"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/bpowers/go-django/internal/github.com/kisielk/og-rek"
)

type Serializer int

const (
	Pickle Serializer = iota
	JSON
)

const DefaultMaxAge = 14 * 24 * time.Hour

// the salt value used by the signed_cookies SessionStore, it is not
// configurable through normal means.
const salt = "django.contrib.sessions.backends.signed_cookies"

var defaultSep = []byte{':'}

// b64Encode encodes a slice of bytes in a Django-compatable way,
// trimming trailing '=' padding specified by the standard.
func b64Encode(b []byte) []byte {
	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.URLEncoding, &buf)
	encoder.Write(b)
	encoder.Close()
	return bytes.TrimRight(buf.Bytes(), "=")
}

// b64Decode decodes a base64-encoded string that was generated by
// Django - specifically it adds any '=' padding that had previously
// been stripped back to the end of the byte slice to ensure Go's
// base64 decoder reads the entire payload.
func b64Decode(b []byte) ([]byte, error) {
	// Django's signing module strips all '=' padding from its
	// encoded representation of b.  Add them back here.
	pad := 4 - (len(b) % 4)
	for i := 0; i < pad; i++ {
		// append is ideal here, because we can overwrite the
		// timestamp that immediately follows the payload and
		// avoid an allocation.
		b = append(b, '=')
	}
	return ioutil.ReadAll(base64.NewDecoder(base64.URLEncoding, bytes.NewReader(b)))
}

var (
	base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// b62decode decodes a base62-encoded string into an int64, using the
// same method as Django's django.utils.baseconv.BaseConverter.
func b62Decode(b []byte) (int64, error) {
	var n int64
	for _, d := range b {
		i := strings.IndexByte(base62Alphabet, d)
		if i < 0 {
			return -1, fmt.Errorf("not base62 encoded")
		}
		n = n*int64(len(base62Alphabet)) + int64(i)
	}
	return n, nil
}

// djangoSignature calculates a HMAC signature in a way that matches
// django.core.signing.Signer.signature().
func djangoSignature(salt string, value []byte, secret string) []byte {
	// explicit make + append instead of
	// []byte(salt+"signer"+secret) avoids an allocation. copy
	// instead of append doesn't change allocation count.
	key := make([]byte, 0, len(salt)+len("signer")+len(secret))
	key = append(key, salt...)
	key = append(key, "signer"...)
	key = append(key, secret...)
	mac := hmac.New(sha1.New, key)
	mac.Write(value)
	return b64Encode(mac.Sum(nil))
}

// unsign returns the cookie payload if the signature matches the
// expected signature using the given secret, or an error otherwise.
func unsign(secret string, cookie []byte) ([]byte, error) {
	i := bytes.LastIndex(cookie, defaultSep)
	if i == -1 {
		return nil, fmt.Errorf("expected : in '%s'", string(cookie))
	}
	val := cookie[:i]
	sig := cookie[i+1:]
	expectedSig := djangoSignature(salt, val, secret)
	if subtle.ConstantTimeCompare([]byte(sig), expectedSig) != 1 {
		return nil, fmt.Errorf("signature mismatch: '%s' != '%s'", sig, string(expectedSig))
	}
	return val, nil
}

var now = time.Now

// timestampUnsign returns the cookie payload if the signature matches
// the expected signature using the given secret, and the timestamp of
// the cookie is still valid.  It wraps the unsign method.
func timestampUnsign(maxAge time.Duration, secret string, cookie []byte) ([]byte, error) {
	val, err := unsign(secret, cookie)
	if err != nil {
		return nil, fmt.Errorf("unsign('%s'): %s", string(cookie), err)
	}
	i := bytes.LastIndex(val, defaultSep)
	if i == -1 {
		return nil, fmt.Errorf("expected : in '%s'", string(cookie))
	}
	ts := val[i+1:]
	val = val[:i]
	stamp, err := b62Decode(ts)
	if err != nil {
		return nil, fmt.Errorf("b62Decode: %s", err)
	}
	if time.Unix(stamp, 0).Add(maxAge).Before(now()) {
		return nil, fmt.Errorf("expired timestamp: %d", stamp)
	}
	return val, nil
}

// signingLoads implements cookie object decoding in a way that is
// compatable with django.core.signing.loads.  It returns a map
// representing the encoded object, or an error if one occured.
func signingLoads(s Serializer, maxAge time.Duration, secret, cookie string) (map[string]interface{}, error) {
	c := []byte(cookie) // XXX: does this escape?
	payload, err := timestampUnsign(maxAge, secret, c)
	if err != nil {
		return nil, fmt.Errorf("timestampUnsign: %s", err)
	}
	decompress := false
	if payload[0] == '.' {
		decompress = true
		payload = payload[1:]
	}
	payload, err = b64Decode(payload)
	if err != nil {
		return nil, fmt.Errorf("base64Decode('%s'): %s", string(payload), err)
	}
	if decompress {
		r, err := zlib.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("zlib.NewReader: %s", err)
		}
		payload, err = ioutil.ReadAll(r)
		r.Close()
		if err != nil {
			return nil, fmt.Errorf("ReadAll(zlib): %s", err)
		}
	}
	o := make(map[string]interface{})
	if s == JSON {
		json.Unmarshal(payload, &o)
	} else {
		d := ogórek.NewDecoder(bytes.NewReader(payload))
		val, err := d.Decode()
		if err != nil {
			return nil, fmt.Errorf("Decode: %s", err)
		}
		mapI, ok := val.(map[interface{}]interface{})
		if !ok {
			return nil, fmt.Errorf("mapI not an object: %#v", mapI)
		}
		for ki, v := range mapI {
			k, ok := ki.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key in map: %#v", ki)
			}
			o[k] = v
		}
	}
	return o, nil
}

// Decode returns a map representing an object that was encoded and
// signed by the django.contrib.sessions.backends.signed_cookies
// SessionStore, or an error if the cookie could not be decoded or if
// signature validation failed.
func Decode(s Serializer, maxAge time.Duration, secret, cookie string) (map[string]interface{}, error) {
	return signingLoads(s, maxAge, secret, cookie)
}
