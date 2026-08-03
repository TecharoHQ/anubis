package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// rewriteDirectoryIDs replaces wixl-heat's randomized dir<HEX> directory IDs
// in a generated fragment with IDs derived deterministically from each
// directory's path, so the fragment -- and therefore the MSI wixl builds
// from it -- is reproducible across builds.
//
// wixl-heat's cmp<HEX> and fil<HEX> IDs are already stable, derived from
// each component's and file's relative path. Only its dir<HEX> IDs are
// random: running wixl-heat twice over an identical tree produces two
// completely disjoint sets of directory IDs (verified empirically). Each
// dir<HEX> ID appears exactly once in the fragment, as a Directory
// element's own Id attribute -- nothing else in the document references it
// -- which is what makes a single rewrite pass safe.
//
// This walks the document with encoding/xml rather than a regex: a Name
// attribute or a doc file's path could otherwise contain something that
// looks like an id, and a regex over the raw text would have no way to tell
// that apart from a real dir<HEX> attribute.
func rewriteDirectoryIDs(doc []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(doc))

	var out bytes.Buffer
	enc := xml.NewEncoder(&out)

	// dirNames accumulates the Name attributes of open Directory elements,
	// in nesting order, so each directory's deterministic ID is derived
	// from its full relative path rather than just its own Name -- two
	// directories with the same leaf name under different parents (e.g. two
	// "haproxy" directories in the Anubis docs tree) must not collide.
	var dirNames []string

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("mkmsi: cannot parse doc fragment: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			// encoding/xml's Encoder writes an xmlns declaration for every
			// element with a resolved namespace, regardless of whether an
			// ancestor already declared it. Left alone, the xmlns attribute
			// this document's root already carries in Attr collides with
			// that auto-generated one and produces a duplicate attribute --
			// invalid XML that libxml2 (and therefore wixl) rejects. Every
			// element's namespace is preserved via Name.Space either way,
			// so dropping the explicit declarations here is lossless.
			t.Attr = dropXMLNSAttrs(t.Attr)

			if t.Name.Local == "Directory" {
				dirNames = append(dirNames, attrValue(t.Attr, "Name"))

				id := directoryID(directoryPath(dirNames))
				for i := range t.Attr {
					if t.Attr[i].Name.Local == "Id" {
						t.Attr[i].Value = id
					}
				}
			}

			tok = t
		case xml.EndElement:
			if t.Name.Local == "Directory" {
				dirNames = dirNames[:len(dirNames)-1]
			}
		}

		if err := enc.EncodeToken(tok); err != nil {
			return nil, fmt.Errorf("mkmsi: cannot rewrite doc fragment: %w", err)
		}
	}

	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("mkmsi: cannot rewrite doc fragment: %w", err)
	}

	return out.Bytes(), nil
}

// dropXMLNSAttrs removes default and prefixed xmlns declarations from an
// attribute list, keeping everything else in order.
func dropXMLNSAttrs(attrs []xml.Attr) []xml.Attr {
	kept := attrs[:0]
	for _, a := range attrs {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		kept = append(kept, a)
	}
	return kept
}

// attrValue returns the value of the first attribute named local, or "" if
// there is none.
func attrValue(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// directoryPath joins accumulated Directory Name attributes into a relative
// path. wixl-heat's own root directory is named ".", which is dropped so
// the path starts clean instead of carrying a leading "./".
func directoryPath(names []string) string {
	var parts []string
	for _, n := range names {
		if n == "." {
			continue
		}
		parts = append(parts, n)
	}
	return strings.Join(parts, "/")
}

// directoryID derives a deterministic Directory Id from a relative path, in
// the same dir<HEX> shape wixl-heat itself uses for cmp<HEX> and fil<HEX>: a
// fixed three-letter prefix plus a 32-character uppercase hex digest. This
// is an identifier, not a security boundary, so the first 16 bytes of a
// SHA-256 digest are used rather than MD5, purely to keep the digest the
// same length wixl-heat's own IDs use.
//
// It is path-derived rather than sequential on purpose: a sequential
// numbering scheme would renumber every directory whenever a doc file is
// added or removed anywhere in the tree, while a path-derived one only
// changes for a directory that itself moved.
func directoryID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return "dir" + strings.ToUpper(hex.EncodeToString(sum[:16]))
}
