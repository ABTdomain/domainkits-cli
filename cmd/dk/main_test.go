package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// seen is what the test server received in the last request.
type seen struct {
	method, path, query, auth, ua, ctype, body string
}

type recorder struct {
	mu   sync.Mutex
	last *seen
}

func (r *recorder) get() *seen {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last
}

// serve starts a server that records each request and answers with h, or
// with {"success":true} when h is nil, and points the CLI at it.
func serve(t *testing.T, h http.HandlerFunc) *recorder {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.last = &seen{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"),
			r.Header.Get("User-Agent"), r.Header.Get("Content-Type"), string(b)}
		rec.mu.Unlock()
		if h == nil {
			io.WriteString(w, `{"success":true}`)
			return
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DOMAINKITS_API_URL", srv.URL+"/api/v1")
	t.Setenv("DOMAINKITS_API_KEY", "dk_test")
	return rec
}

// call runs the CLI with the given stdin and returns the exit code, stdout
// and stderr.
func call(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	open := func(name, content string) *os.File {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	in, out, errOut := open("stdin", stdin), open("stdout", ""), open("stderr", "")
	oldIn, oldOut, oldErr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = in, out, errOut
	code := run(args)
	os.Stdin, os.Stdout, os.Stderr = oldIn, oldOut, oldErr
	for _, f := range []*os.File{in, out, errOut} {
		f.Close()
	}
	o, _ := os.ReadFile(filepath.Join(dir, "stdout"))
	e, _ := os.ReadFile(filepath.Join(dir, "stderr"))
	return code, string(o), string(e)
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func TestRequests(t *testing.T) {
	cases := []struct {
		args                      []string
		method, path, query, body string
	}{
		{[]string{"search", "nrds", "query=ai", "--tld=com", "--limit", "20"}, "GET", "/api/v1/search/nrds", "limit=20&query=ai&tld=com", ""},
		{[]string{"search", "expired", "exclude=-shop"}, "GET", "/api/v1/search/expired", "exclude=-shop", ""},
		{[]string{"whois", "example.com"}, "GET", "/api/v1/whois", "domain=example.com", ""},
		{[]string{"ns-reverse", "ns1.example.com,ns2.example.com", "tld=com"}, "GET", "/api/v1/ns-reverse", "ns=ns1.example.com%2Cns2.example.com&tld=com", ""},
		{[]string{"registrar", "godaddy"}, "GET", "/api/v1/registrar", "query=godaddy", ""},
		{[]string{"tld-check", "shop"}, "GET", "/api/v1/tld-check", "prefix=shop", ""},
		{[]string{"ct", "subdomains", "github.com"}, "GET", "/api/v1/ct/subdomains", "domain=github.com", ""},
		{[]string{"trends", "keywords", "hot", "scope=com"}, "GET", "/api/v1/trends/keywords/hot", "scope=com", ""},
		{[]string{"monitor", "type=new"}, "GET", "/api/v1/monitor/changes", "type=new", ""},
		{[]string{"get", "/api/v1/search/status"}, "GET", "/api/v1/search/status", "", ""},
		{[]string{"bulk", "whois", "a.com", "b.com"}, "POST", "/api/v1/bulk/whois", "", `{"domains":["a.com","b.com"]}`},
		{[]string{"post", "bulk/dns", `{"domains": ["a.com"]}`}, "POST", "/api/v1/bulk/dns", "", `{"domains":["a.com"]}`},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			rec := serve(t, nil)
			code, out, errOut := call(t, "", c.args...)
			if code != 0 {
				t.Fatalf("exit %d, stderr %q", code, errOut)
			}
			got := rec.get()
			if got.method != c.method || got.path != c.path || got.query != c.query || got.body != c.body {
				t.Errorf("sent %s %s?%s %s", got.method, got.path, got.query, got.body)
			}
			if got.auth != "Bearer dk_test" || got.ua != "dk-cli/"+version {
				t.Errorf("auth %q, user agent %q", got.auth, got.ua)
			}
			if c.body != "" && got.ctype != "application/json" {
				t.Errorf("content type %q", got.ctype)
			}
			if out != `{"success":true}`+"\n" {
				t.Errorf("stdout %q", out)
			}
		})
	}
}

func TestHealthNeedsNoKey(t *testing.T) {
	rec := serve(t, nil)
	t.Setenv("DOMAINKITS_API_KEY", "")
	if code, _, errOut := call(t, "", "health"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if auth := rec.get().auth; auth != "" {
		t.Errorf("health sent Authorization %q", auth)
	}
}

func TestMissingKey(t *testing.T) {
	rec := serve(t, nil)
	t.Setenv("DOMAINKITS_API_KEY", "")
	code, _, errOut := call(t, "", "usage")
	if code != 2 || !strings.Contains(errOut, "DOMAINKITS_API_KEY is not set") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
	if rec.get() != nil {
		t.Error("request sent without a key")
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"nope"},
		{"whois"},
		{"whois", "domain=example.com"},
		{"ct", "search", "example.com"},
		{"trends", "tlds", "hot"},
		{"bulk", "whois"},
		{"search", "nrds", "tld"},
		{"search", "nrds", "--exclude_query", "--tld=com"},
		{"post", "bulk/dns", "{bad"},
		{"search", "nrds", "-o"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			rec := serve(t, nil)
			code, _, errOut := call(t, "", args...)
			if code != 2 || errOut == "" {
				t.Errorf("exit %d, stderr %q", code, errOut)
			}
			if rec.get() != nil {
				t.Error("request sent")
			}
		})
	}
}

func TestBulkFromStdin(t *testing.T) {
	rec := serve(t, nil)
	if code, _, errOut := call(t, "a.com\n\n b.com \n", "bulk", "dns", "-"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if body := rec.get().body; body != `{"domains":["a.com","b.com"]}` {
		t.Errorf("body %s", body)
	}
}

func TestAPIError(t *testing.T) {
	reset := strconv.FormatInt(time.Now().Add(42*time.Second).Unix(), 10)
	serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.Header().Set("X-RateLimit-Reset", reset)
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"success":false}`)
	})
	code, out, errOut := call(t, "", "usage")
	if code != 1 || out != `{"success":false}`+"\n" {
		t.Errorf("exit %d, stdout %q", code, out)
	}
	for _, want := range []string{"HTTP 429", "Retry-After: 42", "rate limit window resets in"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr %q lacks %q", errOut, want)
		}
	}
}

func TestExportToFile(t *testing.T) {
	const csv = "domain\nexample.com\n"
	serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="../nrds.csv"`)
		io.WriteString(w, csv)
	})
	dir := t.TempDir()
	chdir(t, dir)

	code, out, errOut := call(t, "", "search", "nrds", "export=csv")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "nrds.csv")); err != nil || string(b) != csv {
		t.Errorf("file %q, %v", b, err)
	}
	var res struct {
		Saved string
		Bytes int
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Bytes != len(csv) || filepath.Base(res.Saved) != "nrds.csv" {
		t.Errorf("stdout %q", out)
	}

	named := filepath.Join(t.TempDir(), "mine.csv")
	if code, _, errOut := call(t, "", "search", "nrds", "export=csv", "-o", named); code != 0 {
		t.Fatalf("-o: exit %d, stderr %q", code, errOut)
	}
	if b, err := os.ReadFile(named); err != nil || string(b) != csv {
		t.Errorf("-o file %q, %v", b, err)
	}
}

func TestDownloadFollowsRedirect(t *testing.T) {
	serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/nrds/download" {
			http.Redirect(w, r, "/files/2026-10-13_com.txt", http.StatusFound)
			return
		}
		io.WriteString(w, "example.com\n")
	})
	dir := t.TempDir()
	chdir(t, dir)
	if code, _, errOut := call(t, "", "download", "tld=com"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "2026-10-13_com.txt")); err != nil || string(b) != "example.com\n" {
		t.Errorf("file %q, %v", b, err)
	}
}

func TestBrokenTransferLeavesNoFile(t *testing.T) {
	serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="big.csv"`)
		w.Header().Set("Content-Length", "1000")
		io.WriteString(w, "partial")
	})
	dir := t.TempDir()
	chdir(t, dir)
	if code, _, _ := call(t, "", "search", "nrds", "export=csv"); code != 2 {
		t.Errorf("exit %d", code)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("left behind %d entries, first %q", len(entries), entries[0].Name())
	}
}

func TestVersion(t *testing.T) {
	if code, out, _ := call(t, "", "version"); code != 0 || out != "dk "+version+"\n" {
		t.Errorf("exit %d, stdout %q", code, out)
	}
}
