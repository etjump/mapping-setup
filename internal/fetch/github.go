package fetch

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// GitHub lookups use the REST API first and fall back to plain github.com
// pages, so the 60 requests/hour API limit for anonymous users never blocks
// an install. Setting GITHUB_TOKEN raises that limit.

// Asset is a downloadable file attached to a GitHub release.
type Asset struct {
	Name   string
	URL    string
	SHA256 string // empty when only the fallback could be used
	Size   int64
}

func apiHeader() http.Header {
	h := http.Header{}
	h.Set("Accept", "application/vnd.github+json")
	h.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	return h
}

// LatestReleaseTag returns the tag of the release GitHub marks as latest.
func (c *Client) LatestReleaseTag(ctx context.Context, repo string) (string, error) {
	var rel struct {
		TagName string `json:"tag_name"`
	}
	err := c.GetJSON(ctx, "https://api.github.com/repos/"+repo+"/releases/latest", apiHeader(), &rel)
	if err == nil && rel.TagName != "" {
		return rel.TagName, nil
	}

	loc, ferr := c.RedirectLocation(ctx, "https://github.com/"+repo+"/releases/latest")
	if ferr != nil {
		return "", fmt.Errorf("finding the latest %s release: %w", repo, firstErr(err, ferr))
	}
	i := strings.LastIndex(loc, "/tag/")
	if i < 0 {
		return "", fmt.Errorf("finding the latest %s release: unexpected redirect to %s", repo, loc)
	}
	return loc[i+len("/tag/"):], nil
}

// ReleaseAsset finds the asset of release tag whose name satisfies match.
func (c *Client) ReleaseAsset(ctx context.Context, repo, tag string, match func(name string) bool) (Asset, error) {
	var rel struct {
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		} `json:"assets"`
	}
	err := c.GetJSON(ctx, "https://api.github.com/repos/"+repo+"/releases/tags/"+tag, apiHeader(), &rel)
	if err == nil {
		for _, a := range rel.Assets {
			if match(a.Name) {
				return Asset{
					Name:   a.Name,
					URL:    a.URL,
					SHA256: strings.TrimPrefix(a.Digest, "sha256:"),
					Size:   a.Size,
				}, nil
			}
		}
		return Asset{}, fmt.Errorf("%s release %s has no matching download", repo, tag)
	}

	// Fallback: the HTML fragment the release page loads its asset list from.
	page, ferr := c.GetBytes(ctx, "https://github.com/"+repo+"/releases/expanded_assets/"+tag, nil)
	if ferr != nil {
		return Asset{}, fmt.Errorf("listing %s release %s: %w", repo, tag, firstErr(err, ferr))
	}
	prefix := "/" + repo + "/releases/download/" + tag + "/"
	re := regexp.MustCompile(`href="(` + regexp.QuoteMeta(prefix) + `[^"]+)"`)
	for _, m := range re.FindAllSubmatch(page, -1) {
		path := string(m[1])
		name := path[len(prefix):]
		if match(name) {
			return Asset{Name: name, URL: "https://github.com" + path}, nil
		}
	}
	return Asset{}, fmt.Errorf("%s release %s has no matching download", repo, tag)
}

// Refs lists a repository's branches and tags with their commit hashes,
// using git's own HTTP protocol (not rate limited like the API).
// Keys look like "refs/heads/master" or "refs/tags/v1.0".
func (c *Client) Refs(ctx context.Context, repo string) (map[string]string, error) {
	body, err := c.GetBytes(ctx, "https://github.com/"+repo+".git/info/refs?service=git-upload-pack", nil)
	if err != nil {
		return nil, fmt.Errorf("listing %s versions: %w", repo, err)
	}
	refs := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		if i := strings.IndexByte(line, 0); i >= 0 {
			line = line[:i] // capabilities follow the first ref
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) < 40 {
			continue
		}
		// Each line starts with a 4-digit pkt-line length before the hash.
		hash := fields[0][len(fields[0])-40:]
		name := strings.TrimSuffix(fields[1], "^{}")
		if _, seen := refs[name]; !seen || strings.HasSuffix(fields[1], "^{}") {
			refs[name] = hash // peeled tags point at the commit itself
		}
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("listing %s versions: empty response", repo)
	}
	return refs, nil
}

// BranchCommit returns the commit hash a branch currently points at.
func (c *Client) BranchCommit(ctx context.Context, repo, branch string) (string, error) {
	refs, err := c.Refs(ctx, repo)
	if err != nil {
		return "", err
	}
	hash, ok := refs["refs/heads/"+branch]
	if !ok {
		return "", fmt.Errorf("%s has no branch %s", repo, branch)
	}
	return hash, nil
}

// NewestVersionTag returns the highest plain version tag (like 3.6.2 or
// v3.6.2), ignoring rolling tags such as "latest" and pre-releases.
func (c *Client) NewestVersionTag(ctx context.Context, repo string) (string, error) {
	refs, err := c.Refs(ctx, repo)
	if err != nil {
		return "", err
	}
	best, bestVer := "", []int(nil)
	for name := range refs {
		tag, ok := strings.CutPrefix(name, "refs/tags/")
		if !ok {
			continue
		}
		if v := ParseVersion(tag); v != nil && CompareVersions(v, bestVer) > 0 {
			best, bestVer = tag, v
		}
	}
	if best == "" {
		return "", fmt.Errorf("%s has no version tags", repo)
	}
	return best, nil
}

var versionRe = regexp.MustCompile(`^v?(\d+(?:\.\d+)*)$`)

// ParseVersion parses "1.2.3" or "v1.2.3" into its numbers, or returns nil.
func ParseVersion(s string) []int {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	var v []int
	for _, part := range strings.Split(m[1], ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		v = append(v, n)
	}
	return v
}

// CompareVersions compares two parsed versions; nil sorts lowest.
func CompareVersions(a, b []int) int {
	if a == nil || b == nil {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return -1
		default:
			return 1
		}
	}
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
