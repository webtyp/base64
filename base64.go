// Package base64 implements the base64 codec (RFC 4648) without the Go
// standard library.
//
// It exists because `encoding/base64` costs a measured 18,740 bytes in a TinyGo
// wasm binary — real weight on the edge (Cloudflare Workers, goflare) for a
// transformation that is a lookup table and some bit shifting. The stdlib is
// TinyGo-compatible; this package is about size, not compatibility.
package base64

// invalidError is the package's only error. It is a bare type rather than
// fmt.Err/errors.New on purpose: this package must import NOTHING.
//
// Measured under TinyGo (wasm target), pulling in webtyp/fmt just to build one
// error value costs ~74 KB — four times more than the whole encoding/base64 this
// package exists to avoid. A zero-import package is what makes it pay off.
type invalidError struct{}

func (invalidError) Error() string { return "base64 invalid" }

// ErrInvalid is returned for any input that is not well-formed base64.
var ErrInvalid error = invalidError{}

// urlAlphabet is the URL- and filename-safe alphabet (RFC 4648 §5). It differs
// from the standard one only in the last two symbols: '-' and '_' replace '+'
// and '/', which are unsafe in URLs and in JWT segments.
const urlAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// standardAlphabet is the RFC 4648 §4 alphabet used by JSON/HTTP payloads —
// data URIs, MCP content blocks, anything a browser decodes with atob() or a
// client SDK decodes as plain base64. '+' and '/' are fine there; only URLs
// and JWT segments need the URL-safe variant above.
const standardAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

var stdEncoding *Encoding
var urlEncoding *Encoding

func init() {
	var err error
	stdEncoding, err = NewEncoding(standardAlphabet, true)
	if err != nil {
		panic(err)
	}
	urlEncoding, err = NewEncoding(urlAlphabet, false)
	if err != nil {
		panic(err)
	}
}

// URLEncode encodes src as base64url (RFC 4648 §5) WITHOUT padding.
//
// Unpadded is what JWT uses (equivalent to the stdlib's RawURLEncoding). The
// output never contains '+', '/' or '='.
func URLEncode(src []byte) string {
	return urlEncoding.Encode(src)
}

// URLDecode decodes an unpadded base64url string.
//
// Every byte outside the alphabet is rejected, including '=', '+', '/' and
// whitespace, and so is any non-canonical encoding (RFC 4648 §3.5: the unused
// trailing bits of the final group must be zero). This decodes tokens, so
// leniency would mean accepting a signature segment the signer never produced.
// It is equivalent to the stdlib's RawURLEncoding.Strict() — deliberately
// stricter than the stdlib default, which accepts non-canonical input.
func URLDecode(s string) ([]byte, error) {
	b, err := urlEncoding.Decode(s)
	if err != nil {
		return nil, ErrInvalid
	}
	return b, nil
}

// Encode encodes src as standard base64 (RFC 4648 §4) WITH padding —
// equivalent to the stdlib's StdEncoding. This is what data URIs, MCP image
// content, and most JSON/HTTP payloads expect; use URLEncode instead for
// tokens embedded in a URL or a JWT segment.
func Encode(src []byte) string {
	return stdEncoding.Encode(src)
}

// Decode decodes a padded standard base64 string (RFC 4648 §4), equivalent
// to the stdlib's StdEncoding.Strict(). Padding is required and its length
// must match the trailing group size; any '=' outside the final group — or
// any byte outside the standard alphabet — is rejected via the same
// canonicality rule URLDecode applies.
func Decode(s string) ([]byte, error) {
	b, err := stdEncoding.Decode(s)
	if err != nil {
		return nil, ErrInvalid
	}
	return b, nil
}
