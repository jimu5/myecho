package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxResponseBytes = 64 << 20

// Keep aligned with service.MaxContentArchiveBytes without importing service into this HTTP-only command.
const maxContentArchiveBytes int64 = 512 << 20

type client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

func newClient(rawURL, token string) (*client, error) {
	base, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("MYECHO_URL must be an http(s) base URL without credentials, query, or fragment")
	}
	base.Path = strings.TrimRight(base.Path, "/")
	return &client{
		baseURL: base,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *client) endpoint(path string) string {
	target, _ := url.Parse(path)
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + target.Path
	u.RawPath = ""
	u.RawQuery = target.RawQuery
	return u.String()
}

func (c *client) auth(req *http.Request) {
	if c.token == "" {
		return
	}
	scheme := "token"
	if strings.HasPrefix(c.token, "mye_") {
		scheme = "Bearer"
	}
	req.Header.Set("Authorization", scheme+" "+c.token)
}

func (c *client) request(method, target, contentType, idempotencyKey string, body io.Reader) (json.RawMessage, error) {
	req, err := http.NewRequest(method, c.endpoint(target), body)
	if err != nil {
		return nil, err
	}
	c.auth(req)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("server response exceeds %d bytes", maxResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(resp.StatusCode, data, c.token)
	}
	var envelope struct {
		Code *int   `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("server returned invalid JSON: %w", err)
	}
	if envelope.Code == nil {
		return nil, errors.New("server response is missing API code")
	}
	if *envelope.Code != 0 {
		return nil, fmt.Errorf("API code %d: %s", *envelope.Code, redact(envelope.Msg, c.token))
	}
	return json.RawMessage(data), nil
}

func responseError(status int, body []byte, token string) error {
	var envelope struct {
		Code *int   `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Msg != "" {
		if envelope.Code != nil {
			return fmt.Errorf("HTTP %d, API code %d: %s", status, *envelope.Code, redact(envelope.Msg, token))
		}
		return fmt.Errorf("HTTP %d: %s", status, redact(envelope.Msg, token))
	}
	return fmt.Errorf("HTTP %d", status)
}

func redact(message, token string) string {
	if token != "" {
		message = strings.ReplaceAll(message, token, "[redacted]")
	}
	return strings.TrimSpace(message)
}

func emit(out io.Writer, data json.RawMessage) error {
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return err
	}
	_, err := fmt.Fprintln(out, compact.String())
	return err
}

func jsonBody(value any) (*bytes.Reader, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (c *client) json(method, target string, value any, idempotencyKey string) (json.RawMessage, error) {
	var body io.Reader
	contentType := ""
	if value != nil {
		encoded, err := jsonBody(value)
		if err != nil {
			return nil, err
		}
		body, contentType = encoded, "application/json"
	}
	return c.request(method, target, contentType, idempotencyKey, body)
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

func run(args []string, out, errOut io.Writer, getenv func(string) string) int {
	global := flag.NewFlagSet("myecho", flag.ContinueOnError)
	var globalHelp bytes.Buffer
	global.SetOutput(&globalHelp)
	baseURL := global.String("url", getenv("MYECHO_URL"), "myecho server base URL (or MYECHO_URL)")
	global.Usage = func() { printUsage(&globalHelp) }
	if err := global.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.Copy(out, &globalHelp)
			return 0
		}
		fmt.Fprintf(errOut, "myecho: %s\n", err)
		return 2
	}
	args = global.Args()
	if len(args) == 0 {
		printUsage(out)
		return 2
	}
	if hasHelp(args) {
		if err := execute(nil, args, out); err != nil {
			fmt.Fprintf(errOut, "myecho: %s\n", err)
			return 2
		}
		return 0
	}
	if *baseURL == "" {
		fmt.Fprintln(errOut, "myecho: set MYECHO_URL or pass --url")
		return 2
	}
	c, err := newClient(*baseURL, getenv("MYECHO_TOKEN"))
	if err != nil {
		fmt.Fprintf(errOut, "myecho: %s\n", err)
		return 2
	}
	if err := execute(c, args, out); err != nil {
		fmt.Fprintf(errOut, "myecho: %s\n", redact(err.Error(), c.token))
		return 1
	}
	return 0
}

func hasHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, `myecho [--url URL] <command>

Environment:
  MYECHO_URL    Server base URL, for example http://localhost:2999
  MYECHO_TOKEN  Existing user token or mye_ agent token
Options:
  --url URL     Override MYECHO_URL (place it before the command)

Commands:
  article list|get|create|save|submit|publish|preview|revisions|restore-revision|trash|restore-trash
  category list
  tag list
  media list|upload
  token list|create|revoke
  content export|import

Create requires a caller-supplied stable idempotency key. Edits use fresh edit versions and never retry conflicts.
Save defaults to draft (status 4); submit uses 5; publish uses 1. Import previews unless --apply is set.
Use --help after a command for its flags. Responses are JSON envelopes on stdout; errors go to stderr.`)
}

func execute(c *client, args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New("expected a resource and command; use --help for usage")
	}
	switch args[0] {
	case "article":
		return articleCommand(c, args[1], args[2:], out)
	case "category":
		if args[1] != "list" {
			return errors.New("category supports: list")
		}
		return listCommand(c, "/api/article/categories/all", args[2:], out)
	case "tag":
		if args[1] != "list" {
			return errors.New("tag supports: list")
		}
		return listCommand(c, "/api/tags/all", args[2:], out)
	case "media":
		return mediaCommand(c, args[1], args[2:], out)
	case "token":
		return tokenCommand(c, args[1], args[2:], out)
	case "content":
		return contentCommand(c, args[1], args[2:], out)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func commandFlags(name string, _ io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s\n", name)
		fs.PrintDefaults()
	}
	return fs
}

func parseCommandFlags(fs *flag.FlagSet, args []string, out io.Writer) (bool, error) {
	var parserOutput bytes.Buffer
	fs.SetOutput(&parserOutput)
	err := fs.Parse(args)
	fs.SetOutput(io.Discard)
	if errors.Is(err, flag.ErrHelp) {
		_, _ = io.Copy(out, &parserOutput)
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if fs.NArg() > 0 {
		return false, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return false, nil
}

func listCommand(c *client, target string, args []string, out io.Writer) error {
	fs := commandFlags("myecho list [--page N] [--page-size N]", out)
	page := fs.Int("page", 1, "page number")
	pageSize := fs.Int("page-size", 10, "rows per page (1-100)")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if *page < 1 || *pageSize < 1 || *pageSize > 100 {
		return errors.New("page must be positive and page-size must be between 1 and 100")
	}
	query := url.Values{"page": {strconv.Itoa(*page)}, "page_size": {strconv.Itoa(*pageSize)}}
	response, err := c.request(http.MethodGet, target+"?"+query.Encode(), "", "", nil)
	if err != nil {
		return err
	}
	return emit(out, response)
}

func articleCommand(c *client, command string, args []string, out io.Writer) error {
	switch command {
	case "list":
		return articleList(c, args, out)
	case "get":
		return articleGet(c, args, out)
	case "create":
		return articleCreate(c, args, out)
	case "save", "submit", "publish":
		return articleEdit(c, command, args, out)
	case "preview":
		return articlePreview(c, args, out)
	case "revisions":
		return articleRevisions(c, args, out)
	case "restore-revision":
		return articleRestoreRevision(c, args, out)
	case "trash":
		return articleTrash(c, args, out)
	case "restore-trash":
		return articleRestoreTrash(c, args, out)
	default:
		return fmt.Errorf("unknown article command %q", command)
	}
}

func articleList(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho article list [filters]", out)
	page := fs.Int("page", 1, "page number")
	pageSize := fs.Int("page-size", 10, "rows per page (1-100)")
	status := fs.String("status", "", "article status")
	keyword := fs.String("keyword", "", "search title/content")
	categoryUID := fs.String("category-uid", "", "category UID")
	tagUID := fs.String("tag-uid", "", "tag UID")
	authorID := fs.String("author-id", "", "author ID")
	typeName := fs.String("type", "", "post or page")
	dateFrom := fs.String("date-from", "", "inclusive start date")
	dateTo := fs.String("date-to", "", "inclusive end date")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if *page < 1 || *pageSize < 1 || *pageSize > 100 {
		return errors.New("page must be positive and page-size must be between 1 and 100")
	}
	query := url.Values{"page": {strconv.Itoa(*page)}, "page_size": {strconv.Itoa(*pageSize)}}
	for _, filter := range []struct{ key, value string }{
		{"status", *status}, {"keyword", *keyword}, {"category_uid", *categoryUID},
		{"tag_uid", *tagUID}, {"author_id", *authorID}, {"date_from", *dateFrom}, {"date_to", *dateTo},
	} {
		key, value := filter.key, filter.value
		if value != "" {
			query.Set(key, value)
		}
	}
	if *typeName != "" {
		switch *typeName {
		case "post":
			query.Set("type", "1")
		case "page":
			query.Set("type", "2")
		default:
			return errors.New("type must be post or page")
		}
	}
	response, err := c.request(http.MethodGet, "/api/all_articles?"+query.Encode(), "", "", nil)
	if err != nil {
		return err
	}
	return emit(out, response)
}

func articleGet(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho article get --id ID", out)
	id := fs.String("id", "", "article ID")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if _, err := numericID(*id); err != nil {
		return err
	}
	response, err := c.request(http.MethodGet, "/api/articles/"+*id+"?no_read=true", "", "", nil)
	if err != nil {
		return err
	}
	return emit(out, response)
}

func articleCreate(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho article create --file article.md --title TITLE --idempotency-key KEY", out)
	file := fs.String("file", "", "Markdown content file")
	title := fs.String("title", "", "article title")
	slug := fs.String("slug", "", "article slug")
	categoryUID := fs.String("category-uid", "", "category UID")
	var tags stringList
	fs.Var(&tags, "tag-uid", "tag UID (repeatable)")
	summary := fs.String("summary", "", "article summary")
	seoTitle := fs.String("seo-title", "", "SEO title")
	seoDescription := fs.String("seo-description", "", "SEO description")
	shareImage := fs.String("share-image", "", "share image URL")
	typeName := fs.String("type", "post", "post or page")
	contentFormat := fs.String("content-format", "markdown", "markdown or html")
	status := fs.Int("status", 4, "article status (default: draft 4)")
	data := fs.String("data", "", "additional ArticleRequest JSON object")
	idempotencyKey := fs.String("idempotency-key", "", "stable key reused for retries of this exact create request")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if strings.TrimSpace(*idempotencyKey) == "" {
		return errors.New("--idempotency-key is required; reuse it only with the same request body")
	}
	if *file == "" {
		return errors.New("--file is required")
	}
	content, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("read content file: %w", err)
	}
	payload, err := jsonObject(*data)
	if err != nil {
		return err
	}
	payload["content"] = string(content)
	if *title != "" {
		payload["title"] = *title
	}
	for key, value := range map[string]string{
		"slug": *slug, "category_uid": *categoryUID, "summary": *summary,
		"seo_title": *seoTitle, "seo_description": *seoDescription, "share_image": *shareImage,
	} {
		if value != "" {
			payload[key] = value
		}
	}
	if len(tags) > 0 {
		payload["tag_uids"] = tags
	}
	if *typeName != "post" && *typeName != "page" {
		return errors.New("type must be post or page")
	}
	if *contentFormat != "markdown" && *contentFormat != "html" {
		return errors.New("content-format must be markdown or html")
	}
	payload["type"] = map[string]int{"post": 1, "page": 2}[*typeName]
	payload["content_format"] = *contentFormat
	if flagWasSet(fs, "status") || payload["status"] == nil {
		payload["status"] = *status
	}
	response, err := c.json(http.MethodPost, "/api/articles", payload, *idempotencyKey)
	if err != nil {
		return err
	}
	return emit(out, response)
}

func articleEdit(c *client, command string, args []string, out io.Writer) error {
	fs := commandFlags("myecho article "+command+" --id ID [--file article.md] [--data JSON]", out)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s\n", fs.Name())
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), "  save writes draft status 4; submit writes 5; publish writes 1. Each reads a fresh edit_version and sends one PATCH without retrying conflicts.")
		fmt.Fprintln(fs.Output(), "  Pin --expected-edit-version from an earlier read to abort if the article changed before this command.")
	}
	id := fs.String("id", "", "article ID")
	file := fs.String("file", "", "Markdown content file")
	data := fs.String("data", "", "ArticleRequest metadata JSON patch")
	expectedVersion := fs.String("expected-edit-version", "", "abort unless current version matches this value")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if _, err := numericID(*id); err != nil {
		return err
	}
	if command == "save" && *file == "" && *data == "" {
		return errors.New("provide --file and/or --data")
	}
	patch, err := jsonObject(*data)
	if err != nil {
		return err
	}
	pinnedVersion, pinned := uint64(0), false
	if *expectedVersion != "" {
		pinnedVersion, err = strconv.ParseUint(*expectedVersion, 10, 64)
		if err != nil || pinnedVersion == 0 {
			return errors.New("--expected-edit-version must be a positive integer")
		}
		pinned = true
	}
	if value, ok := patch["expected_edit_version"]; ok {
		dataVersion, versionErr := versionValue(value)
		if versionErr != nil {
			return fmt.Errorf("expected_edit_version in --data: %w", versionErr)
		}
		if pinned && dataVersion != pinnedVersion {
			return errors.New("--expected-edit-version conflicts with --data expected_edit_version")
		}
		pinnedVersion, pinned = dataVersion, true
		delete(patch, "expected_edit_version")
	}
	if *file != "" {
		content, err := os.ReadFile(*file)
		if err != nil {
			return fmt.Errorf("read content file: %w", err)
		}
		patch["content"] = string(content)
		patch["content_format"] = "markdown"
	}
	request, version, err := currentArticleRequest(c, *id)
	if err != nil {
		return err
	}
	if pinned && pinnedVersion != version {
		return fmt.Errorf("article edit_version changed: expected %d, found %d; refresh before editing", pinnedVersion, version)
	}
	for key, value := range patch {
		request[key] = value
	}
	request["status"] = map[string]int{"save": 4, "submit": 5, "publish": 1}[command]
	request["expected_edit_version"] = version
	delete(request, "autosave")
	response, err := c.json(http.MethodPatch, "/api/articles/"+*id, request, "")
	if err != nil {
		return err
	}
	return emit(out, response)
}

func versionValue(value any) (uint64, error) {
	var raw string
	switch v := value.(type) {
	case json.Number:
		raw = v.String()
	case string:
		raw = v
	default:
		return 0, errors.New("must be a positive integer")
	}
	version, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || version == 0 {
		return 0, errors.New("must be a positive integer")
	}
	return version, nil
}

func currentArticleRequest(c *client, id string) (map[string]any, uint64, error) {
	response, err := c.request(http.MethodGet, "/api/articles/"+id+"?no_read=true", "", "", nil)
	if err != nil {
		return nil, 0, err
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, 0, fmt.Errorf("decode current article: %w", err)
	}
	article := envelope.Data
	if article == nil {
		return nil, 0, errors.New("article detail response has no data")
	}
	version, err := uintField(article, "edit_version")
	if err != nil || version == 0 {
		return nil, 0, errors.New("article detail response has no valid edit_version")
	}
	request := map[string]any{}
	for _, key := range []string{
		"title", "slug", "seo_title", "seo_description", "share_image", "type", "content_format",
		"summary", "category_uid", "is_allow_comment", "post_time", "status", "visibility",
	} {
		if value, ok := article[key]; ok {
			request[key] = value
		}
	}
	detail, _ := article["detail"].(map[string]any)
	if detail != nil {
		if content, ok := detail["content"]; ok {
			request["content"] = content
		}
	}
	if _, ok := request["content"]; !ok {
		return nil, 0, errors.New("article detail response has no content")
	}
	if tags, ok := article["tags"].([]any); ok {
		tagUIDs := make([]string, 0, len(tags))
		for _, value := range tags {
			if tag, ok := value.(map[string]any); ok {
				if uid, ok := tag["uid"].(string); ok {
					tagUIDs = append(tagUIDs, uid)
				}
			}
		}
		request["tag_uids"] = tagUIDs
	}
	if submission, ok := article["submission"].(map[string]any); ok {
		status, _ := uintField(submission, "status")
		if status != 2 { // Pending submissions cannot be edited by the caller.
			if snapshot, ok := submission["snapshot"].(map[string]any); ok {
				for _, key := range []string{
					"title", "slug", "seo_title", "seo_description", "share_image", "type", "content_format",
					"summary", "content", "category_uid", "is_allow_comment", "post_time", "tag_uids",
				} {
					if value, ok := snapshot[key]; ok {
						request[key] = value
					}
				}
			}
		}
	}
	request["expected_edit_version"] = version
	return request, version, nil
}

func uintField(object map[string]any, key string) (uint64, error) {
	value, ok := object[key]
	if !ok {
		return 0, fmt.Errorf("%s is missing", key)
	}
	switch v := value.(type) {
	case json.Number:
		return strconv.ParseUint(v.String(), 10, 64)
	case float64:
		if v < 0 || v != float64(uint64(v)) {
			return 0, fmt.Errorf("%s is invalid", key)
		}
		return uint64(v), nil
	default:
		return 0, fmt.Errorf("%s is invalid", key)
	}
}

func jsonObject(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode --data JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("--data must contain exactly one JSON object")
	}
	if value == nil {
		return nil, errors.New("--data must be a JSON object")
	}
	return value, nil
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func numericID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("--id must be a positive integer")
	}
	return id, nil
}

func articlePreview(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho article preview --id ID", out)
	id := fs.String("id", "", "article ID")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if _, err := numericID(*id); err != nil {
		return err
	}
	_, version, err := currentArticleRequest(c, *id)
	if err != nil {
		return err
	}
	response, err := c.json(http.MethodPost, "/api/articles/"+*id+"/previews", map[string]any{"expected_edit_version": version}, "")
	if err != nil {
		return err
	}
	return emit(out, response)
}

func articleRevisions(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho article revisions --id ID", out)
	id := fs.String("id", "", "article ID")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if _, err := numericID(*id); err != nil {
		return err
	}
	response, err := c.request(http.MethodGet, "/api/articles/"+*id+"/revisions", "", "", nil)
	if err != nil {
		return err
	}
	return emit(out, response)
}

func articleRestoreRevision(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho article restore-revision --id ID --revision-id ID", out)
	id := fs.String("id", "", "article ID")
	revisionID := fs.String("revision-id", "", "revision ID")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if _, err := numericID(*id); err != nil {
		return err
	}
	if _, err := numericID(*revisionID); err != nil {
		return errors.New("--revision-id must be a positive integer")
	}
	response, err := c.json(http.MethodPost, "/api/articles/"+*id+"/revisions/"+*revisionID+"/restore", nil, "")
	if err != nil {
		return err
	}
	return emit(out, response)
}

func articleTrash(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho article trash --id ID", out)
	id := fs.String("id", "", "article ID")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if _, err := numericID(*id); err != nil {
		return err
	}
	response, err := c.json(http.MethodDelete, "/api/articles/"+*id, nil, "")
	if err != nil {
		return err
	}
	return emit(out, response)
}

func articleRestoreTrash(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho article restore-trash --id ID", out)
	id := fs.String("id", "", "article ID")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if _, err := numericID(*id); err != nil {
		return err
	}
	response, err := c.json(http.MethodPost, "/api/articles/"+*id+"/restore", nil, "")
	if err != nil {
		return err
	}
	return emit(out, response)
}

func mediaCommand(c *client, command string, args []string, out io.Writer) error {
	switch command {
	case "list":
		return mediaList(c, args, out)
	case "upload":
		return mediaUpload(c, args, out)
	default:
		return fmt.Errorf("unknown media command %q", command)
	}
}

func mediaList(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho media list [--page N] [--page-size N]", out)
	page := fs.Int("page", 1, "page number")
	pageSize := fs.Int("page-size", 10, "rows per page (1-100)")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if *page < 1 || *pageSize < 1 || *pageSize > 100 {
		return errors.New("page must be positive and page-size must be between 1 and 100")
	}
	query := url.Values{"page": {strconv.Itoa(*page)}, "page_size": {strconv.Itoa(*pageSize)}}
	response, err := c.request(http.MethodGet, "/mos/files?"+query.Encode(), "", "", nil)
	if err != nil {
		return err
	}
	return emit(out, response)
}

func mediaUpload(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho media upload --file PATH", out)
	filePath := fs.String("file", "", "file to upload")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if *filePath == "" {
		return errors.New("--file is required")
	}
	response, err := c.multipartFile(http.MethodPost, "/mos/files/upload", *filePath, nil, 0)
	if err != nil {
		return err
	}
	return emit(out, response)
}

func contentCommand(c *client, command string, args []string, out io.Writer) error {
	switch command {
	case "export":
		return contentExport(c, args, out)
	case "import":
		return contentImport(c, args, out)
	default:
		return fmt.Errorf("unknown content command %q", command)
	}
}

func contentExport(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho content export --output archive.zip [--include-media]", out)
	output := fs.String("output", "", "output ZIP path (must not exist)")
	includeMedia := fs.Bool("include-media", false, "include referenced media files")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if *output == "" {
		return errors.New("--output is required")
	}
	query := url.Values{"include_media": {strconv.FormatBool(*includeMedia)}}
	count, err := c.download("/api/content/export?"+query.Encode(), *output, maxContentArchiveBytes)
	if err != nil {
		return err
	}
	return writeJSON(out, map[string]any{
		"code": 0, "msg": "ok", "data": map[string]any{"output": *output, "bytes": count}, "meta": map[string]any{},
	})
}

func contentImport(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho content import --file archive.zip [--apply] [--author-map JSON]", out)
	file := fs.String("file", "", "portable content ZIP")
	apply := fs.Bool("apply", false, "apply the import; without this flag only preview")
	authorMap := fs.String("author-map", "", "JSON object mapping archive author names to user IDs (requires --apply)")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if *file == "" {
		return errors.New("--file is required")
	}
	fields := map[string]string{}
	if *authorMap != "" {
		if !*apply {
			return errors.New("--author-map requires --apply")
		}
		object, err := jsonObject(*authorMap)
		if err != nil {
			return err
		}
		ids := make(map[string]uint64, len(object))
		for author, value := range object {
			if strings.TrimSpace(author) == "" {
				return errors.New("author-map keys must not be empty")
			}
			id, err := versionValue(value)
			if err != nil {
				return fmt.Errorf("author-map value for %q: %w", author, err)
			}
			ids[author] = id
		}
		data, err := json.Marshal(ids)
		if err != nil {
			return err
		}
		fields["author_map"] = string(data)
	}
	dryRun := !*apply
	query := url.Values{"dry_run": {strconv.FormatBool(dryRun)}}
	response, err := c.multipartFile(http.MethodPost, "/api/content/import?"+query.Encode(), *file, fields, maxContentArchiveBytes)
	if err != nil {
		return err
	}
	return emit(out, response)
}

func (c *client) multipartFile(method, target, filePath string, fields map[string]string, maxFileBytes int64) (json.RawMessage, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open upload file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() <= 0 {
		return nil, errors.New("upload file must not be empty")
	}
	if maxFileBytes > 0 && info.Size() > maxFileBytes {
		return nil, fmt.Errorf("upload file exceeds %d bytes", maxFileBytes)
	}
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	contentType := form.FormDataContentType()
	writeDone := make(chan error, 1)
	go func() {
		part, writeErr := form.CreateFormFile("file", filepath.Base(filePath))
		if writeErr == nil {
			var source io.Reader = file
			if maxFileBytes > 0 {
				source = io.LimitReader(file, maxFileBytes+1)
			}
			var count int64
			count, writeErr = io.Copy(part, source)
			if writeErr == nil && maxFileBytes > 0 && count > maxFileBytes {
				writeErr = fmt.Errorf("upload file exceeds %d bytes", maxFileBytes)
			}
		}
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if writeErr == nil {
			for _, key := range keys {
				if writeErr = form.WriteField(key, fields[key]); writeErr != nil {
					break
				}
			}
		}
		if closeErr := form.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			_ = writer.CloseWithError(writeErr)
		} else {
			_ = writer.Close()
		}
		writeDone <- writeErr
	}()
	response, requestErr := c.request(method, target, contentType, "", reader)
	_ = reader.Close()
	writeErr := <-writeDone
	if requestErr != nil {
		return nil, requestErr
	}
	if writeErr != nil {
		return nil, writeErr
	}
	return response, nil
}

func (c *client) download(target, output string, limit int64) (int64, error) {
	if _, err := os.Lstat(output); err == nil {
		return 0, fmt.Errorf("output path %q already exists", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodGet, c.endpoint(target), nil)
	if err != nil {
		return 0, err
	}
	c.auth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		if readErr != nil {
			return 0, readErr
		}
		return 0, responseError(resp.StatusCode, data, c.token)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/zip" {
		return 0, fmt.Errorf("content export returned unexpected content type %q", resp.Header.Get("Content-Type"))
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 0, err
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(output)
		}
	}()
	count, err := io.Copy(file, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return 0, err
	}
	if count > limit {
		return 0, fmt.Errorf("content export exceeds %d bytes", limit)
	}
	if err := file.Close(); err != nil {
		return 0, err
	}
	keep = true
	return count, nil
}

func writeJSON(out io.Writer, value any) error {
	return json.NewEncoder(out).Encode(value)
}

func tokenCommand(c *client, command string, args []string, out io.Writer) error {
	switch command {
	case "list":
		fs := commandFlags("myecho token list", out)
		if help, err := parseCommandFlags(fs, args, out); help || err != nil {
			return err
		}
		response, err := c.request(http.MethodGet, "/api/account/tokens", "", "", nil)
		if err != nil {
			return err
		}
		return emit(out, response)
	case "create":
		return tokenCreate(c, args, out)
	case "revoke":
		return tokenRevoke(c, args, out)
	default:
		return fmt.Errorf("unknown token command %q", command)
	}
}

func tokenCreate(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho token create --name NAME --scopes scope1,scope2 [--expires-in-days N]", out)
	name := fs.String("name", "", "token name")
	scopesRaw := fs.String("scopes", "", "comma-separated token scopes")
	expiresInDays := fs.Int("expires-in-days", 0, "expiry in days (default: 90; max: 365)")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*scopesRaw) == "" {
		return errors.New("--name and --scopes are required")
	}
	if *expiresInDays < 0 || *expiresInDays > 365 {
		return errors.New("expires-in-days must be between 1 and 365, or 0/omitted for the 90-day default")
	}
	scopes := make([]string, 0)
	for _, scope := range strings.Split(*scopesRaw, ",") {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			return errors.New("scopes must be comma-separated non-empty values")
		}
		scopes = append(scopes, scope)
	}
	payload := map[string]any{"name": *name, "scopes": scopes}
	if *expiresInDays > 0 {
		payload["expires_in_days"] = *expiresInDays
	}
	response, err := c.json(http.MethodPost, "/api/account/tokens", payload, "")
	if err != nil {
		return err
	}
	return emit(out, response)
}

func tokenRevoke(c *client, args []string, out io.Writer) error {
	fs := commandFlags("myecho token revoke --id ID", out)
	id := fs.String("id", "", "token ID")
	if help, err := parseCommandFlags(fs, args, out); help || err != nil {
		return err
	}
	if _, err := numericID(*id); err != nil {
		return err
	}
	response, err := c.json(http.MethodDelete, "/api/account/tokens/"+*id, nil, "")
	if err != nil {
		return err
	}
	return emit(out, response)
}
