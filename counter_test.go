package main

import (
	"bytes"
	"github.com/foxen/urls/counter"
	"io"
	"os"
	"path"
	"strings"
	"testing"
)

func TestCounter(t *testing.T) {
	goldenData := map[string]string{
		"https://golang.org/pkg/compress/":          "9",
		"https://golang.org/pkg/crypto/md5/":        "10",
		"https://golang.org/":                       "20",
		"https://golang.org/doc/":                   "75",
		"https://golang.org/pkg/compress/gzip/":     "17",
		"https://golang.org/pkg/debug/pe/":          "25",
		"https://golang.org/pkg/log/syslog/":        "9",
		"https://golang.org/pkg/sort/":              "51",
		"https://golang.org/pkg/strconv/":           "41",
		"https://golang.org/pkg/sync/":              "21",
		"https://golang.org/pkg/strings/":           "55",
		"https://golang.org/pkg/unsafe/":            "15",
		"https://golang.org/pkg/unicode/":           "37",
		"https://golang.org/pkg/time/":              "50",
		"https://godoc.org/golang.org/x/net":        "9",
		"https://godoc.org/golang.org/x/benchmarks": "8",
		"https://godoc.org/golang.org/x/mobile":     "19",
	}
	goldenTtl := "471"

	ctr := counter.New(counter.Options{
		MaxJobsN: 5,
	})
	f, err := os.Open(path.Join("testdata", "urls")) // https://golang.org/pkg/strings/ doubled in file
	defer f.Close()
	if err != nil {
		t.Fatal(err)
	}
	w := bytes.NewBufferString("")
	if err := ctr.Count(f, w, "Go"); err != nil {
		t.Fatal(err)
	}
	o := w.String()
	data, ttl := parseOutput(o, t)
	if len(goldenData) != len(data) {
		t.Fatalf("want: %d; get: %d", len(goldenData), len(data))
	}
	for u, gv := range goldenData {
		v, ok := data[u]
		if !ok {
			t.Fatalf("missed: %s", u)
		}
		if gv != v {
			t.Fatalf("want: %s; get: %s", gv, v)
		}
	}
	if goldenTtl != ttl {
		t.Fatalf("want: %s; get: %s", goldenTtl, ttl)
	}
	t.Log(o)
	empty := `
`
	justWrong := "https://a.b"
	w.Reset()
	wrongCases := []struct {
		name   string
		substr string
		r      io.Reader
		w      io.Writer
	}{
		{
			"empty string",
			"Go",
			strings.NewReader(empty),
			w,
		},
		{
			"wrong url",
			"Go",
			strings.NewReader(justWrong),
			w,
		},
	} // and so on
	// TODO: improve wrong cases coverage
	for _, c := range wrongCases {
		t.Run(c.name, func(t *testing.T) {
			if err := ctr.Count(c.r, c.w, c.substr); err == nil {
				t.Fatal(err)
			} else {
				t.Log(err)
			}
		})
	}
}

func parseOutput(o string, t *testing.T) (map[string]string, string) {
	lines := strings.Split(o, "\n")
	res := map[string]string{}
	ttl := ""
	for _, l := range lines {
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "Total: ") {
			ttl = l[len("Total: "):]
			continue
		}
		if !strings.HasPrefix(l, "Count for ") {
			t.Fatalf("unexpected line: %s", l)
		}
		l = l[len("Count for "):]
		sl := strings.Split(l, ": ")
		if len(sl) != 2 {
			t.Fatalf("unexpected line format: %s", l)
		}
		res[sl[0]] = sl[1]
	}
	return res, ttl
}
