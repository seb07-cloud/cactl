// Package state provides Git-backed state storage for CA policies using custom refs.
package state

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GitBackend stores policy state in Git's object store using custom refs.
// Policies are stored as blobs and referenced via refs/cactl/tenants/<tenant>/policies/<slug>.
// Version history is tracked via annotated tags at cactl/<tenant>/<slug>/<semver>.
type GitBackend struct {
	repoDir string
}

// NewGitBackend creates a new GitBackend rooted at the given repo directory.
// It validates that the directory is a valid Git repository.
func NewGitBackend(repoDir string) (*GitBackend, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("not a git repository (%s): %s: %w", repoDir, strings.TrimSpace(string(out)), err)
	}
	return &GitBackend{repoDir: repoDir}, nil
}

// WritePolicy writes policy JSON as a blob and updates the custom ref.
// Returns the blob SHA hash.
func (b *GitBackend) WritePolicy(tenantID, slug string, data []byte) (string, error) {
	hash, err := b.hashObject(data)
	if err != nil {
		return "", fmt.Errorf("writing blob for %s: %w", slug, err)
	}

	ref := policyRef(tenantID, slug)
	if err := b.updateRef(ref, hash); err != nil {
		return "", fmt.Errorf("updating ref %s: %w", ref, err)
	}

	return hash, nil
}

// ReadPolicy reads the policy JSON blob from the custom ref.
func (b *GitBackend) ReadPolicy(tenantID, slug string) ([]byte, error) {
	ref := policyRef(tenantID, slug)
	return b.catFile(ref)
}

// ListPolicies returns all tracked policy slugs for a tenant.
func (b *GitBackend) ListPolicies(tenantID string) ([]string, error) {
	prefix := fmt.Sprintf("refs/cactl/tenants/%s/policies/", tenantID)
	return b.forEachRef(prefix)
}

// CreateVersionTag creates an annotated tag pointing to the given blob hash.
// Tag format: cactl/<tenantID>/<slug>/<version>
//
// If the tag already exists and points to the same blob, it is treated as a
// no-op (idempotent). If it exists with different content, the version is
// auto-bumped (patch increment) until a free tag is found.
//
// Returns the actual version used (which may differ from the requested version
// if auto-bumping occurred).
func (b *GitBackend) CreateVersionTag(tenantID, slug, version, blobHash, message string) (string, error) {
	for attempts := 0; attempts < 100; attempts++ {
		tagName := fmt.Sprintf("cactl/%s/%s/%s", tenantID, slug, version)

		existing, err := b.tagTarget(tagName)
		if err != nil {
			// Tag doesn't exist — create it
			cmd := exec.Command("git", "tag", "-a", tagName, blobHash, "-m", message)
			cmd.Dir = b.repoDir
			if out, err := cmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("creating tag %s: %s: %w", tagName, strings.TrimSpace(string(out)), err)
			}
			return version, nil
		}

		// Tag exists — check if same content (idempotent)
		if existing == blobHash {
			return version, nil
		}

		// Different content: bump patch and retry
		version = bumpPatch(version)
	}

	return "", fmt.Errorf("could not find free version tag for %s/%s after 100 attempts", slug, version)
}

// bumpPatch increments the patch component of a semver string.
func bumpPatch(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return version + ".1"
	}
	patch := 0
	fmt.Sscanf(parts[2], "%d", &patch)
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
}

// tagTarget returns the blob hash that an annotated tag points to.
// Returns an error if the tag does not exist.
func (b *GitBackend) tagTarget(tagName string) (string, error) {
	cmd := exec.Command("git", "rev-parse", tagName+"^{}")
	cmd.Dir = b.repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// hashObject writes data as a blob to the Git object store and returns the SHA.
func (b *GitBackend) hashObject(data []byte) (string, error) {
	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Dir = b.repoDir
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git hash-object: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// updateRef points a ref to the given hash.
func (b *GitBackend) updateRef(ref, hash string) error {
	cmd := exec.Command("git", "update-ref", ref, hash)
	cmd.Dir = b.repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git update-ref: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// catFile reads the blob content at the given ref.
func (b *GitBackend) catFile(ref string) ([]byte, error) {
	cmd := exec.Command("git", "cat-file", "blob", ref)
	cmd.Dir = b.repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ref, err)
	}
	return out, nil
}

// forEachRef lists refs under the given prefix and extracts the slug (last path component).
func (b *GitBackend) forEachRef(prefix string) ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname)", prefix)
	cmd.Dir = b.repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	slugs := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Extract slug: last component of ref path
		parts := strings.Split(line, "/")
		if len(parts) > 0 {
			slugs = append(slugs, parts[len(parts)-1])
		}
	}
	return slugs, nil
}

// VersionTag represents a single version entry from the tag history.
type VersionTag struct {
	Version   string
	Timestamp string
	Message   string
}

// ListVersionTags returns all version tags for a policy sorted by semver descending.
// Returns an empty slice (not nil) when no tags exist.
func (b *GitBackend) ListVersionTags(tenantID, slug string) ([]VersionTag, error) {
	prefix := fmt.Sprintf("refs/tags/cactl/%s/%s/", tenantID, slug)
	cmd := exec.Command("git", "for-each-ref",
		"--format=%(refname:strip=5)\t%(creatordate:iso)\t%(contents:lines=1)",
		"--sort=-version:refname",
		prefix,
	)
	cmd.Dir = b.repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing version tags: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return []VersionTag{}, nil
	}

	lines := strings.Split(output, "\n")
	tags := make([]VersionTag, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 1 {
			continue
		}
		version := parts[0]
		timestamp := ""
		message := ""
		if len(parts) >= 2 {
			timestamp = parts[1]
		}
		if len(parts) >= 3 {
			message = parts[2]
		}
		tags = append(tags, VersionTag{
			Version:   version,
			Timestamp: timestamp,
			Message:   message,
		})
	}
	return tags, nil
}

// ReadTagBlob reads the policy JSON content from an annotated tag.
// Uses ^{} to dereference the annotated tag to the underlying blob.
func (b *GitBackend) ReadTagBlob(tenantID, slug, version string) ([]byte, error) {
	tagName := fmt.Sprintf("cactl/%s/%s/%s", tenantID, slug, version)
	cmd := exec.Command("git", "cat-file", "blob", tagName+"^{}")
	cmd.Dir = b.repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("reading tag %s: %w", tagName, err)
	}
	return out, nil
}

// HashObject computes the git SHA-1 hash for arbitrary data bytes.
// This wraps the internal hashObject and writes the blob to the object store.
func (b *GitBackend) HashObject(data []byte) (string, error) {
	return b.hashObject(data)
}

// policyRef returns the full ref path for a policy.
func policyRef(tenantID, slug string) string {
	return fmt.Sprintf("refs/cactl/tenants/%s/policies/%s", tenantID, slug)
}
