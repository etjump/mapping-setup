package extract

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/CalebQ42/squashfs"
)

// AppImage unpacks an AppImage (a small ELF launcher followed by a SquashFS
// image) into dir, like running it with --appimage-extract, but without
// executing anything. It returns the paths it wrote, relative to dir.
func AppImage(appImagePath, dir string) ([]string, error) {
	f, err := os.Open(appImagePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	offset, err := elfEnd(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(appImagePath), err)
	}
	r, err := squashfs.NewReaderAtOffset(f, offset)
	if err != nil {
		return nil, fmt.Errorf("%s: reading the image: %w", filepath.Base(appImagePath), err)
	}

	var written []string
	if err := unpackSquashDir(r.FS, "", dir, &written); err != nil {
		return written, fmt.Errorf("extracting %s: %w", filepath.Base(appImagePath), err)
	}
	return written, nil
}

// elfEnd returns where the ELF launcher ends, which is where the SquashFS
// image starts: the end of the section header table.
func elfEnd(r io.ReaderAt) (int64, error) {
	var h [64]byte
	if _, err := r.ReadAt(h[:], 0); err != nil {
		return 0, err
	}
	if string(h[:4]) != "\x7fELF" || h[4] != 2 || h[5] != 1 {
		return 0, errors.New("not a 64-bit AppImage")
	}
	shoff := binary.LittleEndian.Uint64(h[0x28:])
	shentsize := binary.LittleEndian.Uint16(h[0x3a:])
	shnum := binary.LittleEndian.Uint16(h[0x3c:])
	offset := int64(shoff) + int64(shentsize)*int64(shnum)

	var magic [4]byte
	if _, err := r.ReadAt(magic[:], offset); err != nil || string(magic[:]) != "hsqs" {
		return 0, errors.New("no SquashFS image found after the launcher")
	}
	return offset, nil
}

type eofReader struct{}

func (eofReader) Read([]byte) (int, error) { return 0, io.EOF }

// unpackSquashDir walks the image one folder at a time; opening nested
// paths in one call does not work on Windows with this library.
func unpackSquashDir(dirFS squashfs.FS, rel, dest string, written *[]string) error {
	entries, err := dirFS.ReadDir(".")
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := rel + e.Name()
		f, err := dirFS.OpenFile(e.Name())
		if err != nil {
			return err
		}
		switch {
		case f.IsDir():
			sub, err := f.FS()
			if err == nil {
				err = unpackSquashDir(sub, name+"/", dest, written)
			}
			f.Close()
			if err != nil {
				return err
			}
			continue
		case f.IsSymlink():
			if runtime.GOOS == "windows" {
				// Creating links needs admin rights there. AppImages are
				// only installed on Linux, so this only affects testing.
				f.Close()
				continue
			}
			err = writeSymlink(dest, name, f.SymlinkPath())
		case f.IsRegular():
			var data io.Reader = f
			if info, ierr := e.Info(); ierr == nil && info.Size() == 0 {
				data = eofReader{} // the library fails reading empty files
			}
			err = writeFile(dest, name, f.Mode(), data)
		default:
			f.Close()
			continue // device files and the like have no place here
		}
		f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		*written = append(*written, name)
	}
	return nil
}
