// Package multifile combines multiple fs.File instances into one logical file.
//
// This is used in the configuration parsing subsystem to load multiple logical
// bot rules files as one composite yaml file.
//
// See function YAMLList for more information.
package multifile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"
)

var (
	ErrCantOpen  = errors.New("multifile: can't open")
	ErrCantStat  = errors.New("multifile: can't stat")
	ErrCantClose = errors.New("multifile: can't close")
)

// YAMLList opens multiple files from the given filesystem and gloms them
// together into one big YAML reader where each component file is its own
// YAML document.
//
// This works because the import logic calls yaml decoding in a loop,
// making sure that every YAML document is decoded until the file runs
// out of data.
//
// Make sure to call yaml decoding in a loop until this file returns
// io.EOF.
func YAMLList(fsys fs.FS, fnames []string) (fs.File, error) {
	var files []fs.File
	var readers []io.Reader
	var errs []error
	var mfi multiFileInfo

	mfi.name = fmt.Sprintf("multifile: %v", fnames)
	mfi.mode = 0666 // the filemode of the beast

	for _, fname := range fnames {
		var (
			fin fs.File
			err error
		)

		switch fsys == nil {
		case true:
			fin, err = os.Open(fname)
		case false:
			fin, err = fsys.Open(fname)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: %q: %w", ErrCantOpen, fname, err))
			continue
		}
		files = append(files, fin)
		readers = append(readers, fin)
		readers = append(readers, bytes.NewBufferString("\n---\n"))
		mfi.size += 5

		st, err := fin.Stat()
		if err != nil {
			if closeErr := fin.Close(); closeErr != nil {
				errs = append(errs, fmt.Errorf("%w: %q: %w", ErrCantClose, fname, err))
			}

			errs = append(errs, fmt.Errorf("%w: %q: %w", ErrCantStat, fname, err))
			continue
		}

		mfi.size += st.Size()

		if mfi.modTime.Before(st.ModTime()) {
			mfi.modTime = st.ModTime()
		}
	}

	if len(errs) != 0 {
		// drain files and clean up
		for _, fin := range files {
			_ = fin.Close() // if there's an error here, we're already in trouble
		}

		return nil, errors.Join(errs...)
	}

	return multiFile{
		files:  files,
		reader: io.MultiReader(readers...),
		st:     mfi,
	}, nil
}

type multiFile struct {
	files  []fs.File
	reader io.Reader
	st     fs.FileInfo
}

func (mf multiFile) Stat() (fs.FileInfo, error) { return mf.st, nil }
func (mf multiFile) Read(p []byte) (int, error) { return mf.reader.Read(p) }

func (mf multiFile) Close() error {
	var errs []error

	for _, fin := range mf.files {
		if err := fin.Close(); err != nil {
			st, stErr := fin.Stat()
			if stErr != nil {
				errs = append(errs, fmt.Errorf("%w: %w", ErrCantStat, err))
				continue
			}
			errs = append(errs, fmt.Errorf("%q (%T): %w", st.Name(), st, err))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("%w: %w", ErrCantClose, errors.Join(errs...))
	}

	return nil
}

type multiFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (mfi multiFileInfo) Name() string       { return mfi.name }
func (mfi multiFileInfo) Size() int64        { return mfi.size }
func (mfi multiFileInfo) Mode() fs.FileMode  { return mfi.mode }
func (mfi multiFileInfo) ModTime() time.Time { return mfi.modTime }
func (mfi multiFileInfo) IsDir() bool        { return false }
func (mfi multiFileInfo) Sys() any           { return nil }
