package server

import "testing"

func TestHankSVGAssetsMustRevalidate(t *testing.T) {
	for _, path := range []string{"/img/pip/base.svg", "/img/pip/racket.svg", "/img/pip/face-neutral.svg"} {
		if got := cacheControl(path); got != "no-cache" {
			t.Errorf("%s cache policy = %q", path, got)
		}
	}
}
