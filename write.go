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

	if err := os.MkdirAll(newDir, 0777); err != nil {
		return err
	}
	if err := os.MkdirAll(currentDir, 0777); err != nil {
		return err
	}

	for _, cm := range cms {
		log.Printf("Processing %s/%s", cm.ObjectMeta.Namespace, cm.ObjectMeta.Name)
		for fn, content := range cm.Data {
			newFile := filepath.Join(newDir, fn)
			currentFile := filepath.Join(currentDir, fn)
			log.Printf("Writing file %s", newFile)
			if err := ioutil.WriteFile(newFile, []byte(content), 0666); err != nil {
				return err
			}
			log.Printf("Moving file to %s", currentFile)
			if err := os.Rename(newFile, currentFile); err != nil {
				return fmt.Errorf("failed renaming %s to %s", newDir, currentDir)
			}
		}
	}

	return nil
}
