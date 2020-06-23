package counter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultMaxUrlLength      = http.DefaultMaxHeaderBytes
	DefaultHttpClientTimeout = time.Second
	DefaultTimeout           = time.Second * 10
)

type Options struct {
	MaxUrlLength int
	MaxJobsN     int // 0 stands for unlimited, default
	HttpClient   *http.Client
	Timeout       time.Duration
}

type CountFunc func(bs []byte) (int, error)

type Counter interface {
	CountWith(cf CountFunc, r io.Reader, w io.Writer) error
}

type jobResult struct {
	url string
	cnt int
	err error
}

type counter struct {
	ops Options
}

func New(ops Options) Counter {
	if ops.MaxUrlLength == 0 {
		ops.MaxUrlLength = DefaultMaxUrlLength
	}
	if ops.HttpClient == nil {
		ops.HttpClient = &http.Client{
			Timeout: DefaultHttpClientTimeout,
		}
	}
	if ops.Timeout == 0 {
		ops.Timeout = DefaultTimeout
	}
	return &counter{ops}
}

func (c *counter) CountWith(cf CountFunc, r io.Reader, w io.Writer) error {
	if r == nil {
		return fmt.Errorf("reader cant be nil")
	}
	if cf == nil {
		return fmt.Errorf("count function cant be nil")
	}
	if w == nil {
		return nil
	}
	urls, err := readAndGuardUrls(r, c.ops.MaxUrlLength)
	if err != nil {
		return err
	}
	l := len(urls)
	done := make(chan jobResult, l)
	ctx, cancel := context.WithTimeout(context.Background(), c.ops.Timeout)
	defer cancel()
	maxJobsN := c.ops.MaxJobsN
	if c.ops.MaxJobsN == 0 {
		maxJobsN = len(urls)
	}
	for i := 0; i < maxJobsN; i++ {
		if len(urls) == 0 {
			break
		}
		go newJob(c.ops.HttpClient, cf, done)(urls[0])
		urls = urls[1:]
	}
	ttl := 0
	doneN := 0
outer:
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out")
		case res := <-done:
			if res.err != nil {
				return res.err
			}
			if _, err := w.Write([]byte(fmt.Sprintf("Count for %s: %d\n", res.url, res.cnt))); err != nil {
				return err
			}
			ttl += res.cnt
			doneN++
			if doneN == l {
				break outer
			}
			if len(urls) > 0 {
				go newJob(c.ops.HttpClient, cf, done)(urls[0])
				urls = urls[1:]
			}
		}
	}
	if _, err := w.Write([]byte(fmt.Sprintf("Total: %d\n", ttl))); err != nil {
		return err
	}
	return nil
}

func readAndGuardUrls(r io.Reader, maxUrlLength int) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var n int
	uniqueUrls := map[string]struct{}{}
	var urls []string
	for scanner.Scan() {
		n++
		bs := scanner.Bytes()
		if len(bs) > maxUrlLength {
			return nil, fmt.Errorf("%d url is too long, max url length is: %d bytes", n, maxUrlLength)
		}
		u := string(bs)
		_, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("%d url is invalid: %s", n, err)
		}
		if _, ok := uniqueUrls[u]; ok {
			continue
		}
		uniqueUrls[u] = struct{}{}
		urls = append(urls, u)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}

func newJob(cli *http.Client, cf CountFunc, done chan jobResult) func(string) {
	return func(u string) {
		cnt, err := func(u string) (int, error) {
			resp, err := cli.Get(u)
			if err != nil {
				return 0, err
			}
			if resp.StatusCode != http.StatusOK {
				return 0, fmt.Errorf("non 200 response: %d", resp.StatusCode)
			}
			bs, err := ioutil.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				return 0, err
			}
			return cf(bs)
		}(u)
		done <- jobResult{url: u, cnt: cnt, err: err}
	}
}
