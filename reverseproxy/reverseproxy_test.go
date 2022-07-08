package reverseproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathEncodeAfterDisabled(t *testing.T) {
	d := DebugTransport{
		pathEncodeAfter: "",
	}

	res := d.pathEncode("/content/path/")

	assert.Equal(t, res, "/content/path/")
}

func TestPathEncodeAfter(t *testing.T) {
	d := DebugTransport{
		pathEncodeAfter: "/content/path/",
	}

	res := d.pathEncode("proxy/v1/spaces/-M6UYg3Uh0RcTd_2PSQ0/content/path/activities/setting-up-an-activity")

	assert.Equal(t, res, "proxy/v1/spaces/-M6UYg3Uh0RcTd_2PSQ0/content/path/activities%2Fsetting-up-an-activity")
}

func TestPathEncodeAfterNotFound(t *testing.T) {
	d := DebugTransport{
		pathEncodeAfter: "/abc/",
	}

	res := d.pathEncode("proxy/v1/spaces/-M6UYg3Uh0RcTd_2PSQ0/content/path/activities/setting-up-an-activity")

	assert.Equal(t, res, "proxy/v1/spaces/-M6UYg3Uh0RcTd_2PSQ0/content/path/activities/setting-up-an-activity")
}
