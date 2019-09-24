package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type writer struct {
	xdsDir string
}

func newWriter(dir string) *writer {
	if dir == "" {
		dir = "/srv/runtime"
	}
	return &writer{
		xdsDir: dir,
	}
}

func (rf *writer) write(cms []*ConfigMap) error {
	newDir := filepath.Join(rf.xdsDir, "new")
	currentDir := filepath.Join(rf.xdsDir, "current")

	prev, _ := os.Readlink(currentDir)

	b := filepath.Join(rf.xdsDir, "b")
	g := filepath.Join(rf.xdsDir, "g")

	var tmpdir string
	if prev == b {
		tmpdir = g
		if err := os.RemoveAll(g); err != nil {
			return err
		}
	} else {
		tmpdir = b
		if err := os.RemoveAll(b); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(tmpdir, 0777); err != nil {
		return err
	}

	for _, cm := range cms {
		log.Printf("Processing %s/%s", cm.ObjectMeta.Namespace, cm.ObjectMeta.Name)
		for fn, content := range cm.Data {
			fp := filepath.Join(tmpdir, fmt.Sprintf("%s", fn))
			log.Printf("Writing file %s", fp)
			if err := ioutil.WriteFile(fp, []byte(content), 0666); err != nil {
				return err
			}
		}
	}

	if err := os.Symlink(tmpdir, newDir); err != nil {
		return fmt.Errorf("failed symlinking src=%s symlink=%s: %v", tmpdir, newDir, err)
	}

	if err := os.Rename(newDir, currentDir); err != nil {
		return fmt.Errorf("failed renaming %s to %s", newDir, currentDir)
	}

	return nil
}
