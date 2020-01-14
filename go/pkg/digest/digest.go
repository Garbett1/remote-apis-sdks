// Package digest contains functions to simplify handling content digests.
package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
)

var (
	// hexStringRegex doesn't contain the size because that's checked separately.
	hexStringRegex = regexp.MustCompile("^[a-f0-9]+$")

	// Empty is the SHA256 digest of the empty blob.
	Empty = NewFromBlob([]byte{})
)

// Digest is a Go type to mirror the repb.Digest message.
type Digest struct {
	Hash string
	Size int64
}

// ToProto converts a Digest into a repb.Digest. No validation is performed!
func (d Digest) ToProto() *repb.Digest {
	return &repb.Digest{Hash: d.Hash, SizeBytes: d.Size}
}

// String returns a hash in a canonical form of hash/size.
func (d Digest) String() string {
	return fmt.Sprintf("%s/%d", d.Hash, d.Size)
}

// IsEmpty returns true iff digest is of an empty blob.
func (d Digest) IsEmpty() bool {
	return d.Size == 0 && d.Hash == Empty.Hash
}

// Validate returns nil if a digest appears to be valid, or a descriptive error
// if it is not. All functions accepting digests directly from clients should
// call this function, whether it's via an RPC call or by reading a serialized
// proto message that contains digests that was uploaded directly from the
// client.
func (d Digest) Validate() error {
	length := len(d.Hash)
	if length != sha256.Size*2 {
		return fmt.Errorf("valid hash length is %d, got length %d (%s)", sha256.Size*2, length, d.Hash)
	}
	if !hexStringRegex.MatchString(d.Hash) {
		return fmt.Errorf("hash is not a lowercase hex string (%s)", d.Hash)
	}
	if d.Size < 0 {
		return fmt.Errorf("expected non-negative size, got %d", d.Size)
	}
	return nil
}

// New creates a new digest from a string and size. It does some basic
// validation, which makes it marginally superior to constructing a Digest
// yourself. It returns an error if the hash/size are invalid.
func New(hash string, size int64) (Digest, error) {
	d := Digest{Hash: hash, Size: size}
	return d, d.Validate()
}

// NewFromBlob takes a blob (in the form of a byte array) and returns the
// Digest for that blob. Changing this function will lead to cache
// invalidations (execution cache and potentially others).
// This cannot return an error, since the result is valid by definition.
func NewFromBlob(blob []byte) Digest {
	sha256Arr := sha256.Sum256(blob)
	return Digest{Hash: hex.EncodeToString(sha256Arr[:]), Size: int64(len(blob))}
}

// NewFromMessage calculates the digest of a protobuf in SHA-256 mode.
// It returns an error if the proto marshalling failed.
func NewFromMessage(msg proto.Message) (Digest, error) {
	blob, err := proto.Marshal(msg)
	if err != nil {
		return Empty, err
	}
	return NewFromBlob(blob), nil
}

// NewFromProto converts a proto digest to a Digest.
// It returns an error if the hash/size are invalid.
func NewFromProto(dg *repb.Digest) (Digest, error) {
	d := NewFromProtoUnvalidated(dg)
	return d, d.Validate()
}

// NewFromProtoUnvalidated converts a proto digest to a Digest, skipping validation.
func NewFromProtoUnvalidated(dg *repb.Digest) Digest {
	return Digest{Hash: dg.Hash, Size: dg.SizeBytes}
}

// NewFromString returns a digest from a canonical digest string.
// It returns an error if the hash/size are invalid.
func NewFromString(s string) (Digest, error) {
	pair := strings.Split(s, "/")
	if len(pair) != 2 {
		return Empty, fmt.Errorf("expected digest in the form hash/size, got %s", s)
	}
	size, err := strconv.ParseInt(pair[1], 10, 64)
	if err != nil {
		return Empty, fmt.Errorf("invalid size in digest %s: %s", s, err)
	}
	return New(pair[0], size)
}

// NewFromFile computes a file digest from a path.
// It returns an error if there was a problem accessing the file.
func NewFromFile(path string) (Digest, error) {
	f, err := os.Open(path)
	if err != nil {
		return Empty, err
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return Empty, err
	}
	return Digest{
		Hash: hex.EncodeToString(h.Sum(nil)),
		Size: size,
	}, nil
}

// TestNew is like New but also pads your hash with zeros if it is shorter than the required length,
// and panics on error rather than returning the error.
// ONLY USE FOR TESTS.
func TestNew(hash string, size int64) Digest {
	hashLen := sha256.Size * 2
	if len(hash) < hashLen {
		hash = strings.Repeat("0", hashLen-len(hash)) + hash
	}

	digest, err := New(hash, size)
	if err != nil {
		panic(err.Error())
	}
	return digest
}

// TestNewFromMessage is only suitable for testing and panics on error.
// ONLY USE FOR TESTS.
func TestNewFromMessage(msg proto.Message) Digest {
	digest, err := NewFromMessage(msg)
	if err != nil {
		panic(err.Error())
	}
	return digest
}