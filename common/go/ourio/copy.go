package ourio

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

// CopyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file. The file mode will be copied from the source.
func CopyFile(src, dst string) (err error) {
	glog.Infof("CopyFile %q -> %q", src, dst)
	in, err := os.Open(src)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	si, err := os.Stat(src)
	if err != nil {
		err = errors.Trace(err)
		return
	}

	err = os.Chmod(dst, si.Mode())
	if err != nil {
		err = errors.Trace(err)
		return
	}

	_, err = io.Copy(out, in)
	if err != nil {
		err = errors.Trace(err)
		return
	}

	return
}

// LinkOrCopyFile creates a copy of src at dst. If possible, it uses hard link
// as an optimization. If not, a full copy is performed (via CopyFile).
func LinkOrCopyFile(src, dst string) (err error) {
	glog.Infof("LinkOrCopyFile %q -> %q", src, dst)
	if err = os.Link(src, dst); err != nil {
		if os.IsExist(err) {
			if err = os.Remove(dst); err == nil {
				err = os.Link(src, dst)
			}
		}
	}
	if err == nil {
		return nil
	}
	// Fall back to Copy file: maybe src and dst are on different filesystems or
	// the OS does not support hard linking.
	return CopyFile(src, dst)
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination must either not exist or be a
// directory.
func CopyDir(src, dst string, blacklist []string) (err error) {
	glog.Infof("CopyDir %q -> %q (blacklist %s)", src, dst, blacklist)
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	dstStat, err := os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		err = errors.Trace(err)
		return
	}
	if err == nil {
		if !dstStat.IsDir() {
			return fmt.Errorf("destination already exists and is not a directory")
		}
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		err = errors.Trace(err)
		return
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		err = errors.Trace(err)
		return
	}

entriesLoop:
	for _, entry := range entries {
		for _, v := range blacklist {
			if entry.Name() == v {
				// Current entry is blacklisted, skip
				continue entriesLoop
			}
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath, blacklist)
			if err != nil {
				err = errors.Trace(err)
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				err = errors.Trace(err)
				return
			}
		}
	}

	return
}
