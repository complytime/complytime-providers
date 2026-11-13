// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Evaluation storage constants define the private output layout. Context parents
// use ordinary traversable directory modes; only fresh evaluation leaves are private.
const (
	DirectoryMode        = 0o700
	FileMode             = 0o600
	ContextDirectoryMode = 0o755
	LabelLimit           = 80
	EvaluationTimeFormat = "20060102T150405Z"
	// AllocationAttempts bounds atomic name-collision retries, not other storage errors.
	AllocationAttempts = 8
	tokenBytes         = 16
	MetadataFile       = "metadata.json"
	AmpelFile          = "ampel.intoto.json"
	SnappyFile         = "snappy.intoto.json"
	ARFFile            = "arf.xml"
	XCCDFFile          = "results.xml"
	ConftestFile       = "conftest.json"
)

// Identity is the effective OS execution account, never an RPC or environment claim.
type Identity struct{ UID, GID int }

// ExecutionIdentity returns the effective OS UID/GID without an ownership handoff.
// Providers may inject this lookup to exercise identity failures without privileges.
func ExecutionIdentity() (Identity, error) {
	return Identity{UID: os.Geteuid(), GID: os.Getegid()}, nil
}

// StorageOptions supplies resolved absolute roots and an OS-derived account.
// Clock, Random, and Attempts have production defaults and permit deterministic tests.
type StorageOptions struct {
	WorkspaceRoot string
	ResultsRoot   string
	Identity      Identity
	Clock         func() time.Time
	Random        io.Reader
	Attempts      int
	Operations    Operations
}

// Evaluation owns a fresh output directory and constrained artifact access. Close
// releases its directory and identity handles, not its files. CollectionTime is collection start,
// captured before tool execution; Token supplies 128 bits of random instance identity.
// This provides local preservation, not immutable or power-loss-durable storage.
type Evaluation struct {
	Directory      string
	Token          string
	CollectionTime time.Time
	workspace      string
	owner          Identity
	dir            *os.File
	validated      map[string]*Artifact
	operations     Operations
}

// Artifact is one snapshot read through a no-follow regular-file handle. Data is
// exactly the stored bytes covered by Digest; parse Data rather than reopening Path.
// Coordinate is relative to the explicit workspace; Path is absolute and origin-specific.
type Artifact struct {
	Data       []byte
	Digest     string
	Coordinate string
	Path       string
	info       os.FileInfo
	identity   *os.File
}

// Label makes a bounded display component, independently of logical-ID encoding.
// Unsafe characters become hyphens; an empty value becomes "unknown". Labels may
// collide and must never be used as evaluation identity. URL credentials are removed.
func Label(value string) string {
	value = PublicContext(value)
	var label strings.Builder
	for _, character := range value {
		if label.Len() == LabelLimit {
			break
		}
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' {
			label.WriteRune(character)
		} else {
			label.WriteByte('-')
		}
	}
	if label.Len() == 0 {
		return "unknown"
	}
	return label.String()
}

// Allocate creates context parents and one exclusive private evaluation directory.
// Roots must already be resolved and absolute; results must be strictly beneath the
// workspace. Symlinked components below the workspace are rejected using directory
// handles. Existing directories, files, and owners are never changed. Allocation
// errors (including collision exhaustion) leave any created parents/partials in place.
func Allocate(options StorageOptions, labels ...string) (*Evaluation, error) {
	if !filepath.IsAbs(options.WorkspaceRoot) || !filepath.IsAbs(options.ResultsRoot) {
		return nil, fmt.Errorf("evaluation roots must be absolute")
	}
	relative, err := filepath.Rel(options.WorkspaceRoot, options.ResultsRoot)
	if err != nil || !filepath.IsLocal(relative) || relative == "." {
		return nil, fmt.Errorf("evaluation results root must be beneath workspace")
	}
	if options.Identity.UID < 0 || options.Identity.GID < 0 {
		return nil, fmt.Errorf("invalid execution identity")
	}
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.Random == nil {
		options.Random = rand.Reader
	}
	if options.Attempts <= 0 {
		options.Attempts = AllocationAttempts
	}
	operations := options.Operations.defaults()
	parent, err := os.Open(options.WorkspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("opening evaluation workspace: %w", err)
	}
	components := strings.Split(relative, string(filepath.Separator))
	for _, label := range labels {
		components = append(components, Label(label))
	}
	parentPath := options.WorkspaceRoot
	for _, component := range components {
		next, openErr := contextDirectory(parent, component, operations.OpenDirectory)
		closeErr := parent.Close()
		if openErr != nil {
			return nil, openErr
		}
		if closeErr != nil {
			return nil, errors.Join(closeErr, next.Close())
		}
		parent = next
		parentPath = filepath.Join(parentPath, component)
	}
	defer parent.Close()
	collected := options.Clock().UTC()
	for range options.Attempts {
		var entropy [tokenBytes]byte
		if _, err := io.ReadFull(options.Random, entropy[:]); err != nil {
			return nil, fmt.Errorf("generating evaluation token: %w", err)
		}
		token := hex.EncodeToString(entropy[:])
		name := "evaluation-" + collected.Format(EvaluationTimeFormat) + "-" + token
		parentFD, err := directoryDescriptor(parent)
		if err != nil {
			return nil, err
		}
		if err := syscall.Mkdirat(parentFD, name, DirectoryMode); err != nil {
			if errors.Is(err, syscall.EEXIST) {
				continue
			}
			return nil, fmt.Errorf("allocating evaluation: %w", err)
		}
		directory, err := operations.OpenDirectory(parent, name)
		if err != nil {
			return nil, fmt.Errorf("opening new evaluation: %w", err)
		}
		evaluation := &Evaluation{
			Directory: filepath.Join(parentPath, name), Token: token, CollectionTime: collected,
			workspace: options.WorkspaceRoot, owner: options.Identity, dir: directory,
			validated:  make(map[string]*Artifact),
			operations: operations,
		}
		if err := evaluation.verifyFile(directory, DirectoryMode); err != nil {
			return nil, &VerificationError{Err: errors.Join(err, directory.Close())}
		}
		return evaluation, nil
	}
	return nil, fmt.Errorf("evaluation allocation exhausted %d collision attempts", options.Attempts)
}

func contextDirectory(parent *os.File, name string, open func(*os.File, string) (*os.File, error)) (*os.File, error) {
	parentFD, err := directoryDescriptor(parent)
	if err != nil {
		return nil, err
	}
	if err := syscall.Mkdirat(parentFD, name, ContextDirectoryMode); err != nil &&
		!errors.Is(err, syscall.EEXIST) {
		return nil, fmt.Errorf("creating evaluation context %q: %w", name, err)
	}
	directory, err := open(parent, name)
	if err != nil {
		return nil, fmt.Errorf("opening evaluation context %q: %w", name, err)
	}
	return directory, nil
}

// EnsureDirectories creates provider context directories beneath a resolved root
// without following any child symlinks. Every path must be workspace-relative and
// local. Existing parents retain their modes and ownership; no process chdir occurs.
func EnsureDirectories(root string, paths ...string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("directory root must be absolute")
	}
	for _, path := range paths {
		if !filepath.IsLocal(path) {
			return fmt.Errorf("provider directory must be workspace-relative")
		}
	}
	for _, path := range paths {
		parent, err := os.Open(root)
		if err != nil {
			return fmt.Errorf("opening provider workspace: %w", err)
		}
		for _, component := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
			next, openErr := contextDirectory(parent, component, openDirectory)
			closeErr := parent.Close()
			if openErr != nil {
				return openErr
			}
			if closeErr != nil {
				return errors.Join(closeErr, next.Close())
			}
			parent = next
		}
		if err := parent.Close(); err != nil {
			return fmt.Errorf("closing provider directory: %w", err)
		}
	}
	return nil
}

func openDirectory(parent *os.File, name string) (*os.File, error) {
	return openAt(parent, name, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
}

func openAt(parent *os.File, name string, flags int, mode uint32) (*os.File, error) {
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return nil, fmt.Errorf("invalid artifact component %q", name)
	}
	parentFD, err := directoryDescriptor(parent)
	if err != nil {
		return nil, err
	}
	descriptor, err := syscall.Openat(parentFD, name,
		flags|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, mode)
	if err != nil {
		return nil, err
	}
	if descriptor < 0 {
		return nil, fmt.Errorf("invalid open file descriptor")
	}
	return os.NewFile(uintptr(descriptor), name), nil
}

func directoryDescriptor(file *os.File) (int, error) {
	descriptor := file.Fd()
	if descriptor > math.MaxInt {
		return 0, fmt.Errorf("invalid directory descriptor")
	}
	return int(descriptor), nil
}

// Close releases allocation and artifact-identity handles without deleting output.
// Every handle is closed even when an earlier close fails; all errors are returned.
func (evaluation *Evaluation) Close() error {
	var closeErr error
	for _, artifact := range evaluation.validated {
		if artifact.identity != nil {
			closeErr = errors.Join(closeErr, artifact.identity.Close())
			artifact.identity = nil
		}
	}
	return errors.Join(closeErr, evaluation.dir.Close())
}

// Path returns an absolute output locator for a single artifact component. It is
// not a safe read API: use Read to validate and hash tool output through a handle.
func (evaluation *Evaluation) Path(name string) string {
	return filepath.Join(evaluation.Directory, name)
}

// Prepare exclusively reserves a private, empty tool-output file. Tool execution
// must finish before Read and Finalize. Existing artifacts are never truncated.
func (evaluation *Evaluation) Prepare(name string) (string, error) {
	file, err := openAt(evaluation.dir, name, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL, FileMode)
	if err != nil {
		return "", fmt.Errorf("reserving output %q: %w", name, err)
	}
	if err := evaluation.operations.Close(file); err != nil {
		return "", fmt.Errorf("closing output %q: %w", name, err)
	}
	return evaluation.Path(name), nil
}

// Write exclusively writes a provider-owned output and checks its close error.
// A failure can leave unreferenced partial bytes; it never replaces an earlier file.
func (evaluation *Evaluation) Write(name string, data []byte) error {
	file, err := openAt(evaluation.dir, name, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL, FileMode)
	if err != nil {
		return fmt.Errorf("creating artifact %q: %w", name, err)
	}
	written, writeErr := evaluation.operations.Write(file, data)
	if writeErr == nil && written != len(data) {
		writeErr = io.ErrShortWrite
	}
	if err := errors.Join(writeErr, evaluation.operations.Close(file)); err != nil {
		return fmt.Errorf("saving artifact %q: %w", name, err)
	}
	return nil
}

// Read rejects symlinks and non-regular outputs, then snapshots and hashes the same
// descriptor's bytes. Parsing must use the returned Data. An identity handle pins
// the validated inode until Close so it cannot be recycled after unlinking. Finalize
// detects subsequent substitutions/changes. Missing/unreadable outputs are errors.
func (evaluation *Evaluation) Read(name string) (*Artifact, error) {
	artifact, err := evaluation.snapshot(name)
	if err != nil {
		return nil, err
	}
	if previous := evaluation.validated[name]; previous != nil {
		if !sameArtifact(previous.info, artifact.info) || !bytes.Equal(previous.Data, artifact.Data) {
			return nil, errors.Join(fmt.Errorf("artifact %q changed after validation", name),
				artifact.identity.Close())
		}
		if err := artifact.identity.Close(); err != nil {
			return nil, fmt.Errorf("closing repeated artifact identity %q: %w", name, err)
		}
		artifact.identity = previous.identity
	}
	evaluation.validated[name] = artifact
	return artifact, nil
}

func (evaluation *Evaluation) snapshot(name string) (*Artifact, error) {
	path := evaluation.Path(name)
	coordinate, err := filepath.Rel(evaluation.workspace, path)
	if err != nil {
		return nil, fmt.Errorf("artifact coordinate: %w", err)
	}
	file, err := openAt(evaluation.dir, name, syscall.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("opening artifact %q: %w", name, err)
	}
	info, statErr := evaluation.operations.Stat(file)
	if statErr != nil {
		return nil, errors.Join(statErr, file.Close())
	}
	if !info.Mode().IsRegular() {
		return nil, errors.Join(fmt.Errorf("artifact %q is not regular", name), file.Close())
	}
	data, readErr := evaluation.operations.Read(file)
	var identity *os.File
	var identityErr error
	if readErr == nil {
		identity, identityErr = evaluation.pinIdentity(name, info)
	}
	if err := errors.Join(readErr, identityErr, evaluation.operations.Close(file)); err != nil {
		if identity != nil {
			err = errors.Join(err, identity.Close())
		}
		return nil, fmt.Errorf("reading artifact %q: %w", name, err)
	}
	return &Artifact{Data: data, Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(data)),
		Path: path, Coordinate: coordinate, info: info, identity: identity}, nil
}

func (evaluation *Evaluation) pinIdentity(name string, expected os.FileInfo) (*os.File, error) {
	// The snapshot descriptor is still open, so a replacement cannot reuse its
	// inode while this second constrained handle is opened and checked.
	identity, err := openAt(evaluation.dir, name, syscall.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("pinning artifact identity %q: %w", name, err)
	}
	info, err := evaluation.operations.Stat(identity)
	if err != nil {
		return nil, errors.Join(err, identity.Close())
	}
	if !sameArtifact(expected, info) {
		return nil, errors.Join(fmt.Errorf("artifact %q replaced during validation", name),
			identity.Close())
	}
	return identity, nil
}

func (evaluation *Evaluation) verifyFile(file *os.File, mode os.FileMode) error {
	info, err := evaluation.operations.Stat(file)
	if err != nil {
		return fmt.Errorf("checking evaluation ownership: %w", err)
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(owner.Uid) != evaluation.owner.UID || int(owner.Gid) != evaluation.owner.GID {
		return fmt.Errorf("unexpected execution-account ownership on %q", file.Name())
	}
	if err := evaluation.operations.Chmod(file, mode); err != nil {
		return fmt.Errorf("setting private modes: %w", err)
	}
	info, err = evaluation.operations.Stat(file)
	if err != nil {
		return fmt.Errorf("verifying private modes on %q: %w", file.Name(), err)
	}
	if info.Mode().Perm() != mode {
		return fmt.Errorf("unexpected private modes on %q", file.Name())
	}
	return nil
}

// Finalize verifies execution-account ownership and private modes on the fresh
// leaf and every present output, including partial metadata. It rejects symlinks,
// non-regular contents and changed validated artifacts. Call only after tools stop
// writing; failures are publication-critical RPC errors. No ownership is transferred.
func (evaluation *Evaluation) Finalize() error {
	if err := evaluation.finalize(); err != nil {
		return &VerificationError{Err: err}
	}
	return nil
}

func (evaluation *Evaluation) finalize() error {
	if err := evaluation.verifyFile(evaluation.dir, DirectoryMode); err != nil {
		return err
	}
	if _, err := evaluation.dir.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewinding evaluation directory: %w", err)
	}
	entries, err := evaluation.dir.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("listing evaluation outputs: %w", err)
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		present[entry.Name()] = true
		file, err := openAt(evaluation.dir, entry.Name(), syscall.O_RDONLY, 0)
		if err != nil {
			return fmt.Errorf("finalizing output %q: %w", entry.Name(), err)
		}
		info, statErr := evaluation.operations.Stat(file)
		if statErr != nil {
			return errors.Join(statErr, file.Close())
		}
		if !info.Mode().IsRegular() {
			return errors.Join(fmt.Errorf("output %q is not a regular file", entry.Name()), file.Close())
		}
		if expected := evaluation.validated[entry.Name()]; expected != nil {
			data, readErr := evaluation.operations.Read(file)
			if readErr != nil {
				return errors.Join(readErr, file.Close())
			}
			if !sameArtifact(expected.info, info) || !bytes.Equal(expected.Data, data) {
				return errors.Join(fmt.Errorf("validated output %q changed before publication", entry.Name()), file.Close())
			}
		}
		if err := errors.Join(evaluation.verifyFile(file, FileMode), file.Close()); err != nil {
			return err
		}
	}
	for name := range evaluation.validated {
		if !present[name] {
			return fmt.Errorf("validated output %q disappeared before publication", name)
		}
	}
	return evaluation.verifyLocator()
}

func sameArtifact(first, second os.FileInfo) bool {
	firstStat, firstOK := first.Sys().(*syscall.Stat_t)
	secondStat, secondOK := second.Sys().(*syscall.Stat_t)
	return firstOK && secondOK && firstStat.Dev == secondStat.Dev && firstStat.Ino == secondStat.Ino
}

func (evaluation *Evaluation) verifyLocator() error {
	// Reopen each component without following links to ensure the public locator
	// still identifies our held directory and remains traversable by the process.
	parent, err := os.Open(evaluation.workspace)
	if err != nil {
		return fmt.Errorf("reopening evidence workspace: %w", err)
	}
	relative, err := filepath.Rel(evaluation.workspace, evaluation.Directory)
	if err != nil {
		return errors.Join(err, parent.Close())
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		next, openErr := evaluation.operations.OpenDirectory(parent, component)
		closeErr := parent.Close()
		if openErr != nil {
			return fmt.Errorf("evidence parent is inaccessible or replaced: %w", openErr)
		}
		if closeErr != nil {
			return errors.Join(closeErr, next.Close())
		}
		parent = next
	}
	info, err := parent.Stat()
	held, heldErr := evaluation.dir.Stat()
	closeErr := parent.Close()
	if err != nil || heldErr != nil || closeErr != nil {
		return errors.Join(err, heldErr, closeErr)
	}
	if !os.SameFile(info, held) {
		return fmt.Errorf("evaluation path changed before publication")
	}
	return nil
}
