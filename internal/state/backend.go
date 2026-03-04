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
func (b *GitBackend) CreateVersionTag(tenantID, slug, version, blobHash, message string) error {
	tagName := fmt.Sprintf("cactl/%s/%s/%s", tenantID, slug, version)
	cmd := exec.Command("git", "tag", "-a", tagName, blobHash, "-m", message)
	cmd.Dir = b.repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating tag %s: %s: %w", tagName, strings.TrimSpace(string(out)), err)
	}
	return nil
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

// policyRef returns the full ref path for a policy.
func policyRef(tenantID, slug string) string {
	return fmt.Sprintf("refs/cactl/tenants/%s/policies/%s", tenantID, slug)
}
