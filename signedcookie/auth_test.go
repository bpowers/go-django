package signedcookie

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.com/bpowers/go-django/internal/github.com/kisielk/og-rek"
)

var decodeData = []struct {
	kind    Serializer
	secret  string
	cookie  string
	decoded map[string]interface{}
}{
	{
		Pickle,
		"70e97f01975bb59ae8804ca164081c46034042aa913a4dac055cad6a7e188bd1",
		".eJxrYKotZNQI5Y1PLC3JiC8tTi2Kz0wpZPI1Yw0VQhJLSkzOTs1LKWQOFSrOz03VKy5PTS3Rc4KIluoBAEyaGG0:1XeDNx:RIsFaf0wIba2w-wXrFz47me6Zcw",
		map[string]interface{}{
			"_auth_user_backend": "some.sweet.Backend",
			"_auth_user_id":      int64(1334),
		},
	},
	{
		JSON,
		"70e97f01975bb59ae8804ca164081c46034042aa913a4dac055cad6a7e188bd1",
		".eJyrVopPLC3JiC8tTi2Kz0xRsjI0NjbRQRZMSkzOTs0DyigV5-em6hWXp6aW6DlBBWsB4AYWwQ:1XeDSa:WrnCueUH3vz5K8cZidNGZSd-zQw",
		map[string]interface{}{
			"_auth_user_backend": "some.sweet.Backend",
			"_auth_user_id":      float64(1334),
		},
	},
}

func TestOgrekAllocs(t *testing.T) {
	now = testNowOK
	d := &decodeData[0]
	c := []byte(d.cookie)
	payload := bytes.Split(c, []byte{':'})[0]
	decompress := false
	if payload[0] == '.' {
		decompress = true
		payload = payload[1:]
	}
	payload, err := b64Decode(payload)
	if err != nil {
		panic(fmt.Errorf("base64Decode('%s'): %s", string(payload), err))
	}
	if decompress {
		var r io.ReadCloser
		r, err = zlib.NewReader(bytes.NewReader(payload))
		if err != nil {
			panic(fmt.Errorf("zlib.NewReader: %s", err))
		}
		payload, err = ioutil.ReadAll(r)
		r.Close()
		if err != nil {
			panic(fmt.Errorf("ReadAll(zlib): %s", err))
		}
	}
	r := bytes.NewReader(payload)
	n := testing.AllocsPerRun(100, func() {
		if _, err := r.Seek(0, 0); err != nil {
			panic(err)
		}
		val, err := ogórek.NewDecoder(r).Decode()
		if err != nil {
			panic(fmt.Errorf("Decode: %s", err))
		}
		_ = val
	})
	fmt.Printf("ogre allocs: %f\n", n)
	if n > 34 {
		t.Errorf("too many (%f) allocs in ogrek", n)
	}
}

func TestLoadsPickleAllocs(t *testing.T) {
	now = testNowOK
	n := testing.AllocsPerRun(100, func() {
		d := &decodeData[0]
		decoded, err := Decode(d.kind, DefaultMaxAge, d.secret, d.cookie)
		if err != nil {
			panic(err)
		}
		_ = decoded
	})
	fmt.Printf("load allocs pickle: %f\n", n)
	if n > 60 {
		t.Errorf("too many (%f) allocs in loads", n)
	}
}

func TestLoadsJSONAllocs(t *testing.T) {
	now = testNowOK
	n := testing.AllocsPerRun(100, func() {
		d := &decodeData[1]
		decoded, err := Decode(d.kind, DefaultMaxAge, d.secret, d.cookie)
		if err != nil {
			panic(err)
		}
		_ = decoded
	})
	fmt.Printf("load allocs json: %f\n", n)
	if n > 55 {
		t.Errorf("too many (%f) allocs in loads", n)
	}
}

func TestDecode(t *testing.T) {
	now = testNowOK
	for _, d := range decodeData {
		decoded, err := Decode(d.kind, DefaultMaxAge, d.secret, d.cookie)
		if err != nil {
			t.Errorf("Decode(%s, '%s', '%s'): %s", d.kind, d.secret, d.cookie, err)
			continue
		}
		expected := d.decoded
		if len(expected) != len(decoded) {
			t.Errorf("wrong len")
		}
		if !reflect.DeepEqual(expected, decoded) {
			t.Errorf("DeepEqual(%#v != %#v)", expected, decoded)
			continue
		}
	}
}

func testNowOK() time.Time {
	t, _ := time.Parse("2006-01-02", "2014-10-15")
	return t
}

func testNowTimedOut() time.Time {
	t, _ := time.Parse("2006-01-02", "2014-11-15")
	return t
}

func TestCookieTimeout(t *testing.T) {
	now = testNowTimedOut
	d := &decodeData[0]
	_, err := Decode(d.kind, DefaultMaxAge, d.secret, d.cookie)
	if err == nil {
		t.Errorf("should fail to decode, but doesn't")
	}
}

var base62Data = []struct {
	encoded string
	decoded int64
}{
	{"d5778337", 137633489102557},
	{"d5778349", 137633489102621},
}

func TestBase62Decode(t *testing.T) {
	for _, d := range base62Data {
		n, err := b62Decode([]byte(d.encoded))
		if err != nil {
			t.Errorf("b62Decode('%s'): %s", d.encoded, err)
			continue
		}
		if n != d.decoded {
			t.Errorf("incorrect decode: %d != %d", n, d.decoded)
		}
	}
}
