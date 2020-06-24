package counter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxUrlLength      = http.DefaultMaxHeaderBytes
	DefaultHttpClientTimeout = time.Second
	DefaultTimeout           = time.Second * 10
	DefaultMaxJobsN          = 1
)

type Options struct {
	MaxUrlLength int
	MaxJobsN     int
	HttpClient   *http.Client
	Timeout      time.Duration
}

type CountFunc func(bs []byte) (int, error)

type Counter interface {
	Count(r io.Reader, w io.Writer, substr string) error
}

type result struct {
	url string
	cnt int
	err error
}

type counter struct {
	Options
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
	if ops.MaxJobsN == 0 {
		ops.MaxJobsN = DefaultMaxJobsN
	}
	return counter{ops}
}

func (c counter) Count(r io.Reader, w io.Writer, substr string) error {
	if r == nil {
		return fmt.Errorf("reader cant be nil")
	}
	if w == nil {
		return nil
	}
	jobs := make(chan string, c.MaxJobsN)
	results := make(chan result, c.MaxJobsN)
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for u := range jobs {
			cnt, err := countInResource(u, c.HttpClient, substr)
			if err != nil {
				results <- result{url: u, cnt: 0, err: err}
				return
			}
			results <- result{url: u, cnt: cnt, err: nil}
		}
	}
	scanner := bufio.NewScanner(r)
	uniqueUrls := map[string]struct{}{}
	var n int
	var queued []string
	for scanner.Scan() {
		bs := scanner.Bytes()
		if len(bs) > c.MaxUrlLength {
			return fmt.Errorf("%d url is too long, max url length is: %d bytes", n, c.MaxUrlLength)
		}
		u := string(bs)
		_, err := url.Parse(u)
		if err != nil {
			return fmt.Errorf("%d url is invalid: %s", n, err)
		}
		if _, ok := uniqueUrls[u]; ok {
			continue
		}
		uniqueUrls[u] = struct{}{}
		n++
		if n <= c.MaxJobsN {
			wg.Add(1)
			go worker()
		}
		if len(jobs) < cap(jobs) {
			jobs <- u
			continue
		}
		queued = append(queued, u)
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	var ttl, done int
outer:
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out")
		case res := <-results:
			if res.err != nil {
				return res.err
			}
			if _, err := w.Write([]byte(fmt.Sprintf("Count for %s: %d\n", res.url, res.cnt))); err != nil {
				return err
			}
			ttl += res.cnt
			done++
			if done == n {
				break outer
			}
			if len(queued) == 0 {
				continue
			}
			jobs <- queued[0]
			queued = queued[1:]
		}
	}
	close(jobs)
	wg.Wait()
	if _, err := w.Write([]byte(fmt.Sprintf("Total: %d\n", ttl))); err != nil {
		return err
	}
	return nil
}

func countInResource(u string, cli *http.Client, substr string) (int, error) {
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
	return strings.Count(string(bs), substr), nil
}
