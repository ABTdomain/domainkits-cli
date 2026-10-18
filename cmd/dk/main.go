// dk: command-line client for the DomainKits API.
//
// Thin by design: every command maps to one endpoint, positional arguments
// fill the endpoint's required parameter, and any trailing key=value pairs
// are appended to the query string untouched. The server validates
// parameters and its error message is printed as-is, so the API
// documentation (https://domainkits.com/dev/api-docs) is the reference for
// what each endpoint accepts.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	version     = "0.4.2"
	defaultBase = "https://premium-api.domainkits.com/api/v1"
)

// Only the wait for response headers is bounded: a whole-request timeout
// would also cap reading the body and cut off large downloads.
var client = func() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 180 * time.Second
	return &http.Client{Transport: t}
}()

type request struct {
	method string
	path   string // relative to the base URL, no leading slash
	query  url.Values
	body   any  // JSON-encoded when non-nil
	auth   bool // false only for endpoints that take no key
	toFile bool // response is a file even without Content-Disposition
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(usageText)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Println("dk " + version)
		return 0
	}

	// -o / --output may appear anywhere.
	var out string
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				return fail(2, "-o needs a file name")
			}
			out = args[i+1]
			i++
		case strings.HasPrefix(a, "--output="):
			out = strings.TrimPrefix(a, "--output=")
		default:
			rest = append(rest, a)
		}
	}

	req, err := build(rest)
	if err != nil {
		return fail(2, err.Error()+"\n\nRun 'dk help' for usage.")
	}
	return send(req, out)
}

// pos takes n positional arguments from the front of a.
func pos(a []string, n int, what string) ([]string, []string, error) {
	if len(a) < n {
		return nil, nil, errors.New(what + " required")
	}
	for _, v := range a[:n] {
		if strings.Contains(v, "=") || strings.HasPrefix(v, "-") {
			return nil, nil, fmt.Errorf("%s required (got parameter %q)", what, v)
		}
	}
	return a[:n], a[n:], nil
}

func oneOf(v string, allowed ...string) error {
	for _, x := range allowed {
		if v == x {
			return nil
		}
	}
	return fmt.Errorf("%q is not one of: %s", v, strings.Join(allowed, ", "))
}

func build(a []string) (*request, error) {
	if len(a) == 0 {
		return nil, errors.New("command required")
	}
	cmd, a := a[0], a[1:]
	r := &request{method: "GET", query: url.Values{}, auth: true}
	var p []string
	var err error

	switch cmd {
	case "health":
		r.path, r.auth = "health", false
	case "usage":
		r.path = "usage"
	case "search":
		if p, a, err = pos(a, 1, "search type"); err != nil {
			return nil, err
		}
		r.path = "search/" + p[0]
	case "whois", "dns":
		if p, a, err = pos(a, 1, "domain"); err != nil {
			return nil, err
		}
		r.path = cmd
		r.query.Set("domain", p[0])
	case "ip-lookup", "registrar", "status-guide":
		if p, a, err = pos(a, 1, "query"); err != nil {
			return nil, err
		}
		r.path = cmd
		r.query.Set("query", p[0])
	case "tld-check":
		if p, a, err = pos(a, 1, "prefix"); err != nil {
			return nil, err
		}
		r.path = cmd
		r.query.Set("prefix", p[0])
	case "ns-reverse":
		if p, a, err = pos(a, 1, "nameserver"); err != nil {
			return nil, err
		}
		r.path = cmd
		r.query.Set("ns", p[0])
	case "typosquat":
		if p, a, err = pos(a, 1, "domain"); err != nil {
			return nil, err
		}
		r.path = cmd
		r.query.Set("domain", p[0])
	case "monitor":
		r.path = "monitor/changes"
	case "download":
		r.path = "nrds/download"
		r.toFile = true
	case "ct":
		if p, a, err = pos(a, 2, "ct subcommand and its argument"); err != nil {
			return nil, err
		}
		if err = oneOf(p[0], "subdomains", "certs"); err != nil {
			return nil, err
		}
		r.path = "ct/" + p[0]
		r.query.Set("domain", p[1])
	case "trends":
		if p, a, err = pos(a, 2, "trends subcommand and type"); err != nil {
			return nil, err
		}
		switch p[0] {
		case "tlds":
			err = oneOf(p[1], "active", "newly")
		case "keywords":
			err = oneOf(p[1], "hot", "emerging", "prefix")
		default:
			err = oneOf(p[0], "tlds", "keywords")
		}
		if err != nil {
			return nil, err
		}
		r.path = "trends/" + p[0] + "/" + p[1]
	case "bulk":
		if p, a, err = pos(a, 1, "bulk subcommand"); err != nil {
			return nil, err
		}
		if err = oneOf(p[0], "whois", "dns"); err != nil {
			return nil, err
		}
		domains, derr := domainList(a)
		if derr != nil {
			return nil, derr
		}
		r.method = "POST"
		r.path = "bulk/" + p[0]
		r.body = map[string]any{"domains": domains}
		a = nil
	case "get":
		if p, a, err = pos(a, 1, "path"); err != nil {
			return nil, err
		}
		r.path = cleanPath(p[0])
	case "post":
		if p, a, err = pos(a, 1, "path"); err != nil {
			return nil, err
		}
		if len(a) < 1 {
			return nil, errors.New("JSON body required (or - to read it from stdin)")
		}
		raw := a[0]
		if raw == "-" {
			b, rerr := io.ReadAll(os.Stdin)
			if rerr != nil {
				return nil, rerr
			}
			raw = string(b)
		}
		r.method = "POST"
		r.path = cleanPath(p[0])
		r.body = json.RawMessage(raw)
		a = a[1:]
	default:
		return nil, fmt.Errorf("unknown command %q", cmd)
	}

	if err := params(r.query, a); err != nil {
		return nil, err
	}
	return r, nil
}

// cleanPath accepts "search/nrds", "/search/nrds" or "/api/v1/search/nrds".
func cleanPath(p string) string {
	p = strings.TrimLeft(p, "/")
	return strings.TrimPrefix(p, "api/v1/")
}

// domainList reads domains from arguments, or from stdin (one per line) when
// the only argument is "-".
func domainList(a []string) ([]string, error) {
	if len(a) == 1 && a[0] == "-" {
		var out []string
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			if d := strings.TrimSpace(sc.Text()); d != "" {
				out = append(out, d)
			}
		}
		return out, sc.Err()
	}
	if len(a) == 0 {
		return nil, errors.New("at least one domain required (or - to read from stdin)")
	}
	return a, nil
}

// params accepts key=value, --key=value and --key value. In the last form a
// following token that starts with "-" is not taken as the value, so a
// valueless --a cannot swallow the next --b=c.
func params(q url.Values, a []string) error {
	for i := 0; i < len(a); i++ {
		s := strings.TrimLeft(a[i], "-")
		if k, v, ok := strings.Cut(s, "="); ok && k != "" {
			q.Add(k, v)
			continue
		}
		if strings.HasPrefix(a[i], "-") && s != "" {
			if i+1 < len(a) && !strings.HasPrefix(a[i+1], "-") {
				q.Add(s, a[i+1])
				i++
				continue
			}
			return fmt.Errorf("parameter %q needs a value (use key=value for values that start with -)", a[i])
		}
		return fmt.Errorf("bad parameter %q (use key=value)", a[i])
	}
	return nil
}

func send(r *request, out string) int {
	base := strings.TrimRight(getenv("DOMAINKITS_API_URL", defaultBase), "/")
	u := base + "/" + r.path
	if enc := r.query.Encode(); enc != "" {
		u += "?" + enc
	}

	var body io.Reader
	if r.body != nil {
		b, err := json.Marshal(r.body)
		if err != nil {
			return fail(2, "invalid JSON body: "+err.Error())
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(r.method, u, body)
	if err != nil {
		return fail(2, err.Error())
	}
	req.Header.Set("User-Agent", "dk-cli/"+version)
	req.Header.Set("Accept", "application/json")
	if r.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if r.auth {
		key := os.Getenv("DOMAINKITS_API_KEY")
		if key == "" {
			return fail(2, "DOMAINKITS_API_KEY is not set")
		}
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fail(2, "request failed: "+err.Error())
	}
	defer resp.Body.Close()

	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	if ok {
		if name := fileName(r, resp, out); name != "" {
			return save(name, resp.Body)
		}
	}
	if err := copyBody(os.Stdout, resp.Body); err != nil {
		return fail(2, "reading response: "+err.Error())
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "dk: HTTP %d%s\n", resp.StatusCode, retryHint(resp))
		return 1
	}
	return 0
}

// retryHint passes on when the server says to come back: Retry-After as
// given, and on 429 the time until the per-minute window resets.
func retryHint(resp *http.Response) string {
	var parts []string
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		parts = append(parts, "Retry-After: "+ra)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		if sec, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			if wait := time.Until(time.Unix(sec, 0)).Round(time.Second); wait > 0 {
				parts = append(parts, "rate limit window resets in "+wait.String())
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, "; ") + ")"
}

// fileName decides whether the response goes to a file: an explicit -o
// wins; otherwise an attachment uses the server's file name, and a file
// endpoint without one is named after the final URL.
func fileName(r *request, resp *http.Response, out string) string {
	if out != "" {
		return out
	}
	if _, p, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil && p["filename"] != "" {
		return filepath.Base(p["filename"])
	}
	if r.toFile && resp.Request != nil {
		if n := filepath.Base(resp.Request.URL.Path); n != "." && n != "/" && n != "download" {
			return n
		}
		return "download.bin"
	}
	return ""
}

// save writes to a temporary file next to the target and renames it into
// place only once the body has arrived in full, so a broken transfer does
// not leave a partial file under the final name.
func save(name string, body io.Reader) int {
	tmp, err := os.CreateTemp(filepath.Dir(name), ".dk-*.part")
	if err != nil {
		return fail(2, err.Error())
	}
	n, err := io.Copy(tmp, body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), name)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return fail(2, "writing "+name+": "+err.Error())
	}

	abs, _ := filepath.Abs(name)
	b, _ := json.Marshal(struct {
		Saved string `json:"saved"`
		Bytes int64  `json:"bytes"`
	}{abs, n})
	fmt.Println(string(b))
	return 0
}

// copyBody streams the body and guarantees a trailing newline.
func copyBody(w io.Writer, body io.Reader) error {
	var last byte
	buf := make([]byte, 32*1024)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			last = buf[n-1]
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if last != 0 && last != '\n' {
		_, _ = w.Write([]byte{'\n'})
	}
	return nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fail(code int, msg string) int {
	fmt.Fprintln(os.Stderr, "dk: "+msg)
	return code
}

const usageText = `dk - DomainKits API client (v` + version + `)

Usage: dk <command> [arguments] [key=value ...] [-o FILE]

Commands
  search <type> [params]        type: nrds, aged, active, expired, deleted,
                                market, hostname, status
  whois <domain>                dns <domain>
  bulk whois <domain ...>       bulk dns <domain ...>   (or "-" to read stdin)
  ns-reverse <ns[,ns2...]>      domains using all of the nameservers
  ip-lookup <ip|domain>         registrar <name|iana-id>   status-guide <code>
  tld-check <prefix> [params]   typosquat <domain> [params]
  ct subdomains <domain>        ct certs <domain>
  trends tlds <active|newly>    trends keywords <hot|emerging|prefix>
  monitor [params]              recent domain changes
  download tld=X type=Y date=YYYY-MM-DD     NRDS daily file
  usage                         your quota and usage      health   service status
  get <path> [params]           any GET endpoint, e.g. dk get search/nrds tld=com
  post <path> <json|->          any POST endpoint

Parameters
  Everything after the positional arguments is passed to the API as-is:
  key=value, --key=value or --key value (a value that starts with - needs
  key=value). Unknown names and bad values are rejected by the server and
  its message is printed. Every endpoint, parameter and limit:
  https://domainkits.com/dev/api-docs

Output
  The response body is printed unchanged (JSON). export=csv or export=txt
  and download write a file instead and print {"saved":...,"bytes":...}.
  A transfer that breaks off leaves nothing under the target name.
  -o FILE forces the file name.
  Exit code: 0 success, 1 API error (body printed; Retry-After and the
  rate limit reset go to stderr), 2 usage or network error.

Environment
  DOMAINKITS_API_KEY   your API key (required for everything except health)
  DOMAINKITS_API_URL   override the API base URL (default ` + defaultBase + `)

Examples
  dk search nrds query=ai tld=com limit=20
  dk search expired tld=com age_min=5 sort=age_desc export=csv
  dk whois example.com
  dk bulk whois example.com example.org
  dk ns-reverse ns1.example.com,ns2.example.com tld=com
  dk ct subdomains github.com
  dk trends keywords hot scope=com
`
