package update

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultOwner = "junaidrehman04"
	defaultRepo  = "altas-panel"
	githubAPI    = "https://api.github.com"
	maxBinary    = 64 << 20
	maxMeta      = 1 << 20
)

// ErrSource means the official release API could not be read.
// The updater does not fall back to any other URL.
var ErrSource = errors.New("release source")

// ErrNoRelease means the pinned repository has not published a release.
var ErrNoRelease = errors.New("no official release")

// ErrAsset means the release did not contain the asset for this OS and architecture.
var ErrAsset = errors.New("release asset")

// Release is one official GitHub release asset plus its checksum file.
type Release struct {
	Tag       string
	Name      string
	Binary    []byte
	Checksums []byte
	Signature []byte
}

// ReleaseSource fetches the latest official release.
type ReleaseSource interface {
	FetchLatest(ctx context.Context) (Release, error)
}

// Client talks only to the GitHub Releases API for one owner and repository.
// Asset bytes are downloaded from the API asset route built here. URLs in
// the JSON body are ignored.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	Owner   string
	Repo    string
	OS      string
	Arch    string
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// FetchLatest reads the latest release and the checksummed binary for this OS.
func (c *Client) FetchLatest(ctx context.Context) (Release, error) {
	if c == nil {
		return Release{}, ErrSource
	}
	owner := c.Owner
	if owner == "" {
		owner = defaultOwner
	}
	repo := c.Repo
	if repo == "" {
		repo = defaultRepo
	}
	if !validRepoName(owner) || !validRepoName(repo) {
		return Release{}, ErrSource
	}
	name, err := BinaryAssetName(c.OS, c.Arch)
	if err != nil {
		return Release{}, err
	}
	base, err := parseBase(c.BaseURL)
	if err != nil {
		return Release{}, err
	}
	latest := *base
	latest.Path = "/repos/" + owner + "/" + repo + "/releases/latest"
	latest.RawQuery = ""
	latest.Fragment = ""

	body, err := c.get(ctx, &latest, "application/vnd.github+json", maxMeta)
	if err != nil {
		return Release{}, err
	}
	var parsed ghRelease
	if err := json.Unmarshal(body, &parsed); err != nil || !safeTag(parsed.TagName) {
		return Release{}, ErrSource
	}
	bin, sums, sig, err := pickAssets(parsed.Assets, name)
	if err != nil {
		return Release{}, err
	}
	binary, err := c.downloadAsset(ctx, base, owner, repo, bin, maxBinary)
	if err != nil {
		return Release{}, err
	}
	checksums, err := c.downloadAsset(ctx, base, owner, repo, sums, maxMeta)
	if err != nil {
		return Release{}, err
	}
	var signature []byte
	if sig.ID > 0 {
		signature, err = c.downloadAsset(ctx, base, owner, repo, sig, maxMeta)
		if err != nil {
			return Release{}, err
		}
	}
	return Release{
		Tag:       parsed.TagName,
		Name:      name,
		Binary:    binary,
		Checksums: checksums,
		Signature: signature,
	}, nil
}

func pickAssets(assets []ghAsset, binaryName string) (bin, sums, sig ghAsset, err error) {
	rankOf := map[string]int{
		"SHA256SUMS":     3,
		"SHA256SUMS.txt": 2,
		"checksums.txt":  1,
	}
	best := 0
	for _, asset := range assets {
		if asset.ID <= 0 || !safeAssetName(asset.Name) {
			continue
		}
		switch {
		case asset.Name == binaryName:
			bin = asset
		case asset.Name == binaryName+".ed25519":
			sig = asset
		default:
			if rank := rankOf[asset.Name]; rank > best {
				sums = asset
				best = rank
			}
		}
	}
	if bin.ID == 0 || sums.ID == 0 {
		return ghAsset{}, ghAsset{}, ghAsset{}, ErrAsset
	}
	if bin.Size > maxBinary || sums.Size > maxMeta || (sig.ID > 0 && sig.Size > maxMeta) {
		return ghAsset{}, ghAsset{}, ghAsset{}, ErrSource
	}
	return bin, sums, sig, nil
}

func (c *Client) downloadAsset(ctx context.Context, base *url.URL, owner, repo string, asset ghAsset, limit int64) ([]byte, error) {
	u := *base
	u.Path = "/repos/" + owner + "/" + repo + "/releases/assets/" + strconv.FormatInt(asset.ID, 10)
	u.RawQuery = ""
	u.Fragment = ""
	return c.get(ctx, &u, "application/octet-stream", limit)
}

func (c *Client) get(ctx context.Context, u *url.URL, accept string, limit int64) ([]byte, error) {
	base, err := parseBase(c.BaseURL)
	if err != nil {
		return nil, err
	}
	if err := allowFetchURL(u, base); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrSource
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "altas-panel")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.http(base).Do(req)
	if err != nil {
		return nil, ErrSource
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound && strings.HasSuffix(u.Path, "/releases/latest") {
		return nil, ErrNoRelease
	}
	if res.StatusCode != http.StatusOK {
		return nil, ErrSource
	}
	if res.ContentLength > limit {
		return nil, ErrSource
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, ErrSource
	}
	if int64(len(body)) > limit {
		return nil, ErrSource
	}
	return body, nil
}

func (c *Client) http(base *url.URL) *http.Client {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if c.HTTP.CheckRedirect == nil {
		c.HTTP.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return ErrSource
			}
			return allowFetchURL(req.URL, base)
		}
	}
	return c.HTTP
}

func parseBase(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		raw = githubAPI
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return nil, ErrSource
	}
	if u.Path != "" && u.Path != "/" {
		return nil, ErrSource
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, ErrSource
	}
	if u.Scheme == "http" && !strings.HasPrefix(u.Host, "127.0.0.1") && u.Hostname() != "localhost" {
		return nil, ErrSource
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}

func allowFetchURL(u *url.URL, base *url.URL) error {
	if u == nil || base == nil || u.Host == "" || u.User != nil {
		return ErrSource
	}
	if strings.Contains(u.Path, "..") {
		return ErrSource
	}
	host := strings.ToLower(u.Hostname())
	baseHost := strings.ToLower(base.Hostname())
	if u.Scheme == "http" {
		if base.Scheme != "http" || host != baseHost {
			return ErrSource
		}
		return nil
	}
	if u.Scheme != "https" {
		return ErrSource
	}
	if host == baseHost {
		return nil
	}
	if baseHost == "api.github.com" && githubCDN(host) {
		return nil
	}
	return ErrSource
}

func githubCDN(host string) bool {
	switch host {
	case "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com", "github-releases.githubusercontent.com":
		return true
	default:
		return false
	}
}

func validRepoName(name string) bool {
	if len(name) < 1 || len(name) > 39 || strings.Contains(name, "..") {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
			if i == 0 || i == len(name)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func safeTag(tag string) bool {
	if tag == "" || len(tag) > 64 {
		return false
	}
	for _, r := range tag {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-' || r == '+':
		default:
			return false
		}
	}
	return true
}

// Newer reports whether latest should replace current.
// Unparsable latest tags are never newer. A final release is newer than a
// prerelease with the same numbers.
func Newer(latest, current string) bool {
	next, ok := parseVersion(latest)
	if !ok {
		return false
	}
	have, ok := parseVersion(current)
	if !ok {
		return true
	}
	if next.major != have.major {
		return next.major > have.major
	}
	if next.minor != have.minor {
		return next.minor > have.minor
	}
	if next.patch != have.patch {
		return next.patch > have.patch
	}
	if have.pre != "" && next.pre == "" {
		return true
	}
	if have.pre == "" && next.pre != "" {
		return false
	}
	return false
}

type parsedVersion struct {
	major int
	minor int
	patch int
	pre   string
}

func parseVersion(raw string) (parsedVersion, bool) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "v")
	main, pre, _ := strings.Cut(text, "-")
	parts := strings.Split(main, ".")
	if len(parts) != 3 {
		return parsedVersion{}, false
	}
	nums := [3]int{}
	for i, part := range parts {
		if part == "" || len(part) > 6 {
			return parsedVersion{}, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return parsedVersion{}, false
		}
		nums[i] = n
	}
	if pre != "" && !safeTag(pre) {
		return parsedVersion{}, false
	}
	return parsedVersion{major: nums[0], minor: nums[1], patch: nums[2], pre: pre}, true
}
