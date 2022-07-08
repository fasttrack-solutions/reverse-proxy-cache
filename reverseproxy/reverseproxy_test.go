package reverseproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathEncodeAfterDisabled(t *testing.T) {
	rp := ReverseProxy{
		pathEncodeAfter: "",
	}

	res := rp.pathEncode("/content/path/")

	assert.Equal(t, res, "/content/path/")
}

func TestPathEncodeAfter(t *testing.T) {
	rp := ReverseProxy{
		pathEncodeAfter: "/content/path/",
	}

	res := rp.pathEncode("proxy/v1/spaces/-M6UYg3Uh0RcTd_2PSQ0/content/path/activities/setting-up-an-activity")

	assert.Equal(t, res, "proxy/v1/spaces/-M6UYg3Uh0RcTd_2PSQ0/content/path/activities%2Fsetting-up-an-activity")
}

func TestPathEncodeAfterNotFound(t *testing.T) {
	rp := ReverseProxy{
		pathEncodeAfter: "/abc/",
	}

	res := rp.pathEncode("proxy/v1/spaces/-M6UYg3Uh0RcTd_2PSQ0/content/path/activities/setting-up-an-activity")

	assert.Equal(t, res, "proxy/v1/spaces/-M6UYg3Uh0RcTd_2PSQ0/content/path/activities/setting-up-an-activity")
}
